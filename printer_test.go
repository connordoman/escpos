package escpos

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakePrinter answers queries over one end of a net.Pipe, which supports
// deadlines like a TCP connection.
func fakePrinter(t *testing.T, respond func(cmd []byte) []byte) (net.Conn, *bytes.Buffer, *sync.Mutex) {
	t.Helper()
	host, dev := net.Pipe()
	var mu sync.Mutex
	var received bytes.Buffer
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := dev.Read(buf)
			if err != nil {
				return
			}
			mu.Lock()
			received.Write(buf[:n])
			mu.Unlock()
			if resp := respond(buf[:n]); resp != nil {
				dev.Write(resp)
			}
		}
	}()
	t.Cleanup(func() { host.Close(); dev.Close() })
	return host, &received, &mu
}

func TestFlushAndSend(t *testing.T) {
	var out bytes.Buffer
	p := New(&out, WithChunkSize(3))
	p.Initialize()
	p.Textln("hello")
	n, err := p.Flush(context.Background())
	if err != nil || n != 8 {
		t.Fatalf("Flush = %d, %v", n, err)
	}
	if p.Len() != 0 {
		t.Errorf("buffer not emptied after Flush")
	}

	job := NewBuilder(0)
	job.FeedAndCut(0)
	if _, err := p.Send(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if job.Len() != 4 {
		t.Errorf("Send modified the job")
	}
	want := []byte{ESC, '@', 'h', 'e', 'l', 'l', 'o', LF, GS, 'V', 66, 0}
	if !bytes.Equal(out.Bytes(), want) {
		t.Errorf("got % X, want % X", out.Bytes(), want)
	}
}

type failingWriter struct{ limit int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.limit <= 0 {
		return 0, errors.New("unplugged")
	}
	n := min(len(p), w.limit)
	w.limit -= n
	return n, nil
}

func TestFlushKeepsRemainderOnError(t *testing.T) {
	p := New(&failingWriter{limit: 4}, WithChunkSize(2))
	p.Text("abcdefgh")
	n, err := p.Flush(context.Background())
	if err == nil || n != 4 {
		t.Fatalf("Flush = %d, %v; want 4 and an error", n, err)
	}
	if got := string(p.Bytes()); got != "efgh" {
		t.Errorf("remaining buffer %q, want %q", got, "efgh")
	}
}

func TestStatus(t *testing.T) {
	responses := map[byte]byte{1: 0x16, 2: 0x16, 3: 0x12, 4: 0x72}
	conn, received, mu := fakePrinter(t, func(cmd []byte) []byte {
		if len(cmd) == 3 && cmd[0] == DLE && cmd[1] == EOT {
			return []byte{responses[cmd[2]]}
		}
		return nil
	})
	p := New(conn)
	s, err := p.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !s.Printer.DrawerSignalHigh() || !s.Offline.CoverOpen() || s.Error.AutoCutterError() || !s.Paper.PaperEnd() {
		t.Errorf("decoded status wrong: %+v", s)
	}
	if s.Ready() {
		t.Errorf("Ready() = true with cover open and no paper")
	}
	mu.Lock()
	defer mu.Unlock()
	want := []byte{DLE, EOT, 1, DLE, EOT, 2, DLE, EOT, 3, DLE, EOT, 4}
	if !bytes.Equal(received.Bytes(), want) {
		t.Errorf("sent % X, want % X", received.Bytes(), want)
	}
}

func TestStatusInvalidByte(t *testing.T) {
	conn, _, _ := fakePrinter(t, func([]byte) []byte { return []byte{0xFF} })
	_, err := New(conn).RealTimeStatus(context.Background(), StatusPrinter)
	var ise *InvalidStatusError
	if !errors.As(err, &ise) || ise.Byte != 0xFF {
		t.Fatalf("got %v, want InvalidStatusError", err)
	}
}

func TestStatusTimeout(t *testing.T) {
	conn, _, _ := fakePrinter(t, func([]byte) []byte { return nil })
	p := New(conn, WithTimeout(50*time.Millisecond))
	start := time.Now()
	_, err := p.RealTimeStatus(context.Background(), StatusPrinter)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("timeout took %v", time.Since(start))
	}
}

func TestPrinterInfo(t *testing.T) {
	conn, _, _ := fakePrinter(t, func(cmd []byte) []byte {
		if bytes.Equal(cmd, []byte{GS, 'I', 67}) {
			return []byte("_RP326\x00")
		}
		if bytes.Equal(cmd, []byte{GS, 'I', 2}) {
			return []byte{0x02}
		}
		return nil
	})
	p := New(conn)
	name, err := p.PrinterInfo(context.Background(), PrinterInfoName)
	if err != nil || name != "RP326" {
		t.Fatalf("PrinterInfo = %q, %v", name, err)
	}
	id, err := p.PrinterID(context.Background(), PrinterIDTypeID)
	if err != nil || !TypeID(id).AutoCutter() || TypeID(id).MultiByte() {
		t.Fatalf("PrinterID = %#x, %v", id, err)
	}
}

