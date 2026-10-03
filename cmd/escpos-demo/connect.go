package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/connordoman/escpos"
	"github.com/connordoman/escpos/serial"
)

// errNoLibusb is returned by openLibusb in builds without libusb support.
var errNoLibusb = errors.New("this build has no libusb support; rebuild with -tags libusb")

// usbFilter returns a match function for the --vid, --pid and --usb-serial
// flags, or nil if none were given.
func (o *options) usbFilter() (func(escpos.USBPrinter) bool, error) {
	if o.vid == "" && o.pid == "" && o.usbSerial == "" {
		return nil, nil
	}
	vid, err := parseID("--vid", o.vid)
	if err != nil {
		return nil, err
	}
	pid, err := parseID("--pid", o.pid)
	if err != nil {
		return nil, err
	}
	return func(p escpos.USBPrinter) bool {
		return (vid == 0 || p.VendorID == vid) &&
			(pid == 0 || p.ProductID == pid) &&
			(o.usbSerial == "" || p.Serial == o.usbSerial)
	}, nil
}

func parseID(flag, s string) (uint16, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 16)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a 4-digit hex ID", flag, s)
	}
	return uint16(v), nil
}

// connect opens the connection selected by the flags.
func (o *options) connect(ctx context.Context) (io.Writer, error) {
	switch {
	case o.dryRun:
		return hexDumper{hex.Dumper(os.Stdout)}, nil
	case o.output != "":
		o.logf("writing commands to %s", o.output)
		return os.Create(o.output)
	case o.tcp != "":
		o.logf("connecting to %s over TCP", o.tcp)
		return escpos.DialTCP(ctx, o.tcp)
	case o.serialPort != "":
		o.logf("opening serial port %s at %d baud", o.serialPort, o.baud)
		return serial.Open(o.serialPort, serial.Options{BaudRate: o.baud})
	case o.device != "":
		o.logf("opening %s", o.device)
		return escpos.OpenFile(o.device)
	}

	match, err := o.usbFilter()
	if err != nil {
		return nil, err
	}
	if !o.libusb {
		f, p, err := escpos.OpenUSB(match)
		if err == nil {
			o.logf("connected to %v", p)
			return f, nil
		}
		// Fall back to libusb where the OS driver route is unavailable, or
		// found nothing (the printer may not use the printer driver).
		if !errors.Is(err, errors.ErrUnsupported) && !errors.Is(err, escpos.ErrNoUSBPrinter) {
			return nil, err
		}
		o.logf("OS printer driver: %v; trying libusb", err)
		conn, info, lerr := openLibusb(match)
		if errors.Is(lerr, errNoLibusb) {
			return nil, err
		}
		if lerr != nil {
			return nil, fmt.Errorf("%w; libusb: %w", err, lerr)
		}
		o.logf("connected to %v via libusb", info)
		return conn, nil
	}
	conn, info, err := openLibusb(match)
	if err != nil {
		return nil, err
	}
	o.logf("connected to %v via libusb", info)
	return conn, nil
}

// listUSB lists printers found by every available detection method.
func listUSB() (osDriver, libusb []escpos.USBPrinter, osErr, libErr error) {
	osDriver, osErr = escpos.FindUSBPrinters()
	libusb, libErr = findLibusb()
	return
}

// hexDumper writes a hex dump and closes the dumper on Close so the final
// line is printed.
type hexDumper struct{ io.WriteCloser }
