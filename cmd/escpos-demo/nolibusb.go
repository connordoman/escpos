//go:build !darwin && !libusb

package main

import (
	"io"

	"github.com/connordoman/escpos"
)

func openLibusb(func(escpos.USBPrinter) bool) (io.ReadWriteCloser, escpos.USBPrinter, error) {
	return nil, escpos.USBPrinter{}, errNoLibusb
}

func findLibusb() ([]escpos.USBPrinter, error) { return nil, errNoLibusb }
