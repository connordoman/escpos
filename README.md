# escpos

A Go package for Rongta RP325/RP326/RP327/RP328 thermal receipt printers (and
other ESC/POS printers). It implements every command in
[`reference/RP325&RP326&RP327&RP328-Command Set (RT V1.0).pdf`](reference/)
and talks to the printer over USB, Ethernet or serial.

```go
conn, _ := usb.Open(usb.Options{}) // Rongta RP326 IDs by default
p := escpos.New(conn)
defer p.Close()

p.Initialize()
p.SetAlign(escpos.AlignCenter)
p.Textln("Hello, world")
p.PrintQRCode("https://example.com", 0, escpos.QRErrorM, 6)
p.FeedAndCut(0)
_, err := p.Flush(ctx)
```

## Design

- **`Builder`** accumulates commands in memory and does no I/O. Methods with
  restricted parameters validate them and return an error wrapping
  `ErrInvalidArgument` without writing anything.
- **`Printer`** wraps a connection. It embeds a `Builder` (append, then
  `Flush`) and can also `Send` separately built jobs. A web server can build
  each request's job on its own and let `Send` serialize access to the device.
  `Printer` also handles the commands that need a reply: real-time status,
  printer ID and process ID.
- **Connections** are any `io.Writer`. If the connection is also an
  `io.Reader`, status queries work. If it implements `WriteContext`/`ReadContext`
  or `SetWriteDeadline`/`SetReadDeadline`, the `context.Context` passed to each
  call bounds its I/O.
- **Text** is encoded for the selected code page (CP437 by default), so
  `"café ─"` prints correctly. Characters the code page lacks are replaced
  with ASCII where possible (`“”` → `""`, `—` → `-`) and with `?` otherwise.

## Connections

| Interface | How | Notes |
|---|---|---|
| USB (libusb) | `usb.Open(usb.Options{})` | Needs cgo and libusb (`apt install libusb-1.0-0-dev`). Detaches the kernel driver automatically. Finds the bulk endpoints on its own. |
| USB (Linux usblp) | `escpos.OpenFile("/dev/usb/lp0")` | No cgo, so it cross-compiles with `GOOS=linux GOARCH=arm64`. Probably the simplest choice on a Raspberry Pi. |
| Ethernet | `escpos.DialTCP(ctx, "192.168.1.87")` | Raw TCP on port 9100. |
| Serial (RS-232) | `serial.Open("/dev/ttyUSB0", serial.Options{BaudRate: 115200})` | Pure Go. Defaults to 9600 8N1. |
| RJ11 | not a host interface | This is the cash drawer kick-out port. Drive it with `OpenCashDrawer`, `GeneratePulse` (ESC p) or `RealTimePulse` (DLE DC4) over any connection above. |

On Linux, using the USB device as a non-root user needs a udev rule, for
example `/etc/udev/rules.d/99-escpos.rules`:

```
SUBSYSTEM=="usb", ATTRS{idVendor}=="0fe6", ATTRS{idProduct}=="811e", MODE="0666"
SUBSYSTEM=="usbmisc", KERNEL=="lp*", MODE="0666"
```

## Command coverage

`B` = `Builder` method, `P` = `Printer` method (sent immediately and/or reads a reply).

