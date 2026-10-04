package escpos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultTimeout bounds status queries whose context has no deadline.
const DefaultTimeout = 2 * time.Second

// DefaultChunkSize is the largest write passed to the connection at once.
const DefaultChunkSize = 512

// ErrNotReadable is returned by status queries when the connection cannot be
// read from.
var ErrNotReadable = errors.New("escpos: connection is write-only")

// Printer sends commands to a printer over a connection.
//
// Printer embeds a [Builder], so commands can be appended to it directly and
// sent with [Printer.Flush]. Alternatively, jobs can be prepared in separate
// Builders and sent with [Printer.Send], which lets an application build jobs
// concurrently and only serialize the I/O.
//
// Flush, Send and the query methods are safe for concurrent use. The
// embedded Builder is not; guard it yourself if several goroutines use it.
type Printer struct {
	Builder

	mu        sync.Mutex
	conn      io.Writer
	chunkSize int
	timeout   time.Duration

	rbuf     []byte          // bytes read but not yet consumed
	inflight chan readResult // pending read on a connection without deadlines

	processSeq atomic.Uint32 // IDs for SendConfirmed
}

type readResult struct {
	data []byte
	err  error
}

// Option configures a [Printer].
type Option func(*Printer)

// WithPaperWidth sets the printable width in dots that layout helpers assume
// (default [PaperWidth80mm]).
func WithPaperWidth(dots int) Option {
	return func(p *Printer) { p.paperWidth = dots }
}

// WithChunkSize sets the largest write passed to the connection at once
// (default [DefaultChunkSize]).
func WithChunkSize(n int) Option {
	return func(p *Printer) { p.chunkSize = n }
}

// WithTimeout sets how long status queries wait for a response when their
// context has no deadline (default [DefaultTimeout]).
func WithTimeout(d time.Duration) Option {
	return func(p *Printer) { p.timeout = d }
}

// New returns a Printer that writes to conn. conn may also implement
// [io.Reader], which status queries require, and [io.Closer], which
// [Printer.Close] calls.
//
// Connections that implement WriteContext/ReadContext (like the usb
// package's) or SetWriteDeadline/SetReadDeadline (like [net.Conn] and
// [os.File]) have their I/O bounded by the context passed to each call.
func New(conn io.Writer, opts ...Option) *Printer {
	p := &Printer{conn: conn, chunkSize: DefaultChunkSize, timeout: DefaultTimeout}
	for _, o := range opts {
		o(p)
	}
	if p.chunkSize <= 0 {
		p.chunkSize = DefaultChunkSize
	}
	return p
}

// Conn returns the underlying connection.
func (p *Printer) Conn() io.Writer { return p.conn }

// Flush sends the commands buffered in the embedded Builder. On success the
// buffer is emptied; on error the unsent remainder is kept so the caller can
// retry or [Builder.Reset] it. It returns the number of bytes sent.
func (p *Printer) Flush(ctx context.Context) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n, err := p.write(ctx, p.buf)
	p.buf = p.buf[:copy(p.buf, p.buf[n:])]
	return n, err
}

// Send sends the commands buffered in b without modifying it. It returns the
// number of bytes sent.
func (p *Printer) Send(ctx context.Context, b *Builder) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.write(ctx, b.buf)
}

// SendRaw sends data immediately, bypassing the embedded Builder.
func (p *Printer) SendRaw(ctx context.Context, data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.write(ctx, data)
}

