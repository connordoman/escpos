package escpos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Connection describes a connection opened by [Open].
type Connection struct {
	// Kind is the scheme that opened it: "usb", "libusb", "file", "tcp",
	// "serial", "discard", or one registered with [RegisterScheme].
	Kind string

	// Target is what was opened: a device path, network address or port.
	Target string

	// USB describes the printer when Kind is "usb" or "libusb".
	USB *USBPrinter

	// Readable reports whether the connection can receive replies, which
	// status queries need.
	Readable bool
}

// Opener opens a connection for a scheme registered with [RegisterScheme].
// spec is the parsed connection string; the returned Connection's Readable
// field is filled in by Open.
type Opener func(ctx context.Context, spec Spec) (io.WriteCloser, Connection, error)

// Spec is a parsed connection string.
type Spec struct {
	Scheme string     // "tcp", "serial", ...
	Target string     // the part after "scheme:" (and "//"), without the query
	Query  url.Values // options after '?'
}

// String formats s as a connection string.
func (s Spec) String() string {
	out := s.Scheme
	switch {
	case s.Scheme == "tcp" && s.Target != "":
		out += "://" + s.Target
	case s.Target != "":
		out += ":" + s.Target
	}
	if len(s.Query) > 0 {
		out += "?" + s.Query.Encode()
	}
	return out
}

// ParseSpec parses a connection string; see [Open] for the forms. A file:
// target is taken as is, query and all, since Windows device paths contain
// '?'.
func ParseSpec(s string) (Spec, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Spec{}, errors.New("escpos: empty connection string")
	}
	end := strings.IndexAny(s, ":?")
	if end < 0 {
		end = len(s)
	}
	spec := Spec{Scheme: strings.ToLower(s[:end]), Query: url.Values{}}
	rest := s[end:]
	if spec.Scheme == "file" {
		t := strings.TrimPrefix(rest, ":")
		// file:///dev/x and file:/dev/x are equivalent.
		if after, ok := strings.CutPrefix(t, "//"); ok && strings.HasPrefix(after, "/") {
			t = after
		}
		spec.Target = t
		return spec, nil
	}
	head, query, _ := strings.Cut(rest, "?")
	q, err := url.ParseQuery(query)
	if err != nil {
		return Spec{}, fmt.Errorf("escpos: connection %q: %w", s, err)
	}
	spec.Query = q
	spec.Target = strings.TrimPrefix(strings.TrimPrefix(head, ":"), "//")
	return spec, nil
}

var (
	schemesMu sync.RWMutex
	schemes   = map[string]Opener{}
)

// RegisterScheme makes Open handle connection strings starting with
// scheme. The serial and usb subpackages register "serial" and "libusb"
// when imported. It panics if scheme is already registered.
func RegisterScheme(scheme string, open Opener) {
	schemesMu.Lock()
	defer schemesMu.Unlock()
	scheme = strings.ToLower(scheme)
	if _, ok := schemes[scheme]; ok {
		panic("escpos: scheme " + scheme + " registered twice")
	}
	schemes[scheme] = open
}

