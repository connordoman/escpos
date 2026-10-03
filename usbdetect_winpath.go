package escpos

import (
	"regexp"
	"strconv"
	"strings"
)

// Windows device interface paths look like
//
//	\\?\USB#VID_0FE6&PID_811E#0123456789#{28d78fad-5a12-11d1-ae5b-0000f803a8c2}
//	\\?\USB#VID_0FE6&PID_811E&MI_00#7&1b2c3d4e&0&0000#{28d78fad-...}
//
// The third segment is the USB serial number unless Windows generated an
// instance ID instead, which always contains '&'.
var winVIDPID = regexp.MustCompile(`(?i)VID_([0-9a-f]{4})&PID_([0-9a-f]{4})`)

// parseUSBPrintPath fills a USBPrinter from a Windows usbprint device
// interface path. ok is false if the path has no vendor and product ID.
func parseUSBPrintPath(path string) (p USBPrinter, ok bool) {
	m := winVIDPID.FindStringSubmatch(path)
	if m == nil {
		return p, false
	}
	vid, _ := strconv.ParseUint(m[1], 16, 16)
	pid, _ := strconv.ParseUint(m[2], 16, 16)
	p = USBPrinter{Path: path, VendorID: uint16(vid), ProductID: uint16(pid)}
	if parts := strings.Split(path, "#"); len(parts) >= 4 && !strings.Contains(parts[2], "&") {
		p.Serial = parts[2]
	}
	return p, true
}