// Close closes the connection if it implements [io.Closer]. Buffered
// commands that have not been flushed are discarded.
func (p *Printer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf = nil
	if c, ok := p.conn.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

type contextWriter interface {
	WriteContext(ctx context.Context, p []byte) (int, error)
}

type contextReader interface {
	ReadContext(ctx context.Context, p []byte) (int, error)
}

type writeDeadliner interface {
	SetWriteDeadline(t time.Time) error
}

type readDeadliner interface {
	SetReadDeadline(t time.Time) error
}

// write sends data in chunks. The caller must hold p.mu.
func (p *Printer) write(ctx context.Context, data []byte) (int, error) {
	if p.conn == nil {
		return 0, errors.New("escpos: printer has no connection")
	}
	if dw, ok := p.conn.(writeDeadliner); ok {
		d, _ := ctx.Deadline()
		if dw.SetWriteDeadline(d) == nil {
			defer dw.SetWriteDeadline(time.Time{})
			stop := context.AfterFunc(ctx, func() { dw.SetWriteDeadline(time.Now()) })
			defer stop()
		}
	}
	written := 0
	for written < len(data) {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		chunk := data[written:min(written+p.chunkSize, len(data))]
		var n int
		var err error
		if cw, ok := p.conn.(contextWriter); ok {
			n, err = cw.WriteContext(ctx, chunk)
		} else {
			n, err = p.conn.Write(chunk)
		}
		written += n
		if err != nil {
			return written, contextError(ctx, err)
		}
		if n < len(chunk) {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

// contextError attributes an I/O error to ctx when ctx ended or its deadline
// (applied to the connection) caused the error.
func contextError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return fmt.Errorf("%w: %w", cerr, err)
	}
	if _, ok := ctx.Deadline(); ok && errors.Is(err, os.ErrDeadlineExceeded) {
		return fmt.Errorf("%w: %w", context.DeadlineExceeded, err)
	}
	return err
}

// withTimeout applies the default timeout to contexts without a deadline.
func (p *Printer) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok || p.timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, p.timeout)
}

// read reads at least one byte into buf. The caller must hold p.mu.
func (p *Printer) read(ctx context.Context, buf []byte) (int, error) {
	if len(p.rbuf) > 0 {
		n := copy(buf, p.rbuf)
		p.rbuf = p.rbuf[n:]
		return n, nil
	}
	r, ok := p.conn.(io.Reader)
	if !ok {
		return 0, ErrNotReadable
	}
	if cr, ok := p.conn.(contextReader); ok {
		return cr.ReadContext(ctx, buf)
	}
	if dr, ok := p.conn.(readDeadliner); ok {
		d, _ := ctx.Deadline()
		if err := dr.SetReadDeadline(d); err == nil {
			defer dr.SetReadDeadline(time.Time{})
			stop := context.AfterFunc(ctx, func() { dr.SetReadDeadline(time.Now()) })
			defer stop()
			n, err := r.Read(buf)
			return n, contextError(ctx, err)
		}
	}

	// The connection cannot be interrupted, so read in a goroutine. If the
	// context ends first, the pending read is picked up by the next call
	// rather than losing its data.
	if p.inflight == nil {
		ch := make(chan readResult, 1)
		p.inflight = ch
		size := max(len(buf), 64)
		go func() {
			tmp := make([]byte, size)
			n, err := r.Read(tmp)
			ch <- readResult{tmp[:n], err}
		}()
	}
	select {
	case res := <-p.inflight:
		p.inflight = nil
		n := copy(buf, res.data)
		p.rbuf = append(p.rbuf, res.data[n:]...)
		if n > 0 {
			return n, nil
		}
		return 0, res.err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// readFull reads exactly len(buf) bytes. The caller must hold p.mu.
func (p *Printer) readFull(ctx context.Context, buf []byte) error {
	for got := 0; got < len(buf); {
		n, err := p.read(ctx, buf[got:])
		got += n
		if err != nil && got < len(buf) {
			return err
		}
	}
	return nil
}

// query sends cmd and reads an n-byte response.
func (p *Printer) query(ctx context.Context, cmd []byte, n int) ([]byte, error) {
	ctx, cancel := p.withTimeout(ctx)
	defer cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.write(ctx, cmd); err != nil {
		return nil, err
	}
	resp := make([]byte, n)
	if err := p.readFull(ctx, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// readUntilNUL reads up to and excluding a NUL byte. The caller must hold
// p.mu.
func (p *Printer) readUntilNUL(ctx context.Context) ([]byte, error) {
	var out []byte
	var c [1]byte
	for {
		if err := p.readFull(ctx, c[:]); err != nil {
			return out, err
		}
		if c[0] == NUL {
			return out, nil
		}
		out = append(out, c[0])
	}
}
