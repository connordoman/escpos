# escpos

A Go package for Rongta RP325/RP326/RP327/RP328 thermal receipt printers (and
other ESC/POS printers). It implements every command in Rongta's
*RP325\RP326\RP327\RP328 Command Set* (RT V1.0) and talks to the printer
over USB, Ethernet or serial.

The command set comes on the flash drive bundled with the printer. Rongta
doesn't publish it on its own sites ([rongtatech.com](https://www.rongtatech.com/user-manual/)
only offers user manuals), but a copy is hosted on
[ManualsLib](https://www.manualslib.com/manual/3423402/Rongta-Technology-Rp325.html).

> **Validated hardware:** this package has only been tested on a **Rongta
> RP326** (80 mm, USB; firmware "7.03 ESC/POS", which identifies itself as an
> EPSON TM-T88III). Other RP32x models and other ESC/POS printers may work,
> but nobody has tested them. See [RP326 compatibility](#rp326-compatibility)
> for which commands work on that printer.

```go
conn, _ := usb.Open(usb.Options{}) // Rongta RP326 IDs by default
p := escpos.New(conn)
defer p.Close()

p.Initialize()
p.SetAlign(escpos.AlignCenter)
p.Textln("Hello, world")
p.PrintQRCode("https://example.com", escpos.QRErrorM, 6)
p.FeedAndCut(0)
_, err := p.Flush(ctx)
```

## Demo

`cmd/escpos-demo` is a CLI that connects to a printer (auto-detected, or set
with `--device`, `--vid/--pid`, `--serial` or `--tcp`) and prints samples of
every feature using placeholder data:

```sh
go run ./cmd/escpos-demo demo --list        # list sections
go run ./cmd/escpos-demo demo               # print all standard sections
go run ./cmd/escpos-demo demo barcodes 2d   # print selected sections
go run ./cmd/escpos-demo status             # status and printer info
go run ./cmd/escpos-demo verify --cuts       # numbered checklist of every command
go run ./cmd/escpos-demo list               # detected USB printers
go run ./cmd/escpos-demo -n demo receipt    # hex dump instead of printing
```

It uses libusb on macOS, or elsewhere with `-tags libusb`. Linux and Windows
builds need no cgo.

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

Every way of connecting is always available. Auto-detection is an optional
extra on top of explicit paths and IDs.

| Interface | Explicit | Auto-detected | cgo? | Platforms |
|---|---|---|---|---|
| USB (OS driver) | `escpos.OpenFile("/dev/usb/lp0")` | `escpos.OpenUSB(nil)` | no | Linux (usblp), Windows (usbprint) |
| USB (libusb) | `usb.Open(usb.Options{VendorID: …, ProductID: …})` | `usb.Open(usb.Options{})` | yes | Linux, macOS, Windows* |
| Ethernet | `escpos.DialTCP(ctx, "192.168.1.87")` | n/a | no | all |
| Serial (RS-232) | `serial.Open("/dev/ttyUSB0" or "COM3", serial.Options{…})` | n/a | no | all |
| RJ11 | not a host interface | | | |

The RJ11 socket is the cash drawer kick-out port. Drive it with
`OpenCashDrawer`, `GeneratePulse` (ESC p) or `RealTimePulse` (DLE DC4) over
any connection above.

Which USB option to use on each platform:

- **Linux / Raspberry Pi:** `escpos.OpenUSB` reads sysfs and opens the usblp
  node. It needs no cgo, so it cross-compiles with `GOOS=linux GOARCH=arm64`.
- **Windows:** `escpos.OpenUSB` lists the built-in usbprint driver's device
  interfaces and opens one directly. It bypasses the print spooler, so no
  printer queue or vendor driver is needed. Manufacturer and product names
  are not filled in on Windows.
- **macOS:** there is no cgo-free route. Use `usb.Open(usb.Options{})`
  (`brew install libusb`); `escpos.OpenUSB` returns `ErrUSBDetectUnsupported`.

\*libusb on Windows only works after replacing the printer's driver with
WinUSB (for example using Zadig). That stops Windows printing to it, which is
why `escpos.OpenUSB` is the better choice there.

Both detection routes choose a printer with `escpos.SelectUSBPrinter`: a
Rongta RP326 if one is connected, otherwise the only printer present. If
several printers are connected, pass a match function (for example by
`Serial`). `escpos.FindUSBPrinters()` and `usb.Find()` list the candidates.

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
| GS Z / ESC Z | B `Select2DBarcodeType`, `Print2DBarcode`, `PrintQRCodeESCZ`, `PrintPDF417` |
| GS FF | B `FeedToMark` |
| GS C 0 / 1 / 2 / ; and GS c | B `SetCounterPrintMode`, `SetCounterModeA`, `SetCounter`, `SetCounterModeB`, `PrintCounter` |

QR codes are printed with the standard `GS ( k` commands (`PrintQRCode`,
`SelectQRCodeModel`, `SetQRCodeModuleSize`, `SetQRCodeErrorCorrection`,
`StoreQRCodeData`, `PrintStoredQRCode`), which are not in the reference.

Also not in the reference: `Cut(CutFull)` (GS V 0), `FeedAndFullCut`
(GS V 65) and `FontC`, which are standard ESC/POS. The RP326 accepts all
three; see below for how they behave.

`Builder.Raw` and `Printer.SendRaw` cover anything else.

## RP326 compatibility

This package has only been validated on one printer. Results below are from
a **Rongta RP326** (80 mm; firmware "7.03 ESC/POS", which reports itself as
"EPSON TM-T88III"; model ID `0x20`, type ID `0x02`), tested in October 2026
with `escpos-demo status` and `escpos-demo verify --cuts`. Run those
commands to check another printer.

**Connections tested on hardware:** USB through libusb (`usb.Open`) on macOS
only. `escpos.OpenUSB` (Linux usblp, Windows usbprint), `DialTCP` and
`serial.Open` are tested in software but not yet against a printer.

**Work as the reference describes:**

- Text and layout: LF, CR, HT, ESC D, ESC !, ESC M (fonts A, B and C),
  ESC E, ESC G, ESC -, GS !, GS B, ESC V, ESC {, ESC SP, ESC 2, ESC 3,
  ESC a, GS L, GS W, ESC $, ESC \, ESC J, ESC d, GS P, ESC t, ESC R, ESC =
- User-defined characters: ESC &, ESC %, ESC ?
- Images: ESC *, GS *, GS /, GS v 0
- Bar codes: GS k (both forms), GS H, GS f, GS h, GS w, GS x
- PDF417: GS Z 0 + ESC Z
- Page mode: ESC L, ESC S, ESC W, ESC T, GS $, GS \, ESC FF, FF
- Counter: GS C 0, GS C 1, GS C 2, GS c
- Macros: GS :, GS ^
- Buzzer: ESC B (ESC B 1 2 gives one beep of about 0.3 s)
- Cutting: GS V 1, GS V 66 n
- Queries: DLE EOT 1–4, GS r, GS I (1, 2, 65–69), GS a (ASB report
  `14 00 00 0f` on enabling), GS ( H (`WaitProcessID`)

**Work differently from the reference:**

| Command | On the RP326 |
|---|---|
| GS Z 1 + ESC Z (`PrintQRCodeESCZ`) | Prints PDF417, not QR. GS Z is ignored, as the reference says of "M37702 version" printers. Use `PrintQRCode` (standard `GS ( k`), which prints proper QR codes. |
| GS V 0, GS V 65 n, ESC i, ESC m | No full cutter: all make a partial cut. GS V 65 n behaves like GS V 66 n. |
| GS C ; | The first GS c afterwards prints the value plus one step (value 50, step 10 prints 60, 70). |
| GS I 69 | Reports "CHINA GB18030", but the printer has no Kanji support (type ID multi-byte bit is 0). |

**Not supported:**

- FS Kanji commands (FS &, FS !, FS -, FS 2, FS S, FS W, FS .) and ESC 9. GBK
  bytes print as single-byte characters. These need a Chinese-firmware model.
- `ESC ( A` (an Epson beeper command used by the original `connordoman/pos`
  code). It prints stray characters such as `A0`; use `Beep` instead.

**Not yet verified on hardware:** FS q / FS p (NV images, which write
flash), GS ( A (test print; resets the printer), DC2 T (self-test page),
ESC p / DLE DC4 (no cash drawer attached), GS FF (black-mark paper only),
ESC c 5 (panel buttons) and DLE ENQ (needs an error state to recover from).

## Things the reference leaves unclear

- **GS I info strings** are sent as `_` + text + NUL. This is confirmed on an
  RP326, which reports itself as "EPSON TM-T88III", firmware "7.03 ESC/POS".
  `PrinterInfo` strips the `_` header.
- **The GS ( H reply** has no documented format. `WaitProcessID` waits for any
  NUL-terminated reply that ends in the ID; this works on the RP326.
- **ASB layout.** `ReadASB` returns the four bytes as they arrive, without
  decoding them. They appear to follow Epson's ASB format (the RP326 sent
  `14 00 00 0f`).
- **Real-time sequences inside data.** The printer acts on `DLE EOT`, `DLE ENQ`
  and `DLE DC4` byte sequences wherever they appear, including inside image
  data.
