package escpos

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSysfs builds a sysfs and /dev tree under a temporary directory,
// mirroring the layout on a Raspberry Pi, and points the package at it.
func fakeSysfs(t *testing.T) (addPrinter func(lp, vid, pid, product string), addBareDevice func(vid, pid string)) {
	t.Helper()
	root := t.TempDir()
	oldSys, oldDev, oldFind, oldDiag := sysfsRoot, devRoot, findUSBPrinters, diagnoseNoUSB
	sysfsRoot, devRoot = filepath.Join(root, "sys"), filepath.Join(root, "dev")
	findUSBPrinters, diagnoseNoUSB = findSysfsPrinters, diagnoseSysfs
	t.Cleanup(func() { sysfsRoot, devRoot, findUSBPrinters, diagnoseNoUSB = oldSys, oldDev, oldFind, oldDiag })

	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	port := 0
	addDevice := func(vid, pid, product string) string {
		port++
		dev := filepath.Join(sysfsRoot, "devices", "platform", "usb1", "1-"+string(rune('0'+port)))
		write(filepath.Join(dev, "idVendor"), vid+"\n")
		write(filepath.Join(dev, "idProduct"), pid+"\n")
		if product != "" {
			write(filepath.Join(dev, "product"), product+"\n")
		}
		bus := filepath.Join(sysfsRoot, "bus", "usb", "devices")
		os.MkdirAll(bus, 0o755)
		if err := os.Symlink(dev, filepath.Join(bus, filepath.Base(dev))); err != nil {
			t.Fatal(err)
		}
		return dev
	}
	addPrinter = func(lp, vid, pid, product string) {
		t.Helper()
		dev := addDevice(vid, pid, product)
		intf := filepath.Join(dev, filepath.Base(dev)+":1.0")
		write(filepath.Join(intf, "ieee1284_id"), "MFG:Test;CMD:ESC/POS;\n")
		node := filepath.Join(intf, "usbmisc", lp)
		write(filepath.Join(node, "uevent"), "MAJOR=180\nMINOR=0\nDEVNAME=usb/"+lp+"\n")
		if err := os.Symlink(intf, filepath.Join(node, "device")); err != nil {
			t.Fatal(err)
		}
		class := filepath.Join(sysfsRoot, "class", "usbmisc")
		os.MkdirAll(class, 0o755)
		if err := os.Symlink(node, filepath.Join(class, lp)); err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(devRoot, "usb", lp), "")
	}
	addBareDevice = func(vid, pid string) { addDevice(vid, pid, "") }
	return addPrinter, addBareDevice
}

