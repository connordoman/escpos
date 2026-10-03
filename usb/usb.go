// Package usb connects to ESC/POS printers over USB using libusb.
//
// It requires cgo and libusb-1.0 (on Raspberry Pi OS: apt install
// libusb-1.0-0-dev). On Linux the kernel's usblp driver is detached
// automatically while the connection is open. To avoid cgo entirely on
// Linux, use escpos.OpenFile("/dev/usb/lp0") instead.
//
// Accessing the device without root on Linux needs a udev rule such as:
//
//	SUBSYSTEM=="usb", ATTRS{idVendor}=="0fe6", ATTRS{idProduct}=="811e", MODE="0666"
package usb

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/gousb"
)

// IDs of the Rongta RP326 this package was developed against.
const (
	DefaultVendorID  = 0x0fe6
	DefaultProductID = 0x811e
)

// ErrNotFound is returned when no matching device is connected.
var ErrNotFound = errors.New("usb: printer not found")

// Options selects which device and interface to use. The zero value selects
// the Rongta RP326 IDs and discovers everything else.
type Options struct {
	VendorID  uint16 // default DefaultVendorID
	ProductID uint16 // default DefaultProductID

	// Serial selects a device by serial number when several with the same
	// IDs are connected. Empty means the first one found.
	Serial string

	// Config is the configuration number; 0 means the active one.
	Config int

	// Interface and Alternate select the interface; by default the first
	// interface with a bulk OUT endpoint is used.
	Interface int
	Alternate int
}

// Conn is a USB connection to a printer. It implements io.ReadWriteCloser as
// well as WriteContext and ReadContext, which escpos.Printer uses to bound
// I/O with contexts.
type Conn struct {
	ctx  *gousb.Context
	dev  *gousb.Device
	cfg  *gousb.Config
	intf *gousb.Interface
	out  *gousb.OutEndpoint
	in   *gousb.InEndpoint // nil if the interface has no bulk IN endpoint

	mu      sync.Mutex
	pending []byte // bytes from the last IN packet not yet returned
	packet  []byte
}

// Open opens the first matching printer.
func Open(opts Options) (*Conn, error) {
	if opts.VendorID == 0 {
		opts.VendorID = DefaultVendorID
	}
	if opts.ProductID == 0 {
		opts.ProductID = DefaultProductID
	}

	c := &Conn{ctx: gousb.NewContext()}
	devs, err := c.ctx.OpenDevices(func(d *gousb.DeviceDesc) bool {
		return uint16(d.Vendor) == opts.VendorID && uint16(d.Product) == opts.ProductID
	})
	for _, d := range devs {
		if c.dev == nil && (opts.Serial == "" || serialMatches(d, opts.Serial)) {
			c.dev = d
		} else {
			d.Close()
		}
	}
	if c.dev == nil {
		c.ctx.Close()
		if err != nil {
			return nil, fmt.Errorf("usb: opening %04x:%04x: %w", opts.VendorID, opts.ProductID, err)
		}
		return nil, fmt.Errorf("%w (%04x:%04x)", ErrNotFound, opts.VendorID, opts.ProductID)
	}
	if err := c.setup(opts); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func serialMatches(d *gousb.Device, serial string) bool {
	s, err := d.SerialNumber()
	return err == nil && s == serial
}

func (c *Conn) setup(opts Options) error {
	if err := c.dev.SetAutoDetach(true); err != nil {
		return fmt.Errorf("usb: enabling kernel driver auto-detach: %w", err)
	}
	cfgNum := opts.Config
	if cfgNum == 0 {
		n, err := c.dev.ActiveConfigNum()
		if err != nil {
			return fmt.Errorf("usb: reading active configuration: %w", err)
		}
		cfgNum = max(n, 1)
	}
	cfg, err := c.dev.Config(cfgNum)
	if err != nil {
		return fmt.Errorf("usb: claiming configuration %d: %w", cfgNum, err)
	}
	c.cfg = cfg

	intfNum, alt := opts.Interface, opts.Alternate
	if intfNum == 0 && alt == 0 {
		intfNum, alt = findPrinterInterface(cfg.Desc)
	}
	intf, err := cfg.Interface(intfNum, alt)
	if err != nil {
		return fmt.Errorf("usb: claiming interface %d/%d: %w", intfNum, alt, err)
	}
	c.intf = intf

	for _, ep := range intf.Setting.Endpoints {
		if ep.TransferType != gousb.TransferTypeBulk {
			continue
		}
		if ep.Direction == gousb.EndpointDirectionIn && c.in == nil {
			if c.in, err = intf.InEndpoint(ep.Number); err != nil {
				return fmt.Errorf("usb: opening IN endpoint %d: %w", ep.Number, err)
			}
			c.packet = make([]byte, max(ep.MaxPacketSize, 64))
		} else if ep.Direction == gousb.EndpointDirectionOut && c.out == nil {
			if c.out, err = intf.OutEndpoint(ep.Number); err != nil {
				return fmt.Errorf("usb: opening OUT endpoint %d: %w", ep.Number, err)
			}
		}
	}
	if c.out == nil {
		return fmt.Errorf("usb: interface %d/%d has no bulk OUT endpoint", intfNum, alt)
	}
	return nil
}

// findPrinterInterface returns the first interface setting with a bulk OUT
// endpoint, preferring the USB printer class.
func findPrinterInterface(desc gousb.ConfigDesc) (intf, alt int) {
	found := false
	for _, i := range desc.Interfaces {
		for _, s := range i.AltSettings {
			for _, ep := range s.Endpoints {
				if ep.TransferType != gousb.TransferTypeBulk || ep.Direction != gousb.EndpointDirectionOut {
					continue
				}
				if s.Class == gousb.ClassPrinter {
					return s.Number, s.Alternate
				}
				if !found {
					found, intf, alt = true, s.Number, s.Alternate
				}
			}
		}
	}
	return intf, alt
}

// Write sends data to the printer.
func (c *Conn) Write(p []byte) (int, error) {
	return c.WriteContext(context.Background(), p)
}

// WriteContext sends data to the printer, giving up when ctx ends.
func (c *Conn) WriteContext(ctx context.Context, p []byte) (int, error) {
	if c.out == nil {
		return 0, errors.New("usb: connection closed")
	}
	return c.out.WriteContext(ctx, p)
}

// Read reads data sent by the printer, blocking until some arrives.
func (c *Conn) Read(p []byte) (int, error) {
	return c.ReadContext(context.Background(), p)
}

// ReadContext reads data sent by the printer, giving up when ctx ends.
// Whole USB packets are read and buffered, so p may be as small as one byte.
func (c *Conn) ReadContext(ctx context.Context, p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.in == nil {
		return 0, errors.New("usb: printer has no IN endpoint")
	}
	for len(c.pending) == 0 {
		n, err := c.in.ReadContext(ctx, c.packet)
		c.pending = c.packet[:n]
		if err != nil && n == 0 {
			return 0, err
		}
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

// Device returns the underlying gousb device, for example to read its
// string descriptors.
func (c *Conn) Device() *gousb.Device { return c.dev }

// Close releases the interface and device.
func (c *Conn) Close() error {
	if c.intf != nil {
		c.intf.Close()
		c.intf = nil
	}
	var err error
	if c.cfg != nil {
		err = c.cfg.Close()
		c.cfg = nil
	}
	if c.dev != nil {
		err = errors.Join(err, c.dev.Close())
		c.dev = nil
	}
	if c.ctx != nil {
		err = errors.Join(err, c.ctx.Close())
		c.ctx = nil
	}
	c.out, c.in = nil, nil
	return err
}
