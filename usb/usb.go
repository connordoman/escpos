// Package usb connects to ESC/POS printers over USB using libusb. It works
// on Linux, macOS and Windows, and can detect the printer automatically.
//
// It requires cgo and libusb-1.0:
//
//   - Raspberry Pi OS / Debian: apt install libusb-1.0-0-dev
//   - macOS: brew install libusb
//   - Windows: libusb from MSYS2 (pacman -S mingw-w64-x86_64-libusb), and the
//     printer must use the WinUSB driver (installed with Zadig) instead of
//     the built-in usbprint driver.
//
// On Linux the kernel's usblp driver is detached automatically while the
// connection is open. On Linux and Windows, escpos.OpenUSB detects and opens
// the printer through the operating system's own driver without cgo, and is
// usually the simpler choice there; this package is the way to go on macOS.
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

	"github.com/connordoman/escpos"
	"github.com/google/gousb"
)

// IDs of the Rongta RP326 this package was developed against.
const (
	DefaultVendorID  = escpos.RongtaVendorID
	DefaultProductID = escpos.RongtaProductID
)

// ErrNotFound is returned when no matching device is connected. It is the
// same error as escpos.ErrNoUSBPrinter.
var ErrNotFound = escpos.ErrNoUSBPrinter

// Options selects which device and interface to use. The zero value detects
// the printer and discovers everything else.
type Options struct {
	// VendorID and ProductID select devices by ID. If both are zero, the
	// printer is detected instead: every device with a USB printer-class
	// interface, or with the Rongta RP326 IDs, is a candidate, and one is
	// chosen by escpos.SelectUSBPrinter using Match.
	VendorID  uint16
	ProductID uint16

	// Match chooses among detected printers when VendorID and ProductID are
	// zero. Nil prefers a Rongta RP326, or else the only printer connected.
	Match func(escpos.USBPrinter) bool

	// Serial selects a device by serial number when several match. Empty
	// means the first one found.
	Serial string

	// Config is the configuration number; 0 means the active one.
	Config int

	// Interface and Alternate select the interface; by default the first
	// interface with a bulk OUT endpoint is used, preferring the printer
	// class.
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

	info escpos.USBPrinter

	mu      sync.Mutex
	pending []byte // bytes from the last IN packet not yet returned
	packet  []byte
}

// Find lists connected USB printers: devices with a printer-class interface
// or the Rongta RP326 IDs. Devices libusb cannot open (for example ones
// using Windows' usbprint driver) are skipped.
func Find() ([]escpos.USBPrinter, error) {
	ctx := gousb.NewContext()
	defer ctx.Close()
	devs, infos, err := openCandidates(ctx, isPrinter)
	for _, d := range devs {
		d.Close()
	}
	if len(infos) == 0 && err != nil {
		return nil, fmt.Errorf("usb: listing devices: %w", err)
	}
	return infos, nil
}

// Open opens a printer selected by opts.
func Open(opts Options) (*Conn, error) {
	c := &Conn{ctx: gousb.NewContext()}
	if err := c.open(opts); err != nil {
		c.Close()
		return nil, err
	}
	if err := c.setup(opts); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Conn) open(opts Options) error {
	detect := opts.VendorID == 0 && opts.ProductID == 0
	filter := isPrinter
	if !detect {
		filter = func(d *gousb.DeviceDesc) bool {
			return (opts.VendorID == 0 || uint16(d.Vendor) == opts.VendorID) &&
				(opts.ProductID == 0 || uint16(d.Product) == opts.ProductID)
		}
	}
	devs, infos, err := openCandidates(c.ctx, filter)
	defer func() {
		for _, d := range devs {
			if d != c.dev {
				d.Close()
			}
		}
	}()

	var candidates []escpos.USBPrinter
	for _, info := range infos {
		if opts.Serial == "" || info.Serial == opts.Serial {
			candidates = append(candidates, info)
		}
	}
	var chosen escpos.USBPrinter
	var selErr error
	if detect {
		chosen, selErr = escpos.SelectUSBPrinter(candidates, opts.Match)
	} else if len(candidates) > 0 {
		chosen = candidates[0]
	} else {
		selErr = fmt.Errorf("%w (%04x:%04x)", ErrNotFound, opts.VendorID, opts.ProductID)
	}
	if selErr != nil {
		if len(infos) == 0 && err != nil {
			return fmt.Errorf("usb: opening devices: %w", err)
		}
		return selErr
	}
	for i, info := range infos {
		if info.Path == chosen.Path {
			c.dev, c.info = devs[i], info
		}
	}
	return nil
}

// isPrinter reports whether a device looks like a printer.
func isPrinter(d *gousb.DeviceDesc) bool {
	if uint16(d.Vendor) == DefaultVendorID && uint16(d.Product) == DefaultProductID {
		return true
	}
	if d.Class == gousb.ClassPrinter {
		return true
	}
	for _, cfg := range d.Configs {
		for _, intf := range cfg.Interfaces {
			for _, alt := range intf.AltSettings {
				if alt.Class == gousb.ClassPrinter {
					return true
				}
			}
		}
	}
	return false
}

// openCandidates opens every device accepted by filter and describes each.
// devs and infos are parallel. err reports devices that could not be opened.
func openCandidates(ctx *gousb.Context, filter func(*gousb.DeviceDesc) bool) ([]*gousb.Device, []escpos.USBPrinter, error) {
	devs, err := ctx.OpenDevices(filter)
	infos := make([]escpos.USBPrinter, len(devs))
	for i, d := range devs {
		infos[i] = describe(d)
	}
	return devs, infos, err
}

func describe(d *gousb.Device) escpos.USBPrinter {
	info := escpos.USBPrinter{
		Path:      fmt.Sprintf("%d:%d", d.Desc.Bus, d.Desc.Address),
		VendorID:  uint16(d.Desc.Vendor),
		ProductID: uint16(d.Desc.Product),
	}
	info.Manufacturer, _ = d.Manufacturer()
	info.Product, _ = d.Product()
	info.Serial, _ = d.SerialNumber()
	return info
}

// Info describes the connected printer. Its Path is "bus:address".
func (c *Conn) Info() escpos.USBPrinter { return c.info }

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
