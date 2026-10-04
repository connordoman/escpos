package main

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"strconv"

	"github.com/connordoman/escpos"
	_ "github.com/connordoman/escpos/serial" // registers serial: connections
)

// errNoLibusb is returned by findLibusb in builds without libusb support.
var errNoLibusb = errors.New("this build has no libusb support; rebuild with -tags libusb")

// connectionString turns the connection flags into an escpos.Open
// connection string.
func (o *options) connectionString() string {
	switch {
	case o.connection != "":
		return o.connection
	case o.tcp != "":
		return "tcp://" + o.tcp
	case o.serialPort != "":
		return "serial:" + o.serialPort + "?baud=" + strconv.Itoa(o.baud)
	case o.device != "":
		return "file:" + o.device
	}
	scheme := "usb"
	if o.libusb {
		scheme = "libusb"
	}
	q := url.Values{}
	for k, v := range map[string]string{"vid": o.vid, "pid": o.pid, "serial": o.usbSerial} {
		if v != "" {
			q.Set(k, v)
		}
	}
	if len(q) > 0 {
		scheme += "?" + q.Encode()
	}
	return scheme
}

// connect opens the connection selected by the flags.
func (o *options) connect(ctx context.Context) (io.Writer, error) {
	switch {
	case o.dryRun:
		return hexDumper{hex.Dumper(os.Stdout)}, nil
	case o.output != "":
		o.logf("writing commands to %s", o.output)
		return os.Create(o.output)
	}
	conn := o.connectionString()
	o.logf("opening %s", conn)
	w, c, err := escpos.Open(ctx, conn)
	if err != nil {
		return nil, err
	}
	if c.USB != nil {
		o.logf("connected to %v via %s", *c.USB, c.Kind)
	} else {
		o.logf("connected to %s (%s)", c.Target, c.Kind)
	}
	return w, nil
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
