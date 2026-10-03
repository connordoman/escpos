// Package escpos drives Rongta RP325/RP326/RP327/RP328 thermal receipt
// printers (and other ESC/POS-compatible printers) using the command set
// described in "RP325&RP326&RP327&RP328 Command Set (RT V1.0)".
//
// The package is split into two layers:
//
//   - [Builder] accumulates ESC/POS commands into a byte buffer. It does no
//     I/O, so a print job can be assembled without holding the device (for
//     example while handling an HTTP request) and sent later.
//   - [Printer] owns a connection to the device. It embeds a Builder for
//     convenience, sends buffered or prepared jobs, and performs the
//     real-time and status commands that need a response from the printer.
//
// Connections are plain [io.Writer] values, optionally also implementing
// [io.Reader] (needed for status queries) and [io.Closer]. Ready-made
// transports are provided for every host interface the printer has:
//
//   - USB via libusb: package github.com/connordoman/escpos/usb (cgo).
//   - USB via the Linux usblp driver (/dev/usb/lp0): [OpenFile] (no cgo).
//   - Ethernet (raw TCP, port 9100): [DialTCP].
//   - RS-232 serial: package github.com/connordoman/escpos/serial.
//
// The RJ11 socket on these printers is the cash drawer kick-out connector,
// not a host interface. It is driven with [Builder.GeneratePulse],
// [Builder.OpenCashDrawer] and [Printer.RealTimePulse] over any of the
// transports above.
//
// This package has only been validated on a Rongta RP326 (firmware
// "7.03 ESC/POS"). Some commands in the reference behave differently on that
// printer; the README records which ones.
//
// Distances in this command set are given in motion units. With the default
// motion units (see [Builder.SetMotionUnits]) one unit is 0.125 mm, which is
// one dot on these 203 dpi printers.
package escpos