func TestWaitProcessID(t *testing.T) {
	conn, _, _ := fakePrinter(t, func(cmd []byte) []byte {
		if len(cmd) == 11 && cmd[2] == 'H' {
			return append([]byte{0x37, 0x22, 'O', 'L', 'D', '0', 0, 0x37, 0x22}, append(cmd[7:11], 0)...)
		}
		return nil
	})
	p := New(conn)
	id := [4]byte{'J', 'O', 'B', '1'}
	if err := p.SetProcessIDResponse(id); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.WaitProcessID(context.Background(), id); err != nil {
		t.Fatal(err)
	}
}

func TestRealTimeCommands(t *testing.T) {
	var out bytes.Buffer
	p := New(&out)
	p.Text("buffered")
	ctx := context.Background()
	if err := p.RecoverFromError(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := p.RealTimePulse(ctx, DrawerPin2, 2); err != nil {
		t.Fatal(err)
	}
	if err := p.RealTimePulse(ctx, DrawerPin2, 9); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("t=9: got %v, want ErrInvalidArgument", err)
	}
	want := []byte{DLE, ENQ, 2, DLE, DC4, 1, 0, 2}
	if !bytes.Equal(out.Bytes(), want) {
		t.Errorf("got % X, want % X", out.Bytes(), want)
	}
	if string(p.Bytes()) != "buffered" {
		t.Errorf("real-time commands disturbed the buffer")
	}
}

// blockingReader has no deadline support, exercising the goroutine fallback.
type blockingReader struct {
	io.Writer
	r io.Reader
}

func (b blockingReader) Read(p []byte) (int, error) { return b.r.Read(p) }

func TestReadFallbackKeepsLateData(t *testing.T) {
	pr, pw := io.Pipe()
	p := New(blockingReader{io.Discard, pr}, WithTimeout(20*time.Millisecond))
	if _, err := p.RealTimeStatus(context.Background(), StatusPrinter); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
	// The response to the timed-out query arrives late, followed by the
	// response to the next one; neither may be lost.
	go pw.Write([]byte{0x12, 0x16})
	b1, err := p.RealTimeStatus(context.Background(), StatusPrinter)
	if err != nil || b1 != 0x12 {
		t.Fatalf("first = %#x, %v", b1, err)
	}
	b2, err := p.RealTimeStatus(context.Background(), StatusPrinter)
	if err != nil || b2 != 0x16 {
		t.Fatalf("second = %#x, %v", b2, err)
	}
}

func TestWriteOnlyConnection(t *testing.T) {
	_, err := New(io.Discard).RealTimeStatus(context.Background(), StatusPrinter)
	if !errors.Is(err, ErrNotReadable) {
		t.Fatalf("got %v, want ErrNotReadable", err)
	}
}

func TestFlushHonoursContext(t *testing.T) {
	host, dev := net.Pipe() // nobody reads dev, so writes block
	defer host.Close()
	defer dev.Close()
	p := New(host)
	p.Text("stuck")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := p.Flush(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
}

func TestSendConfirmed(t *testing.T) {
	conn, received, mu := fakePrinter(t, func(cmd []byte) []byte {
		if i := bytes.Index(cmd, []byte{GS, '(', 'H'}); i >= 0 && len(cmd) >= i+11 {
			return append([]byte{0x37, 0x22}, append(cmd[i+7:i+11], 0)...)
		}
		return nil
	})
	p := New(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.SendConfirmed(ctx, []byte("job\n")); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := received.String()
	mu.Unlock()
	if !strings.HasPrefix(got, "job\n\x1d(H") {
		t.Errorf("sent %q", got)
	}
}

func TestSendConfirmedUnconfirmed(t *testing.T) {
	conn, _, _ := fakePrinter(t, func([]byte) []byte { return nil })
	p := New(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := p.SendConfirmed(ctx, []byte("job\n")); !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("got %v, want ErrUnconfirmed", err)
	}
	if err := New(io.Discard).SendConfirmed(context.Background(), nil); !errors.Is(err, ErrNotReadable) {
		t.Errorf("write-only: got %v", err)
	}
}

func TestStatusString(t *testing.T) {
	if s := (Status{Printer: 0x16, Offline: 0x12, Error: 0x12, Paper: 0x12}).String(); s != "ready" {
		t.Errorf("ready status: %q", s)
	}
	if s := (Status{Offline: 0x12 | 0x04 | 0x40, Paper: 0x12 | 0x60}).String(); s != "cover open, out of paper" {
		t.Errorf("got %q", s)
	}
}
