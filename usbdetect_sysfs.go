package escpos

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Linux detection reads sysfs. It is not build-constrained so it can be
// tested on any platform against a fake tree.

// Roots of the sysfs and device trees, replaced in tests.
var (
	sysfsRoot = "/sys"
	devRoot   = "/dev"
)

// findSysfsPrinters lists printers bound to the usblp driver: each appears
// as /sys/class/usbmisc/lpN, whose device link is the USB interface and
// whose interface's parent is the USB device.
func findSysfsPrinters() ([]USBPrinter, error) {
	classDir := filepath.Join(sysfsRoot, "class", "usbmisc")
	entries, err := os.ReadDir(classDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var printers []USBPrinter
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "lp") {
			continue
		}
		p, err := readSysfsPrinter(filepath.Join(classDir, e.Name()))
		if err != nil {
			continue // unplugged mid-scan or not a USB device
		}
		printers = append(printers, p)
	}
	sort.Slice(printers, func(i, j int) bool { return printers[i].Path < printers[j].Path })
	return printers, nil
}

func readSysfsPrinter(node string) (USBPrinter, error) {
	var p USBPrinter

	// DEVNAME in uevent is the node path relative to /dev, e.g. usb/lp0.
	devName := "usb/" + filepath.Base(node)
	if uevent, err := os.ReadFile(filepath.Join(node, "uevent")); err == nil {
		for line := range strings.SplitSeq(string(uevent), "\n") {
			if v, ok := strings.CutPrefix(line, "DEVNAME="); ok {
				devName = v
			}
		}
	}
	p.Path = filepath.Join(devRoot, devName)

	intf, err := filepath.EvalSymlinks(filepath.Join(node, "device"))
	if err != nil {
		return p, err
	}
	dev := filepath.Dir(intf)
	vid, err := readHex(filepath.Join(dev, "idVendor"))
	if err != nil {
		return p, err
	}
	pid, err := readHex(filepath.Join(dev, "idProduct"))
	if err != nil {
		return p, err
	}
	p.VendorID, p.ProductID = vid, pid
	p.Manufacturer = readAttr(filepath.Join(dev, "manufacturer"))
	p.Product = readAttr(filepath.Join(dev, "product"))
	p.Serial = readAttr(filepath.Join(dev, "serial"))
	p.DeviceID = readAttr(filepath.Join(intf, "ieee1284_id"))
	return p, nil
}

func readAttr(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readHex(path string) (uint16, error) {
	v, err := strconv.ParseUint(readAttr(path), 16, 16)
	return uint16(v), err
}

// diagnoseSysfs explains why no usblp printer was found, distinguishing a
// connected Rongta printer without a usblp device node from no printer.
func diagnoseSysfs() error {
	devs, _ := filepath.Glob(filepath.Join(sysfsRoot, "bus", "usb", "devices", "*", "idVendor"))
	for _, vf := range devs {
		vid, err1 := readHex(vf)
		pid, err2 := readHex(filepath.Join(filepath.Dir(vf), "idProduct"))
		if err1 == nil && err2 == nil && vid == RongtaVendorID && pid == RongtaProductID {
			return fmt.Errorf("%w: a Rongta printer (%04x:%04x) is connected but has no usblp device node; "+
				"load the driver with \"modprobe usblp\" and make sure no libusb program holds the printer",
				ErrNoUSBPrinter, vid, pid)
		}
	}
	return ErrNoUSBPrinter
}
