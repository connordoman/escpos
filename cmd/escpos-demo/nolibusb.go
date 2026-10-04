//go:build !darwin && !libusb

package main

import "github.com/connordoman/escpos"

func findLibusb() ([]escpos.USBPrinter, error) { return nil, errNoLibusb }
