package escpos

import (
	"fmt"
	"sort"

	"golang.org/x/sys/windows"
)

// guidUSBPrint is GUID_DEVINTERFACE_USBPRINT, the device interface class
// registered by Windows' built-in usbprint.sys driver for every USB printer.
var guidUSBPrint = windows.GUID{
	Data1: 0x28d78fad,
	Data2: 0x5a12,
	Data3: 0x11d1,
	Data4: [8]byte{0xae, 0x5b, 0x00, 0x00, 0xf8, 0x03, 0xa8, 0xc2},
}

// platformFindUSBPrinters lists present usbprint device interfaces. Their
// paths can be opened like files for raw reading and writing, bypassing the
// print spooler, so no printer queue or vendor driver is required.
// Manufacturer, Product and DeviceID are not filled in.
func platformFindUSBPrinters() ([]USBPrinter, error) {
	paths, err := windows.CM_Get_Device_Interface_List("", &guidUSBPrint, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return nil, fmt.Errorf("escpos: listing USB printers: %w", err)
	}
	var printers []USBPrinter
	for _, path := range paths {
		if p, ok := parseUSBPrintPath(path); ok {
			printers = append(printers, p)
		}
	}
	sort.Slice(printers, func(i, j int) bool { return printers[i].Path < printers[j].Path })
	return printers, nil
}

func diagnoseNoPrinter() error {
	return fmt.Errorf("%w: no device uses the Windows USB printer driver (usbprint); "+
		"if a WinUSB/libusb driver was installed for the printer (e.g. with Zadig), use the escpos/usb package instead",
		ErrNoUSBPrinter)
}
