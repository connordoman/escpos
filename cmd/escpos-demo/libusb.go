//go:build darwin || libusb

package main

import (
	"io"

	"github.com/connordoman/escpos"
	"github.com/connordoman/escpos/usb"
)

func openLibusb(match func(escpos.USBPrinter) bool) (io.ReadWriteCloser, escpos.USBPrinter, error) {
	conn, err := usb.Open(usb.Options{Match: match})
	if err != nil {
		return nil, escpos.USBPrinter{}, err
	}
	return conn, conn.Info(), nil
}

func findLibusb() ([]escpos.USBPrinter, error) { return usb.Find() }