func TestFindUSBPrinters(t *testing.T) {
	addPrinter, _ := fakeSysfs(t)
	addPrinter("lp1", "04b8", "0202", "TM-T88V")
	addPrinter("lp0", "0fe6", "811e", "RP326")

	printers, err := FindUSBPrinters()
	if err != nil {
		t.Fatal(err)
	}
	if len(printers) != 2 {
		t.Fatalf("found %d printers, want 2: %v", len(printers), printers)
	}
	p := printers[0]
	if p.Path != filepath.Join(devRoot, "usb", "lp0") || p.VendorID != 0x0fe6 || p.ProductID != 0x811e ||
		p.Product != "RP326" || p.DeviceID != "MFG:Test;CMD:ESC/POS;" {
		t.Errorf("got %+v", p)
	}

	f, got, err := OpenUSB(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if got.Product != "RP326" {
		t.Errorf("OpenUSB(nil) chose %v, want the Rongta printer", got)
	}

	f, got, err = OpenUSB(func(p USBPrinter) bool { return p.VendorID == 0x04b8 })
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if got.Product != "TM-T88V" {
		t.Errorf("OpenUSB(match) chose %v", got)
	}
}

func TestOpenUSBSingleOtherPrinter(t *testing.T) {
	addPrinter, _ := fakeSysfs(t)
	addPrinter("lp0", "1234", "5678", "")
	f, got, err := OpenUSB(nil)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if got.VendorID != 0x1234 {
		t.Errorf("got %v", got)
	}
}

func TestOpenUSBAmbiguous(t *testing.T) {
	addPrinter, _ := fakeSysfs(t)
	addPrinter("lp0", "1234", "5678", "")
	addPrinter("lp1", "1234", "5679", "")
	_, _, err := OpenUSB(nil)
	if !errors.Is(err, ErrNoUSBPrinter) || !strings.Contains(err.Error(), "2 printers") {
		t.Fatalf("got %v", err)
	}
}

func TestOpenUSBNoDriver(t *testing.T) {
	_, addBareDevice := fakeSysfs(t)
	addBareDevice("0fe6", "811e")
	_, _, err := OpenUSB(nil)
	if !errors.Is(err, ErrNoUSBPrinter) || !strings.Contains(err.Error(), "modprobe usblp") {
		t.Fatalf("got %v", err)
	}
}

func TestOpenUSBNothing(t *testing.T) {
	fakeSysfs(t)
	if _, _, err := OpenUSB(nil); !errors.Is(err, ErrNoUSBPrinter) {
		t.Fatalf("got %v", err)
	}
}

func TestParseUSBPrintPath(t *testing.T) {
	tests := []struct {
		path   string
		ok     bool
		vid    uint16
		pid    uint16
		serial string
	}{
		{`\\?\USB#VID_0FE6&PID_811E#0123456789#{28d78fad-5a12-11d1-ae5b-0000f803a8c2}`, true, 0x0fe6, 0x811e, "0123456789"},
		{`\\?\usb#vid_04b8&pid_0202&mi_00#7&1b2c3d4e&0&0000#{28d78fad-5a12-11d1-ae5b-0000f803a8c2}`, true, 0x04b8, 0x0202, ""},
		{`\\?\ROOT#PRINTER#0000#{28d78fad-5a12-11d1-ae5b-0000f803a8c2}`, false, 0, 0, ""},
	}
	for _, tt := range tests {
		p, ok := parseUSBPrintPath(tt.path)
		if ok != tt.ok || p.VendorID != tt.vid || p.ProductID != tt.pid || p.Serial != tt.serial {
			t.Errorf("parseUSBPrintPath(%q) = %+v, %v", tt.path, p, ok)
		}
		if ok && p.Path != tt.path {
			t.Errorf("Path = %q, want %q", p.Path, tt.path)
		}
	}
}

func TestSelectUSBPrinter(t *testing.T) {
	rongta := USBPrinter{Path: "b", VendorID: RongtaVendorID, ProductID: RongtaProductID}
	other := USBPrinter{Path: "a", VendorID: 1, ProductID: 2, Serial: "S1"}
	if p, err := SelectUSBPrinter([]USBPrinter{other, rongta}, nil); err != nil || p != rongta {
		t.Errorf("preferred Rongta: got %v, %v", p, err)
	}
	if p, err := SelectUSBPrinter([]USBPrinter{other}, nil); err != nil || p != other {
		t.Errorf("sole printer: got %v, %v", p, err)
	}
	if _, err := SelectUSBPrinter([]USBPrinter{other, other}, nil); !errors.Is(err, ErrNoUSBPrinter) {
		t.Errorf("ambiguous: got %v", err)
	}
	if _, err := SelectUSBPrinter(nil, nil); !errors.Is(err, ErrNoUSBPrinter) {
		t.Errorf("none: got %v", err)
	}
	bySerial := func(p USBPrinter) bool { return p.Serial == "S1" }
	if p, err := SelectUSBPrinter([]USBPrinter{rongta, other}, bySerial); err != nil || p != other {
		t.Errorf("match: got %v, %v", p, err)
	}
	if _, err := SelectUSBPrinter([]USBPrinter{rongta}, bySerial); !errors.Is(err, ErrNoUSBPrinter) {
		t.Errorf("no match: got %v", err)
	}
}
