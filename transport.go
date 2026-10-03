package escpos

import (
	"context"
	"net"
	"os"
	"time"
)

// DefaultTCPPort is the raw printing port used by the Ethernet interface.
const DefaultTCPPort = "9100"

// DialTCP connects to a printer's Ethernet interface. If addr has no port,
// [DefaultTCPPort] is used. The returned connection supports status queries.
func DialTCP(ctx context.Context, addr string) (net.Conn, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, DefaultTCPPort)
	}
	var d net.Dialer
	if _, ok := ctx.Deadline(); !ok {
		d.Timeout = 5 * time.Second
	}
	return d.DialContext(ctx, "tcp", addr)
}

// OpenFile opens a printer device node for reading and writing, such as
// /dev/usb/lp0 created by the Linux usblp driver for a USB printer. This
// needs no cgo or libusb, making it the simplest way to reach a USB printer
// from a Raspberry Pi; [OpenUSB] finds the path for you. Whether status queries work depends on the driver;
// usblp supports reading.
//
// A serial port can also be opened this way if it has already been
// configured (for example with stty), but the serial package handles that
// for you.
func OpenFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR, 0)
}
