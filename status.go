package escpos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// StatusType selects the status returned by [Printer.RealTimeStatus].
type StatusType byte

const (
	StatusPrinter StatusType = 1 // drawer signal; see [PrinterStatus]
	StatusOffline StatusType = 2 // cover, feed button, error; see [OfflineStatus]
	StatusError   StatusType = 3 // cutter and other errors; see [ErrorStatus]
	StatusPaper   StatusType = 4 // paper end sensor; see [PaperStatus]
)

// InvalidStatusError is returned when a status byte does not have the fixed
// bits every real-time status byte carries, which usually means another
// response (such as an ASB report) was read instead.
type InvalidStatusError struct {
	Byte byte
}

func (e *InvalidStatusError) Error() string {
	return fmt.Sprintf("escpos: invalid status byte %#02x", e.Byte)
}

// validStatus checks the fixed bits of a DLE EOT response: bits 1 and 4 on,
// bits 0 and 7 off.
func validStatus(b byte) bool { return b&0x93 == 0x12 }

// PrinterStatus is the response to DLE EOT 1.
type PrinterStatus byte

// DrawerSignalHigh reports whether the drawer open/close signal (connector
// pin 3) is high. Which level means "open" depends on the drawer.
func (s PrinterStatus) DrawerSignalHigh() bool { return s&0x04 != 0 }

// OfflineStatus is the response to DLE EOT 2.
type OfflineStatus byte

// CoverOpen reports whether the cover is open.
func (s OfflineStatus) CoverOpen() bool { return s&0x04 != 0 }

// FeedButtonPressed reports whether paper is being fed by the FEED button.
func (s OfflineStatus) FeedButtonPressed() bool { return s&0x08 != 0 }

// Error reports whether an error has occurred.
func (s OfflineStatus) Error() bool { return s&0x40 != 0 }

// ErrorStatus is the response to DLE EOT 3.
type ErrorStatus byte

// AutoCutterError reports an auto cutter error.
func (s ErrorStatus) AutoCutterError() bool { return s&0x08 != 0 }

// UnrecoverableError reports an unrecoverable error.
func (s ErrorStatus) UnrecoverableError() bool { return s&0x20 != 0 }

// AutoRecoverableError reports an auto-recoverable error, such as the print
// head overheating or the cover being opened while printing.
func (s ErrorStatus) AutoRecoverableError() bool { return s&0x40 != 0 }

// PaperStatus is the response to DLE EOT 4.
type PaperStatus byte

// PaperEnd reports whether the paper end sensor detects no paper.
func (s PaperStatus) PaperEnd() bool { return s&0x60 != 0 }

// Status combines the four real-time status bytes.
type Status struct {
	Printer PrinterStatus
	Offline OfflineStatus
	Error   ErrorStatus
	Paper   PaperStatus
}

// Ready reports whether the printer can print: the cover is closed, there is
// paper and no error has occurred.
func (s Status) Ready() bool {
	return !s.Offline.CoverOpen() && !s.Offline.Error() && !s.Paper.PaperEnd() &&
		!s.Error.AutoCutterError() && !s.Error.UnrecoverableError() && !s.Error.AutoRecoverableError()
}

// Problems lists what stops the printer from printing, such as
// "cover open" or "out of paper". It is empty when the printer is ready.
func (s Status) Problems() []string {
	var p []string
	if s.Offline.CoverOpen() {
		p = append(p, "cover open")
	}
	if s.Paper.PaperEnd() {
		p = append(p, "out of paper")
	}
	if s.Error.AutoCutterError() {
		p = append(p, "cutter error")
	}
	if s.Error.UnrecoverableError() {
		p = append(p, "unrecoverable error")
	}
	if s.Error.AutoRecoverableError() {
		p = append(p, "auto-recoverable error (overheated?)")
	}
	if len(p) == 0 && s.Offline.Error() {
		p = append(p, "error")
	}
	return p
}

// String summarises the status: "ready", or its problems separated by
// commas.
func (s Status) String() string {
	if p := s.Problems(); len(p) > 0 {
		return strings.Join(p, ", ")
	}
	return "ready"
}

// RealTimeStatus requests one real-time status byte (DLE EOT n). The printer
// answers immediately, even when offline or in an error state.
func (p *Printer) RealTimeStatus(ctx context.Context, t StatusType) (byte, error) {
	if t < 1 || t > 4 {
		return 0, invalid("DLE EOT", "status type %d out of range 1–4", t)
	}
	resp, err := p.query(ctx, []byte{DLE, EOT, byte(t)}, 1)
	if err != nil {
		return 0, err
	}
	if !validStatus(resp[0]) {
		return resp[0], &InvalidStatusError{resp[0]}
	}
	return resp[0], nil
}

