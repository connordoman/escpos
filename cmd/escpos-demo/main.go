// Command escpos-demo connects to an ESC/POS printer and demonstrates the
// features of the escpos package.
//
// By default it detects a USB printer automatically: through the operating
// system's printer driver on Linux and Windows, and through libusb on macOS.
// Flags override the connection with a specific USB device, a serial port or
// a network address.
//
// libusb support is compiled in on macOS, or elsewhere with -tags libusb;
// other builds need no cgo.
//
//	escpos-demo demo                    # print every section
//	escpos-demo demo text barcodes      # print selected sections
//	escpos-demo demo --list             # list sections
//	escpos-demo status                  # query printer status
//	escpos-demo list                    # list detected USB printers
//	escpos-demo --tcp 192.168.1.87 demo # use the Ethernet interface
//	escpos-demo --dry-run demo          # hex dump instead of printing
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// options holds the global flags.
type options struct {
	device    string
	vid, pid  string
	usbSerial string
	libusb    bool

	serialPort string
	baud       int
	tcp        string

	paperWidth int
	timeout    time.Duration
	dryRun     bool
	output     string
	verbose    bool
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:   "escpos-demo",
		Short: "Demonstrate the escpos package on a real printer",
		Long: `escpos-demo connects to an ESC/POS printer (developed against the Rongta
RP326) and prints samples of the package's features using placeholder data.

Connection, in order of precedence:
  --tcp ADDR          Ethernet (raw TCP, port 9100 by default)
  --serial PORT       RS-232 serial port, e.g. /dev/ttyUSB0 or COM3
  --device PATH       USB printer device path, e.g. /dev/usb/lp0 on Linux or a
                      \\?\USB#... interface path on Windows (see "list")
  --vid/--pid/--usb-serial
                      choose among detected USB printers
  (none)              detect a USB printer automatically`,
		SilenceUsage: true,
	}

	f := root.PersistentFlags()
	f.StringVarP(&opts.device, "device", "d", "", "USB printer device path (overrides detection)")
	f.StringVar(&opts.vid, "vid", "", "USB vendor ID in hex, e.g. 0fe6")
	f.StringVar(&opts.pid, "pid", "", "USB product ID in hex, e.g. 811e")
	f.StringVar(&opts.usbSerial, "usb-serial", "", "USB serial number, to choose between identical printers")
	f.BoolVar(&opts.libusb, "libusb", false, "use libusb even where the OS printer driver is available")
	f.StringVar(&opts.serialPort, "serial", "", "serial port, e.g. /dev/ttyUSB0 or COM3")
	f.IntVar(&opts.baud, "baud", 9600, "serial baud rate")
	f.StringVar(&opts.tcp, "tcp", "", "printer network address, host[:port]")
	f.IntVarP(&opts.paperWidth, "paper-width", "w", 576, "printable width in dots (576 = 80 mm, 432 = 58 mm)")
	f.DurationVar(&opts.timeout, "timeout", 30*time.Second, "time limit for sending a job")
	f.BoolVarP(&opts.dryRun, "dry-run", "n", false, "hex dump the commands to stdout instead of printing")
	f.StringVarP(&opts.output, "output", "o", "", "write the raw commands to a file instead of printing")
	f.BoolVarP(&opts.verbose, "verbose", "v", false, "log connection details")

	root.AddCommand(
		newDemoCommand(opts),
		newStatusCommand(opts),
		newListCommand(opts),
		newDrawerCommand(opts),
		newVerifyCommand(opts),
	)
	return root
}

func (o *options) logf(format string, args ...any) {
	if o.verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