| Command | Method |
|---|---|
| HT / LF / CR | B `Tab`, `LineFeed`, `CarriageReturn` (also `Textln`) |
| DLE EOT n | P `RealTimeStatus`, `Status` |
| DLE ENQ n | P `RecoverFromError` |
| DLE DC4 n m t | P `RealTimePulse` |
| ESC SP n | B `SetCharacterSpacing` |
| ESC ! n | B `SelectPrintMode` |
| ESC $ | B `SetAbsolutePosition` |
| ESC % n | B `SelectUserDefinedCharset` |
| ESC & | B `DefineUserCharacters` |
| ESC * | B `PrintBitImage` |
| ESC - n | B `SetUnderline` |
| ESC 2 / ESC 3 n | B `DefaultLineSpacing`, `SetLineSpacing` |
| ESC ? n | B `CancelUserCharacter` |
| ESC @ | B `Initialize` |
| ESC B n t | B `Beep` |
| ESC D | B `SetTabPositions` |
| ESC E / ESC G | B `SetEmphasis`, `SetDoubleStrike` |
| ESC J / ESC d | B `FeedUnits`, `FeedLines` |
| ESC M | B `SelectFont` |
| ESC R / ESC t | B `SelectInternationalCharset`, `SelectCodePage` |
| ESC V / ESC { | B `SetRotate90`, `SetUpsideDown` |
| ESC \ | B `SetRelativePosition` |
| ESC a | B `SetAlign` |
| ESC c 5 | B `SetPanelButtons` |
| ESC p | B `GeneratePulse`, `OpenCashDrawer` |
| ESC i / ESC m | B `CutImmediate`, `PartialCutImmediate` |
| ESC 9 | B `SelectChineseEncoding` |
| ESC = | B `SetPeripheralDevice` |
| FS p / FS q | B `PrintNVBitImage`, `DefineNVBitImages` |
| FS ! / FS & / FS . / FS - | B `SetKanjiPrintMode`, `SelectKanjiMode`, `CancelKanjiMode`, `SetKanjiUnderline` |
| FS 2 / FS S / FS W | B `DefineKanjiCharacter`, `SetKanjiSpacing`, `SetKanjiQuadruple` |
| GS ! | B `SetCharacterSize` |
| GS * / GS / | B `DefineDownloadedBitImage`, `PrintDownloadedBitImage` |
| GS B | B `SetReverse` |
| GS I | B `TransmitPrinterID`; P `PrinterID`, `PrinterInfo` |
| GS ( H | B `SetProcessIDResponse`; P `WaitProcessID` |
| GS H / GS f / GS h / GS w / GS x | B `SetHRIPosition`, `SetHRIFont`, `SetBarcodeHeight`, `SetBarcodeWidth`, `SetBarcodeLeftSpace` |
| GS k (both forms) | B `PrintBarcode` (+ `Code128` helper) |
| GS L / GS W | B `SetLeftMargin`, `SetPrintAreaWidth` |
| GS V m / GS V m n | B `Cut`, `FeedAndCut` |
| GS : / GS ^ | B `ToggleMacroDefinition`, `DefineMacro`, `ExecuteMacro` |
| GS a | B `SetAutoStatusBack`; P `ReadASB` |
| GS r | B `TransmitStatus`; P `PaperSensorStatus` |
| GS v 0 | B `PrintRasterBitImage`, `PrintImage` (any `image.Image`, optional dithering) |
| GS P | B `SetMotionUnits` |
| DC2 T / GS ( A | B `PrintTestPage`, `ExecuteTestPrint` |
| FF / ESC FF / ESC L / ESC S | B `PrintPageAndExit`, `PrintPage`, `EnterPageMode`, `EnterStandardMode` |
| ESC T / ESC W / GS $ / GS \ | B `SetPageDirection`, `SetPageArea`, `SetAbsoluteVerticalPosition`, `SetRelativeVerticalPosition` |
| GS Z / ESC Z | B `Select2DBarcodeType`, `Print2DBarcode`, `PrintQRCode`, `PrintPDF417` |
| GS FF | B `FeedToMark` |
| GS C 0 / 1 / 2 / ; and GS c | B `SetCounterPrintMode`, `SetCounterModeA`, `SetCounter`, `SetCounterModeB`, `PrintCounter` |

Not in the reference, but worked with the original `connordoman/pos` code:
`Buzz` (ESC ( A beeper), `Cut(CutFull)` (GS V 0), `FontC`. `FeedAndFullCut`
(GS V 65) is the standard ESC/POS counterpart of GS V 66.

`Builder.Raw` and `Printer.SendRaw` cover anything else.

## Things the reference leaves unclear

- **QR error correction values (ESC Z).** The reference names the L/M/Q/H
  levels but not their byte values. `QRErrorL`… use the ASCII letters, as
  other ESC Z printers do. Check this on hardware.
- **GS I info strings and the GS ( H reply** have no documented reply format.
  `PrinterInfo` reads up to a NUL and strips a leading `_`. `WaitProcessID`
  waits for any NUL-terminated reply that ends in the ID. Both follow the usual
  ESC/POS conventions.
- **ASB layout.** `ReadASB` returns the four bytes as they arrive, without
  decoding them.
- **Real-time sequences inside data.** The printer acts on `DLE EOT`, `DLE ENQ`
  and `DLE DC4` byte sequences wherever they appear, including inside image
  data.
