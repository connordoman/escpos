//go:build !linux && !windows

package escpos

func platformFindUSBPrinters() ([]USBPrinter, error) { return nil, ErrUSBDetectUnsupported }

func diagnoseNoPrinter() error { return ErrNoUSBPrinter }
