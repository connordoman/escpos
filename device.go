package escpos

import (
	"context"
	"errors"
	"io"
	"sync"
)

// Device is a printer reached through a connection string ([Open]). It
// connects on first use and reconnects after the connection fails, so a
// long-running program keeps working when the printer is switched off,
// unplugged or comes back under a different device path. Use is
// serialised, so a Device is safe for concurrent use.
type Device struct {
	conn string
	opts []Option

	// OnConnect, if set, runs after each successful connection, for
	// example to [Printer.Identify] the printer. Set it before first use.
	OnConnect func(ctx context.Context, p *Printer, c Connection)

	mu      sync.Mutex
	printer *Printer
	closer  io.Closer
	info    Connection
}

// NewDevice returns a device for the connection string conn. opts configure
// the [Printer] created for each connection.
func NewDevice(conn string, opts ...Option) *Device {
	return &Device{conn: conn, opts: opts}
}

// ConnectionString returns the connection string the device opens.
func (d *Device) ConnectionString() string { return d.conn }

// connect opens the connection if needed. d.mu must be held.
func (d *Device) connect(ctx context.Context) error {
	if d.printer != nil {
		return nil
	}
	w, info, err := Open(ctx, d.conn)
	if err != nil {
		return err
	}
	d.printer, d.closer, d.info = New(w, d.opts...), w, info
	if d.OnConnect != nil {
		d.OnConnect(ctx, d.printer, info)
	}
	return nil
}

// Connect opens the connection if it is not open and describes it.
func (d *Device) Connect(ctx context.Context) (Connection, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.connect(ctx); err != nil {
		return Connection{}, err
	}
	return d.info, nil
}

// Connection describes the open connection, if there is one.
func (d *Device) Connection() (Connection, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.info, d.printer != nil
}

// Do connects if needed and runs fn with exclusive use of the printer.
//
// If fn fails, the connection is closed so the next call reopens it, unless
// the error shows the connection itself is fine: an invalid argument, a
// write-only connection, an invalid or missing reply, or ctx ending. Call
// [Device.Disconnect] to force a reconnection in those cases.
func (d *Device) Do(ctx context.Context, fn func(p *Printer) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.connect(ctx); err != nil {
		return err
	}
	err := fn(d.printer)
	if err != nil && !connectionOK(err) {
		d.disconnect()
	}
	return err
}

func connectionOK(err error) bool {
	var ise *InvalidStatusError
	return errors.Is(err, ErrInvalidArgument) || errors.Is(err, ErrNotReadable) ||
		errors.Is(err, ErrUnconfirmed) || errors.As(err, &ise) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// disconnect closes the connection. d.mu must be held.
func (d *Device) disconnect() {
	if d.closer != nil {
		d.closer.Close()
	}
	d.printer, d.closer, d.info = nil, nil, Connection{}
}

// Disconnect closes the connection; the next use reopens it.
func (d *Device) Disconnect() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.disconnect()
}

// Close closes the connection. The device can still be used afterwards,
// which reopens it.
func (d *Device) Close() error {
	d.Disconnect()
	return nil
}