// Status requests all four real-time status bytes.
func (p *Printer) Status(ctx context.Context) (Status, error) {
	var s Status
	var raw [4]byte
	for i := range raw {
		b, err := p.RealTimeStatus(ctx, StatusType(i+1))
		if err != nil {
			return s, err
		}
		raw[i] = b
	}
	s.Printer = PrinterStatus(raw[0])
	s.Offline = OfflineStatus(raw[1])
	s.Error = ErrorStatus(raw[2])
	s.Paper = PaperStatus(raw[3])
	return s, nil
}

// RecoverFromError recovers from a recoverable error such as an auto cutter
// error (DLE ENQ n). If clearBuffers is false, printing restarts from the
// line where the error occurred; otherwise the receive and print buffers are
// cleared first. The command is sent immediately.
func (p *Printer) RecoverFromError(ctx context.Context, clearBuffers bool) error {
	n := byte(1)
	if clearBuffers {
		n = 2
	}
	_, err := p.SendRaw(ctx, []byte{DLE, ENQ, n})
	return err
}

// RealTimePulse outputs a pulse on the drawer kick-out connector immediately
// (DLE DC4 1 m t), on for t×100 ms and off for t×100 ms with 1 ≤ t ≤ 8. It
// works even while the printer is disabled with ESC =.
func (p *Printer) RealTimePulse(ctx context.Context, pin DrawerPin, t uint8) error {
	if pin > 1 {
		return invalid("DLE DC4", "pin %d is not 0 or 1", pin)
	}
	if t < 1 || t > 8 {
		return invalid("DLE DC4", "time %d out of range 1–8", t)
	}
	_, err := p.SendRaw(ctx, []byte{DLE, DC4, 1, byte(pin), t})
	return err
}

// PaperSensorStatus is the response to GS r 1.
type PaperSensorStatus byte

// NearEnd reports whether the paper roll near-end sensor detects that the
// roll is nearly used up.
func (s PaperSensorStatus) NearEnd() bool { return s&0x0C != 0 }

// TransmitStatus asks the printer to send its paper sensor status (GS r 1)
// once it reaches this point in the data. Use [Printer.PaperSensorStatus] to
// send it and read the response.
func (b *Builder) TransmitStatus() { b.cmd(GS, 'r', 1) }

// PaperSensorStatus requests the paper sensor status (GS r 1). Unlike the
// real-time status, it is answered only after earlier data has been
// processed, and not at all while the printer is out of paper.
func (p *Printer) PaperSensorStatus(ctx context.Context) (PaperSensorStatus, error) {
	resp, err := p.query(ctx, []byte{GS, 'r', 1}, 1)
	if err != nil {
		return 0, err
	}
	return PaperSensorStatus(resp[0]), nil
}

// ASB is a set of status items for [Builder.SetAutoStatusBack].
type ASB byte

const (
	ASBErrorStatus ASB = 1 << 2 // report error status changes
	ASBPaperSensor ASB = 1 << 3 // report paper roll sensor changes
)

// SetAutoStatusBack enables or disables Automatic Status Back (GS a n). With
// items set, the printer sends four status bytes immediately and again
// whenever an enabled item changes; read them with [Printer.ReadASB]. Zero
// disables ASB.
func (b *Builder) SetAutoStatusBack(items ASB) { b.cmd(GS, 'a', byte(items)) }

// ReadASB reads one four-byte Automatic Status Back report. The reference
// does not document the layout of the report for this printer.
func (p *Printer) ReadASB(ctx context.Context) ([4]byte, error) {
	ctx, cancel := p.withTimeout(ctx)
	defer cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	var r [4]byte
	err := p.readFull(ctx, r[:])
	return r, err
}

// PrinterIDType selects the information returned by GS I.
type PrinterIDType byte

const (
	PrinterIDModel  PrinterIDType = 1 // printer model ID, one byte
	PrinterIDTypeID PrinterIDType = 2 // type ID, one byte; see [TypeID]

	PrinterInfoFirmware     PrinterIDType = 65 // firmware version
	PrinterInfoManufacturer PrinterIDType = 66 // manufacturer
	PrinterInfoName         PrinterIDType = 67 // printer name
	PrinterInfoSerial       PrinterIDType = 68 // serial number
	PrinterInfoFonts        PrinterIDType = 69 // type of mounted additional fonts
)

func (t PrinterIDType) isInfo() bool { return t >= 65 && t <= 69 }

func (t PrinterIDType) valid() bool {
	return t == 1 || t == 2 || t == 49 || t == 50 || t.isInfo()
}

