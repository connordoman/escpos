package escpos

func platformFindUSBPrinters() ([]USBPrinter, error) { return findSysfsPrinters() }

func diagnoseNoPrinter() error { return diagnoseSysfs() }
