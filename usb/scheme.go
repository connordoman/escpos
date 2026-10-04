package usb

import (
	"context"
	"io"

	"github.com/connordoman/escpos"
)

// Importing this package lets escpos.Open handle libusb?vid=0fe6&pid=811e
// connection strings, and lets "usb" fall back to libusb where the
// operating system's printer driver is unavailable (macOS) or finds
// nothing.
func init() {
	escpos.RegisterScheme("libusb", openSpec)
}

func openSpec(_ context.Context, spec escpos.Spec) (io.WriteCloser, escpos.Connection, error) {
	c := escpos.Connection{Kind: "libusb"}
	match, err := escpos.USBMatch(spec.Query)
	if err != nil {
		return nil, c, err
	}
	conn, err := Open(Options{Match: match})
	if err != nil {
		return nil, c, err
	}
	info := conn.Info()
	c.Target, c.USB = info.Path, &info
	return conn, c, nil
}
