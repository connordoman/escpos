//go:build darwin || libusb

package main

import (
	"github.com/connordoman/escpos"
	"github.com/connordoman/escpos/usb" // also registers libusb: connections
)

func findLibusb() ([]escpos.USBPrinter, error) { return usb.Find() }
