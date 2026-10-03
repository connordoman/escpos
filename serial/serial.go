// Package serial connects to ESC/POS printers over an RS-232 serial port.
//
// It is pure Go (no cgo) and works on Linux, macOS and Windows. On a
// Raspberry Pi a USB-to-serial adapter usually appears as /dev/ttyUSB0 and
// the GPIO UART as /dev/serial0.
package serial

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.bug.st/serial"
)

// Parity is a serial parity mode.
type Parity = serial.Parity

const (
	NoParity   = serial.NoParity
	OddParity  = serial.OddParity
	EvenParity = serial.EvenParity
)

// Options configures the port. The zero value selects 9600 baud, 8 data
// bits, no parity and 1 stop bit; check the printer's self-test page for the
// configured settings.
type Options struct {
	BaudRate int    // default 9600
	DataBits int    // 7 or 8, default 8
	Parity   Parity // default NoParity
	StopBits int    // 1 or 2, default 1
}

// pollInterval bounds how long a single read blocks, so ReadContext can
// notice cancellation.
const pollInterval = 50 * time.Millisecond

// Conn is a serial connection to a printer. It implements io.ReadWriteCloser
// and ReadContext, which escpos.Printer uses to bound status queries.
type Conn struct {
	port serial.Port
}

// Open opens and configures the serial port at path.
func Open(path string, opts Options) (*Conn, error) {
	mode := &serial.Mode{
		BaudRate: opts.BaudRate,
		DataBits: opts.DataBits,
		Parity:   opts.Parity,
		StopBits: serial.OneStopBit,
	}
	if mode.BaudRate == 0 {
		mode.BaudRate = 9600
	}
	if mode.DataBits == 0 {
		mode.DataBits = 8
	}
	switch opts.StopBits {
	case 0, 1:
	case 2:
		mode.StopBits = serial.TwoStopBits
	default:
		return nil, fmt.Errorf("serial: unsupported stop bits %d", opts.StopBits)
	}
	port, err := serial.Open(path, mode)
	if err != nil {
		return nil, fmt.Errorf("serial: opening %s: %w", path, err)
	}
	if err := port.SetReadTimeout(pollInterval); err != nil {
		port.Close()
		return nil, fmt.Errorf("serial: setting read timeout: %w", err)
	}
	return &Conn{port: port}, nil
}

// Write sends data to the printer.
func (c *Conn) Write(p []byte) (int, error) { return c.port.Write(p) }

// Read reads data sent by the printer, blocking until some arrives.
func (c *Conn) Read(p []byte) (int, error) { return c.ReadContext(context.Background(), p) }

// ReadContext reads data sent by the printer, giving up when ctx ends.
func (c *Conn) ReadContext(ctx context.Context, p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		n, err := c.port.Read(p)
		if n > 0 || err != nil {
			return n, err
		}
		// A zero-byte read without error is a timeout.
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
}

// Port returns the underlying port, for example to control DTR or RTS.
func (c *Conn) Port() serial.Port { return c.port }

// Close closes the port.
func (c *Conn) Close() error {
	if c.port == nil {
		return errors.New("serial: already closed")
	}
	err := c.port.Close()
	c.port = nil
	return err
}