// TransmitPrinterID asks the printer to send an ID or information string
// (GS I n). Use [Printer.PrinterID] or [Printer.PrinterInfo] to send it and
// read the response.
func (b *Builder) TransmitPrinterID(t PrinterIDType) error {
	if !t.valid() {
		return invalid("GS I", "type %d is not 1, 2, 49, 50 or 65–69", t)
	}
	b.cmd(GS, 'I', byte(t))
	return nil
}

// TypeID is the response to GS I 2.
type TypeID byte

// MultiByte reports whether multi-byte (Kanji) characters are supported.
func (t TypeID) MultiByte() bool { return t&0x01 != 0 }

// AutoCutter reports whether an auto cutter is installed.
func (t TypeID) AutoCutter() bool { return t&0x02 != 0 }

// PrinterID requests a one-byte printer ID (GS I n) for [PrinterIDModel] or
// [PrinterIDTypeID].
func (p *Printer) PrinterID(ctx context.Context, t PrinterIDType) (byte, error) {
	if !t.valid() || t.isInfo() {
		return 0, invalid("GS I", "type %d is not 1, 2, 49 or 50", t)
	}
	resp, err := p.query(ctx, []byte{GS, 'I', byte(t)}, 1)
	if err != nil {
		return 0, err
	}
	return resp[0], nil
}

// PrinterInfo requests an information string (GS I n) such as
// [PrinterInfoFirmware]. The response is read up to its terminating NUL, and
// a leading '_' header byte, if present, is removed.
func (p *Printer) PrinterInfo(ctx context.Context, t PrinterIDType) (string, error) {
	if !t.isInfo() {
		return "", invalid("GS I", "type %d is not 65–69", t)
	}
	ctx, cancel := p.withTimeout(ctx)
	defer cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.write(ctx, []byte{GS, 'I', byte(t)}); err != nil {
		return "", err
	}
	resp, err := p.readUntilNUL(ctx)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimPrefix(resp, []byte{'_'})), nil
}

// SetProcessIDResponse asks the printer to report id once it has processed
// all data before this command (GS ( H 6 0 48 48 d1 d2 d3 d4). Each byte of
// id must be within 0x20–0x7E. Use [Printer.WaitProcessID] to wait for it,
// for example to learn that a job has finished.
func (b *Builder) SetProcessIDResponse(id [4]byte) error {
	for _, c := range id {
		if c < 0x20 || c > 0x7E {
			return invalid("GS ( H", "id byte %#02x outside 0x20–0x7E", c)
		}
	}
	b.cmd(GS, '(', 'H', 6, 0, 48, 48, id[0], id[1], id[2], id[3])
	return nil
}

// ErrUnconfirmed is returned by [Printer.SendConfirmed] when the data was
// sent but the printer did not confirm processing it in time.
var ErrUnconfirmed = errors.New("escpos: data sent but the printer did not confirm it")

// SendConfirmed sends data followed by a process ID request
// ([Builder.SetProcessIDResponse]) and waits for the printer to report the
// ID, which it does once it has processed everything before it. A nil error
// therefore means the job has been printed, not just sent.
//
// ctx bounds the whole call; printing a long job can take several seconds,
// so allow for it. If the connection cannot be read from, nothing is sent
// and [ErrNotReadable] is returned, so callers can fall back to
// [Printer.SendRaw]. If the data was sent but no confirmation arrived, the
// error wraps [ErrUnconfirmed].
func (p *Printer) SendConfirmed(ctx context.Context, data []byte) error {
	if _, ok := p.conn.(io.Reader); !ok {
		return ErrNotReadable
	}
	n := p.processSeq.Add(1)
	id := [4]byte{}
	copy(id[:], fmt.Sprintf("%04d", n%10000))
	marker := NewBuilder(p.paperWidth)
	marker.SetProcessIDResponse(id)

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, err := p.write(ctx, append(append([]byte{}, data...), marker.buf...)); err != nil {
		return err
	}
	for {
		resp, err := p.readUntilNUL(ctx)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrUnconfirmed, err)
		}
		if bytes.HasSuffix(resp, id[:]) {
			return nil
		}
	}
}

// WaitProcessID reads responses until the process ID set with
// [Builder.SetProcessIDResponse] arrives or ctx ends.
//
// The reference does not document the response format. This accepts any NUL
// terminated response ending in id, which covers the usual ESC/POS format
// (0x37 0x22 d1 d2 d3 d4 NUL).
func (p *Printer) WaitProcessID(ctx context.Context, id [4]byte) error {
	ctx, cancel := p.withTimeout(ctx)
	defer cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		resp, err := p.readUntilNUL(ctx)
		if err != nil {
			return err
		}
		if bytes.HasSuffix(resp, id[:]) {
			return nil
		}
	}
}
