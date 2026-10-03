package escpos

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// IDs of the Rongta RP326 this package was developed against.
const (
	RongtaVendorID  = 0x0fe6
	RongtaProductID = 0x811e
)

// ErrNoUSBPrinter is returned by [OpenUSB] and [SelectUSBPrinter] when no
// suitable printer is found.
var ErrNoUSBPrinter = errors.New("escpos: no USB printer found")

// ErrUSBDetectUnsupported is returned by [FindUSBPrinters] on platforms
// without a cgo-free way to find USB printers (macOS and others). It wraps
// [errors.ErrUnsupported]. Use the escpos/usb package there instead.
var ErrUSBDetectUnsupported = fmt.Errorf("escpos: USB printer detection without libusb is only supported on Linux and Windows; use the escpos/usb package: %w", errors.ErrUnsupported)

// USBPrinter describes a connected USB printer.
type USBPrinter struct {
	// Path identifies the printer to the operating system: a device node
	// such as /dev/usb/lp0 on Linux, a device interface path such as
	// \\?\USB#VID_0FE6&PID_811E#...#{28d78fad-...} on Windows, or
	// "bus:address" for printers found by the escpos/usb package.
	Path string

	VendorID     uint16
	ProductID    uint16
	Manufacturer string // USB string descriptors; may be empty
	Product      string
	Serial       string

	// DeviceID is the IEEE 1284 device ID the printer reports, e.g.
	// "MFG:Rongta;CMD:ESC/POS;MDL:RP326;". It may be empty.
	DeviceID string
}

func (u USBPrinter) String() string {
	name := strings.TrimSpace(u.Manufacturer + " " + u.Product)
	if name == "" {
		name = "USB printer"
	}
	return fmt.Sprintf("%s (%04x:%04x) at %s", name, u.VendorID, u.ProductID, u.Path)
}

// Platform implementations, replaced in tests.
var (
	findUSBPrinters = platformFindUSBPrinters
	diagnoseNoUSB   = diagnoseNoPrinter
)

// FindUSBPrinters lists connected USB printers using the operating system's
// own printer driver, without cgo or libusb:
//
//   - Linux: printers bound to the usblp driver, found through sysfs.
//   - Windows: printers bound to the built-in usbprint driver, found through
//     the device interface list.
//
// Other platforms return [ErrUSBDetectUnsupported].
func FindUSBPrinters() ([]USBPrinter, error) {
	return findUSBPrinters()
}

// SelectUSBPrinter chooses one of printers. If match is nil, a Rongta RP326
// is preferred; failing that, a printer is chosen only if it is the sole one
// connected. Otherwise the first printer match accepts is chosen.
func SelectUSBPrinter(printers []USBPrinter, match func(USBPrinter) bool) (USBPrinter, error) {
	if match != nil {
		for _, p := range printers {
			if match(p) {
				return p, nil
			}
		}
		return USBPrinter{}, fmt.Errorf("%w: none of %d connected printers matched", ErrNoUSBPrinter, len(printers))
	}
	for _, p := range printers {
		if p.VendorID == RongtaVendorID && p.ProductID == RongtaProductID {
			return p, nil
		}
	}
	switch len(printers) {
	case 0:
		return USBPrinter{}, ErrNoUSBPrinter
	case 1:
		return printers[0], nil
	}
	return USBPrinter{}, fmt.Errorf("%w: %d printers connected, pass a match function to choose: %v",
		ErrNoUSBPrinter, len(printers), printers)
}

// OpenUSB finds a USB printer with [FindUSBPrinters], chooses one with
// [SelectUSBPrinter] and opens it, so no device path needs configuring. It
// works on Linux and Windows without cgo; on macOS use the escpos/usb
// package, whose Open also detects the printer.
//
// The device path can change when the printer is unplugged and reconnected,
// so a long-running server should call OpenUSB again after a write fails (or
// simply per job) rather than holding one file forever.
func OpenUSB(match func(USBPrinter) bool) (*os.File, USBPrinter, error) {
	printers, err := FindUSBPrinters()
	if err != nil {
		return nil, USBPrinter{}, err
	}
	p, err := SelectUSBPrinter(printers, match)
	if err != nil {
		if len(printers) == 0 {
			err = diagnoseNoUSB()
		}
		return nil, USBPrinter{}, err
	}
	f, err := OpenFile(p.Path)
	if err != nil {
		return nil, p, fmt.Errorf("escpos: opening %v: %w", p, err)
	}
	return f, p, nil
}