// Schemes lists the schemes Open supports, including registered ones.
func Schemes() []string {
	schemesMu.RLock()
	defer schemesMu.RUnlock()
	out := []string{"discard", "file", "tcp", "usb"}
	for s := range schemes {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

func scheme(name string) Opener {
	schemesMu.RLock()
	defer schemesMu.RUnlock()
	return schemes[name]
}

// ErrUnknownScheme is returned by Open for a scheme it does not support,
// such as "serial" when the serial package has not been imported.
var ErrUnknownScheme = errors.New("escpos: unknown connection scheme")

// Open opens a printer connection described by a string, as found in a
// configuration file or environment variable:
//
//	usb                            auto-detect a USB printer
//	usb?vid=0fe6&pid=811e          ...narrowed by vendor and product ID
//	usb?serial=GD2076B8353DF1833   ...or by USB serial number
//	file:/dev/usb/lp0              a device node
//	tcp://192.168.1.50             Ethernet (port 9100 unless given)
//	serial:/dev/ttyUSB0?baud=9600  RS-232; needs package escpos/serial
//	libusb?vid=0fe6                USB via libusb; needs package escpos/usb
//	discard                        drops everything written (for testing)
//
// "usb" first uses the operating system's printer driver ([OpenUSB]: Linux
// and Windows, no cgo). If that is unsupported or finds nothing and package
// escpos/usb is imported, it falls back to libusb, which also covers macOS.
//
// Packages register more schemes with [RegisterScheme]; importing one for
// its side effect is enough:
//
//	import _ "github.com/connordoman/escpos/serial"
func Open(ctx context.Context, conn string) (io.WriteCloser, Connection, error) {
	spec, err := ParseSpec(conn)
	if err != nil {
		return nil, Connection{}, err
	}
	w, c, err := open(ctx, spec)
	if err != nil {
		return nil, c, err
	}
	_, c.Readable = w.(io.Reader)
	return w, c, nil
}

func open(ctx context.Context, spec Spec) (io.WriteCloser, Connection, error) {
	c := Connection{Kind: spec.Scheme, Target: spec.Target}
	switch spec.Scheme {
	case "discard", "none":
		c.Kind = "discard"
		return discard{}, c, nil
	case "file":
		if spec.Target == "" {
			return nil, c, errors.New("escpos: file connection needs a path, as in file:/dev/usb/lp0")
		}
		f, err := OpenFile(spec.Target)
		return f, c, err
	case "tcp":
		if spec.Target == "" {
			return nil, c, errors.New("escpos: tcp connection needs an address, as in tcp://192.168.1.50")
		}
		conn, err := DialTCP(ctx, spec.Target)
		return conn, c, err
	case "usb", "auto":
		c.Kind = "usb"
		match, err := USBMatch(spec.Query)
		if err != nil {
			return nil, c, err
		}
		f, p, err := OpenUSB(match)
		if err == nil {
			c.Target, c.USB = p.Path, &p
			return f, c, nil
		}
		lib := scheme("libusb")
		if lib == nil || !errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, ErrNoUSBPrinter) {
			return nil, c, err
		}
		w, lc, lerr := lib(ctx, spec)
		if lerr != nil {
			if errors.Is(err, errors.ErrUnsupported) {
				return nil, lc, lerr // the OS driver route never applied
			}
			return nil, lc, fmt.Errorf("%w; libusb: %w", err, lerr)
		}
		return w, lc, nil
	}
	if o := scheme(spec.Scheme); o != nil {
		return o(ctx, spec)
	}
	hint := ""
	switch spec.Scheme {
	case "serial":
		hint = ` (import _ "github.com/connordoman/escpos/serial")`
	case "libusb":
		hint = ` (import _ "github.com/connordoman/escpos/usb"; it needs cgo)`
	}
	return nil, c, fmt.Errorf("%w %q%s", ErrUnknownScheme, spec.Scheme, hint)
}

// USBMatch builds a [USBPrinter] filter from the query options of a "usb"
// or "libusb" connection string: vid and pid (hex) and serial. It returns
// nil if none are set.
func USBMatch(q url.Values) (func(USBPrinter) bool, error) {
	parse := func(key string) (uint16, error) {
		v := strings.TrimSpace(q.Get(key))
		if v == "" {
			return 0, nil
		}
		v = strings.TrimPrefix(strings.ToLower(v), "0x")
		n, err := strconv.ParseUint(v, 16, 16)
		if err != nil {
			return 0, fmt.Errorf("escpos: %s=%q is not a 16-bit hex ID", key, q.Get(key))
		}
		return uint16(n), nil
	}
	vid, err := parse("vid")
	if err != nil {
		return nil, err
	}
	pid, err := parse("pid")
	if err != nil {
		return nil, err
	}
	serial := q.Get("serial")
	if vid == 0 && pid == 0 && serial == "" {
		return nil, nil
	}
	return func(p USBPrinter) bool {
		return (vid == 0 || p.VendorID == vid) &&
			(pid == 0 || p.ProductID == pid) &&
			(serial == "" || p.Serial == serial)
	}, nil
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
func (discard) Close() error                { return nil }
