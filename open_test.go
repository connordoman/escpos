package escpos

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseSpec(t *testing.T) {
	for in, want := range map[string]Spec{
		"usb":                                {Scheme: "usb"},
		"USB?vid=0fe6&pid=811e":              {Scheme: "usb", Query: map[string][]string{"vid": {"0fe6"}, "pid": {"811e"}}},
		"file:/dev/usb/lp0":                  {Scheme: "file", Target: "/dev/usb/lp0"},
		"file:///dev/usb/lp0":                {Scheme: "file", Target: "/dev/usb/lp0"},
		`file:\\?\USB#VID_0FE6#x#{28d78fad}`: {Scheme: "file", Target: `\\?\USB#VID_0FE6#x#{28d78fad}`},
		"tcp://10.0.0.5:9100":                {Scheme: "tcp", Target: "10.0.0.5:9100"},
		"tcp:printer.local":                  {Scheme: "tcp", Target: "printer.local"},
		"serial:COM3?baud=19200":             {Scheme: "serial", Target: "COM3", Query: map[string][]string{"baud": {"19200"}}},
		"serial:///dev/ttyS0":                {Scheme: "serial", Target: "/dev/ttyS0"},
	} {
		got, err := ParseSpec(in)
		if err != nil {
			t.Errorf("ParseSpec(%q): %v", in, err)
			continue
		}
		if got.Scheme != want.Scheme || got.Target != want.Target || got.Query.Encode() != withQuery(want).Query.Encode() {
			t.Errorf("ParseSpec(%q) = %+v, want %+v", in, got, want)
		}
	}
	if _, err := ParseSpec(" "); err == nil {
		t.Error("empty string accepted")
	}
}

func withQuery(s Spec) Spec {
	if s.Query == nil {
		s.Query = map[string][]string{}
	}
	return s
}

func TestOpenBuiltins(t *testing.T) {
	ctx := context.Background()
	w, c, err := Open(ctx, "discard")
	if err != nil || c.Kind != "discard" || c.Readable {
		t.Errorf("discard: %v %+v", err, c)
	}
	w.Close()

	path := filepath.Join(t.TempDir(), "printer")
	if _, _, err := Open(ctx, "file:"+path); err == nil {
		t.Error("missing file opened")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			c.Close()
		}
	}()
	w, c, err = Open(ctx, "tcp://"+ln.Addr().String())
	if err != nil || c.Kind != "tcp" || !c.Readable {
		t.Fatalf("tcp: %v %+v", err, c)
	}
	w.Close()

	_, _, err = Open(ctx, "serial:/dev/ttyUSB0")
	if !errors.Is(err, ErrUnknownScheme) || !strings.Contains(err.Error(), "escpos/serial") {
		t.Errorf("serial without the package: %v", err)
	}
}

func TestUSBMatch(t *testing.T) {
	m, err := USBMatch(map[string][]string{"vid": {"0x0FE6"}, "serial": {"ABC"}})
	if err != nil {
		t.Fatal(err)
	}
	if !m(USBPrinter{VendorID: 0x0fe6, ProductID: 1, Serial: "ABC"}) || m(USBPrinter{VendorID: 0x0fe6, Serial: "XYZ"}) {
		t.Error("filter matches wrongly")
	}
	if m, err := USBMatch(nil); m != nil || err != nil {
		t.Error("empty query should give no filter")
	}
	if _, err := USBMatch(map[string][]string{"pid": {"nope"}}); err == nil {
		t.Error("bad pid accepted")
	}
}

// flaky is a connection whose writes fail while broken is set.
type flaky struct {
	broken *atomic.Bool
}

func (f flaky) Write(p []byte) (int, error) {
	if f.broken.Load() {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}
func (flaky) Close() error { return nil }

func TestDeviceReconnects(t *testing.T) {
	var opens atomic.Int32
	var broken atomic.Bool
	RegisterScheme("test-flaky", func(context.Context, Spec) (io.WriteCloser, Connection, error) {
		opens.Add(1)
		return flaky{&broken}, Connection{Kind: "test-flaky"}, nil
	})
	if !slices.Contains(Schemes(), "test-flaky") {
		t.Error("registered scheme not listed")
	}
	d := NewDevice("test-flaky")
	var connects atomic.Int32
	d.OnConnect = func(context.Context, *Printer, Connection) { connects.Add(1) }
	ctx := context.Background()
	send := func() error {
		return d.Do(ctx, func(p *Printer) error { _, err := p.SendRaw(ctx, []byte("x")); return err })
	}

	if err := send(); err != nil || opens.Load() != 1 {
		t.Fatalf("first send: %v, %d opens", err, opens.Load())
	}
	// An invalid argument does not drop the connection...
	d.Do(ctx, func(p *Printer) error { return p.Beep(0, 0) })
	if _, ok := d.Connection(); !ok {
		t.Error("invalid argument dropped the connection")
	}
	// ...but a failed write does, and the next use reopens it.
	broken.Store(true)
	if err := send(); err == nil {
		t.Fatal("write to a broken connection succeeded")
	}
	if _, ok := d.Connection(); ok {
		t.Error("failed write kept the connection")
	}
	broken.Store(false)
	if err := send(); err != nil || opens.Load() != 2 || connects.Load() != 2 {
		t.Errorf("after reconnecting: %v, %d opens, %d OnConnect calls", err, opens.Load(), connects.Load())
	}
}

func TestIdentify(t *testing.T) {
	conn, _, _ := fakePrinter(t, func(cmd []byte) []byte {
		if len(cmd) != 3 || cmd[0] != GS || cmd[1] != 'I' {
			return nil
		}
		switch cmd[2] {
		case 1:
			return []byte{0x20}
		case 2:
			return []byte{0x02}
		case 65:
			return []byte("_7.03 ESC/POS\x00")
		case 68:
			return []byte("_D6KG074561\x00")
		}
		return []byte("_\x00")
	})
	id, err := New(conn).Identify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if id.ModelID != 0x20 || !id.TypeID.AutoCutter() || id.Firmware != "7.03 ESC/POS" || id.Serial != "D6KG074561" {
		t.Errorf("got %+v", id)
	}
}
