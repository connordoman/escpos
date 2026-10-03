package escpos

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestCommandBytes checks the exact bytes of every command against the
// formats in the reference.
func TestCommandBytes(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name string
		fn   func(b *Builder)
		want []byte
	}{
		// Print and control commands.
		{"HT", func(b *Builder) { b.Tab() }, []byte{0x09}},
		{"LF", func(b *Builder) { b.LineFeed() }, []byte{0x0A}},
		{"CR", func(b *Builder) { b.CarriageReturn() }, []byte{0x0D}},
		{"ESC @", func(b *Builder) { b.Initialize() }, []byte{0x1B, 0x40}},
		{"ESC J", func(b *Builder) { b.FeedUnits(24) }, []byte{0x1B, 0x4A, 24}},
		{"ESC d", func(b *Builder) { b.FeedLines(3) }, []byte{0x1B, 0x64, 3}},
		{"ESC = on", func(b *Builder) { b.SetPeripheralDevice(true) }, []byte{0x1B, 0x3D, 1}},
		{"ESC = off", func(b *Builder) { b.SetPeripheralDevice(false) }, []byte{0x1B, 0x3D, 0}},
		{"DC2 T", func(b *Builder) { b.PrintTestPage() }, []byte{0x12, 0x54}},
		{"GS ( A", func(b *Builder) { must(b.ExecuteTestPrint(TestPaperRoll, TestPatternHexDump)) }, []byte{0x1D, 0x28, 0x41, 2, 0, 1, 1}},
		{"ESC c 5 enabled", func(b *Builder) { b.SetPanelButtons(true) }, []byte{0x1B, 0x63, 0x35, 0}},
		{"ESC c 5 disabled", func(b *Builder) { b.SetPanelButtons(false) }, []byte{0x1B, 0x63, 0x35, 1}},
		{"GS P", func(b *Builder) { b.SetMotionUnits(200, 400%256) }, []byte{0x1D, 0x50, 200, 144}},

		// Line spacing and position.
		{"ESC 2", func(b *Builder) { b.DefaultLineSpacing() }, []byte{0x1B, 0x32}},
		{"ESC 3", func(b *Builder) { b.SetLineSpacing(40) }, []byte{0x1B, 0x33, 40}},
		{"ESC a", func(b *Builder) { must(b.SetAlign(AlignCenter)) }, []byte{0x1B, 0x61, 1}},
		{"GS L", func(b *Builder) { b.SetLeftMargin(300) }, []byte{0x1D, 0x4C, 0x2C, 0x01}},
		{"GS W", func(b *Builder) { b.SetPrintAreaWidth(576) }, []byte{0x1D, 0x57, 64, 2}},
		{"ESC $", func(b *Builder) { b.SetAbsolutePosition(258) }, []byte{0x1B, 0x24, 2, 1}},
		{"ESC \\ right", func(b *Builder) { b.SetRelativePosition(10) }, []byte{0x1B, 0x5C, 10, 0}},
		{"ESC \\ left", func(b *Builder) { b.SetRelativePosition(-10) }, []byte{0x1B, 0x5C, 0xF6, 0xFF}},
		{"ESC D", func(b *Builder) { must(b.SetTabPositions(8, 16, 24)) }, []byte{0x1B, 0x44, 8, 16, 24, 0}},
		{"ESC D clear", func(b *Builder) { must(b.SetTabPositions()) }, []byte{0x1B, 0x44, 0}},

		// Character commands.
		{"ESC !", func(b *Builder) { b.SelectPrintMode(PrintModeEmphasized | PrintModeDoubleWidth) }, []byte{0x1B, 0x21, 0x28}},
		{"GS !", func(b *Builder) { must(b.SetCharacterSize(2, 3)) }, []byte{0x1D, 0x21, 0x12}},
		{"GS ! normal", func(b *Builder) { must(b.SetCharacterSize(1, 1)) }, []byte{0x1D, 0x21, 0x00}},
		{"GS ! max", func(b *Builder) { must(b.SetCharacterSize(8, 8)) }, []byte{0x1D, 0x21, 0x77}},
		{"GS B", func(b *Builder) { b.SetReverse(true) }, []byte{0x1D, 0x42, 1}},
		{"ESC V", func(b *Builder) { b.SetRotate90(true) }, []byte{0x1B, 0x56, 1}},
		{"ESC M", func(b *Builder) { must(b.SelectFont(FontB)) }, []byte{0x1B, 0x4D, 1}},
		{"ESC M ascii", func(b *Builder) { must(b.SelectFont('1')) }, []byte{0x1B, 0x4D, 49}},
		{"ESC G", func(b *Builder) { b.SetDoubleStrike(true) }, []byte{0x1B, 0x47, 1}},
		{"ESC E", func(b *Builder) { b.SetEmphasis(true) }, []byte{0x1B, 0x45, 1}},
		{"ESC E off", func(b *Builder) { b.SetEmphasis(false) }, []byte{0x1B, 0x45, 0}},
		{"ESC SP", func(b *Builder) { b.SetCharacterSpacing(4) }, []byte{0x1B, 0x20, 4}},
		{"ESC {", func(b *Builder) { b.SetUpsideDown(true) }, []byte{0x1B, 0x7B, 1}},
		{"ESC -", func(b *Builder) { must(b.SetUnderline(UnderlineDouble)) }, []byte{0x1B, 0x2D, 2}},
		{"ESC %", func(b *Builder) { b.SelectUserDefinedCharset(true) }, []byte{0x1B, 0x25, 1}},
		{"ESC &", func(b *Builder) {
			must(b.DefineUserCharacters('A', Glyph{Width: 2, Data: []byte{1, 2, 3, 4, 5, 6}}))
		}, []byte{0x1B, 0x26, 3, 'A', 'A', 2, 1, 2, 3, 4, 5, 6}},
		{"ESC & range", func(b *Builder) {
			must(b.DefineUserCharacters('A', Glyph{Width: 1, Data: []byte{1, 2, 3}}, Glyph{}))
		}, []byte{0x1B, 0x26, 3, 'A', 'B', 1, 1, 2, 3, 0}},
		{"ESC ?", func(b *Builder) { must(b.CancelUserCharacter('A')) }, []byte{0x1B, 0x3F, 'A'}},
		{"ESC R", func(b *Builder) { must(b.SelectInternationalCharset(CharsetUK)) }, []byte{0x1B, 0x52, 3}},
		{"ESC t", func(b *Builder) { b.SelectCodePage(CodePageWPC1252) }, []byte{0x1B, 0x74, 16}},
		{"ESC 9", func(b *Builder) { must(b.SelectChineseEncoding(ChineseEncodingUTF8)) }, []byte{0x1B, 0x39, 1}},

		// Kanji.
		{"FS !", func(b *Builder) { b.SetKanjiPrintMode(KanjiDoubleWidth | KanjiUnderline) }, []byte{0x1C, 0x21, 0x84}},
		{"FS &", func(b *Builder) { b.SelectKanjiMode() }, []byte{0x1C, 0x26}},
		{"FS .", func(b *Builder) { b.CancelKanjiMode() }, []byte{0x1C, 0x2E}},
		{"FS -", func(b *Builder) { must(b.SetKanjiUnderline(UnderlineSingle)) }, []byte{0x1C, 0x2D, 1}},
		{"FS S", func(b *Builder) { b.SetKanjiSpacing(1, 2) }, []byte{0x1C, 0x53, 1, 2}},
		{"FS W", func(b *Builder) { b.SetKanjiQuadruple(true) }, []byte{0x1C, 0x57, 1}},

		// Bit images.
		{"ESC *", func(b *Builder) { must(b.PrintBitImage(BitImage24DotDouble, []byte{1, 2, 3, 4, 5, 6})) }, []byte{0x1B, 0x2A, 33, 2, 0, 1, 2, 3, 4, 5, 6}},
		{"GS *", func(b *Builder) { must(b.DefineDownloadedBitImage(1, 1, make([]byte, 8))) }, append([]byte{0x1D, 0x2A, 1, 1}, make([]byte, 8)...)},
		{"GS /", func(b *Builder) { must(b.PrintDownloadedBitImage(ScaleQuadruple)) }, []byte{0x1D, 0x2F, 3}},
		{"GS v 0", func(b *Builder) { must(b.PrintRasterBitImage(ScaleNormal, 2, []byte{0xFF, 0x00, 0x0F, 0xF0})) }, []byte{0x1D, 0x76, 0x30, 0, 2, 0, 2, 0, 0xFF, 0x00, 0x0F, 0xF0}},
		{"FS p", func(b *Builder) { must(b.PrintNVBitImage(1, ScaleDoubleWidth)) }, []byte{0x1C, 0x70, 1, 1}},
		{"FS q", func(b *Builder) {
			must(b.DefineNVBitImages(NVBitImage{X: 1, Y: 1, Data: make([]byte, 8)}))
		}, append([]byte{0x1C, 0x71, 1, 1, 0, 1, 0}, make([]byte, 8)...)},

		// Status and IDs.
		{"GS r", func(b *Builder) { b.TransmitStatus() }, []byte{0x1D, 0x72, 1}},
		{"GS a", func(b *Builder) { b.SetAutoStatusBack(ASBErrorStatus | ASBPaperSensor) }, []byte{0x1D, 0x61, 0x0C}},
		{"GS I", func(b *Builder) { must(b.TransmitPrinterID(PrinterInfoFirmware)) }, []byte{0x1D, 0x49, 65}},
		{"GS ( H", func(b *Builder) { must(b.SetProcessIDResponse([4]byte{'J', 'O', 'B', '1'})) }, []byte{0x1D, 0x28, 0x48, 6, 0, 48, 48, 'J', 'O', 'B', '1'}},
		{"ESC p", func(b *Builder) { must(b.GeneratePulse(DrawerPin5, 25, 250)) }, []byte{0x1B, 0x70, 1, 25, 250}},
		{"ESC p drawer", func(b *Builder) { must(b.OpenCashDrawer(DrawerPin2)) }, []byte{0x1B, 0x70, 0, 50, 250}},

		// Bar codes.
		{"GS H", func(b *Builder) { must(b.SetHRIPosition(HRIBelow)) }, []byte{0x1D, 0x48, 2}},
		{"GS h", func(b *Builder) { must(b.SetBarcodeHeight(80)) }, []byte{0x1D, 0x68, 80}},
		{"GS w", func(b *Builder) { must(b.SetBarcodeWidth(2)) }, []byte{0x1D, 0x77, 2}},
		{"GS f", func(b *Builder) { must(b.SetHRIFont(FontB)) }, []byte{0x1D, 0x66, 1}},
		{"GS x", func(b *Builder) { b.SetBarcodeLeftSpace(20) }, []byte{0x1D, 0x78, 20}},
		{"GS k B", func(b *Builder) { must(b.PrintBarcode(BarcodeCode39, "AB-1")) }, []byte{0x1D, 0x6B, 69, 4, 'A', 'B', '-', '1'}},
		{"GS k A", func(b *Builder) { must(b.PrintBarcode(BarcodeEAN8NUL, "1234567")) }, []byte{0x1D, 0x6B, 3, '1', '2', '3', '4', '5', '6', '7', 0}},
		{"GS k CODE93", func(b *Builder) {
			must(b.PrintBarcode(BarcodeCode93, "Code\r93"))
		}, []byte{0x1D, 0x6B, 72, 7, 67, 111, 100, 101, 13, 57, 51}}, // reference example
		{"GS k CODE128", func(b *Builder) {
			must(b.PrintBarcode(BarcodeCode128, "{BNo.{C\x0c\x22\x38"))
		}, []byte{0x1D, 0x6B, 73, 10, 123, 66, 78, 111, 46, 123, 67, 12, 34, 56}}, // reference example

		// Two-dimensional bar codes.
		{"GS Z", func(b *Builder) { must(b.Select2DBarcodeType(Barcode2DQRCode)) }, []byte{0x1D, 0x5A, 1}},
		{"ESC Z", func(b *Builder) { must(b.Print2DBarcode(0, 'M', 4, "hi")) }, []byte{0x1B, 0x5A, 0, 'M', 4, 2, 0, 'h', 'i'}},
		{"QR", func(b *Builder) { must(b.PrintQRCode("hi", 0, QRErrorM, 6)) }, []byte{0x1D, 0x5A, 1, 0x1B, 0x5A, 0, 'M', 6, 2, 0, 'h', 'i'}},
		{"PDF417", func(b *Builder) { must(b.PrintPDF417("hi", 4, 2, 3)) }, []byte{0x1D, 0x5A, 0, 0x1B, 0x5A, 4, 2, 3, 2, 0, 'h', 'i'}},

		// Cutting, buzzer, macros.
		{"GS V partial", func(b *Builder) { must(b.Cut(CutPartial)) }, []byte{0x1D, 0x56, 1}},
		{"GS V full", func(b *Builder) { must(b.Cut(CutFull)) }, []byte{0x1D, 0x56, 0}},
		{"GS V 66", func(b *Builder) { b.FeedAndCut(80) }, []byte{0x1D, 0x56, 66, 80}},
		{"GS V 65", func(b *Builder) { b.FeedAndFullCut(0) }, []byte{0x1D, 0x56, 65, 0}},
		{"ESC i", func(b *Builder) { b.CutImmediate() }, []byte{0x1B, 0x69}},
		{"ESC m", func(b *Builder) { b.PartialCutImmediate() }, []byte{0x1B, 0x6D}},
		{"ESC B", func(b *Builder) { must(b.Beep(2, 3)) }, []byte{0x1B, 0x42, 2, 3}},
		{"ESC ( A buzz", func(b *Builder) { b.Buzz(10) }, []byte{0x1B, 0x28, 0x41, 4, 0, 0x30, 0, 1, 10}},
		{"GS :", func(b *Builder) { b.ToggleMacroDefinition() }, []byte{0x1D, 0x3A}},
		{"GS : macro", func(b *Builder) {
			must(b.DefineMacro(func(m *Builder) error { m.Text("hi"); return nil }))
		}, []byte{0x1D, 0x3A, 'h', 'i', 0x1D, 0x3A}},
		{"GS ^", func(b *Builder) { b.ExecuteMacro(3, 10, true) }, []byte{0x1D, 0x5E, 3, 10, 1}},

		// Page mode.
		{"ESC L", func(b *Builder) { b.EnterPageMode() }, []byte{0x1B, 0x4C}},
		{"ESC S", func(b *Builder) { b.EnterStandardMode() }, []byte{0x1B, 0x53}},
		{"ESC FF", func(b *Builder) { b.PrintPage() }, []byte{0x1B, 0x0C}},
		{"FF", func(b *Builder) { b.PrintPageAndExit() }, []byte{0x0C}},
		{"ESC T", func(b *Builder) { must(b.SetPageDirection(PageBottomToTop)) }, []byte{0x1B, 0x54, 1}},
		{"ESC W", func(b *Builder) { must(b.SetPageArea(0, 10, 576, 300)) }, []byte{0x1B, 0x57, 0, 0, 10, 0, 64, 2, 44, 1}},
		{"GS $", func(b *Builder) { b.SetAbsoluteVerticalPosition(100) }, []byte{0x1D, 0x24, 100, 0}},
		{"GS \\", func(b *Builder) { b.SetRelativeVerticalPosition(-1) }, []byte{0x1D, 0x5C, 0xFF, 0xFF}},
		{"GS FF", func(b *Builder) { b.FeedToMark() }, []byte{0x1D, 0x0C}},

		// Counter.
		{"GS C 0", func(b *Builder) { must(b.SetCounterPrintMode(3, CounterAlignRightZeros)) }, []byte{0x1D, 0x43, 0x30, 3, 1}},
		{"GS C 1", func(b *Builder) { b.SetCounterModeA(1, 1000, 1, 1) }, []byte{0x1D, 0x43, 0x31, 1, 0, 0xE8, 0x03, 1, 1}},
		{"GS C 2", func(b *Builder) { b.SetCounter(513) }, []byte{0x1D, 0x43, 0x32, 1, 2}},
		{"GS C ;", func(b *Builder) {
			b.SetCounterModeB(CounterModeB{A: new(uint16(1)), B: new(uint16(65535)), Value: new(uint8(7))})
		}, []byte("\x1dC;1;65535;;;7;")},
		{"GS C ; empty", func(b *Builder) { b.SetCounterModeB(CounterModeB{}) }, []byte("\x1dC;;;;;;")},
		{"GS c", func(b *Builder) { b.PrintCounter() }, []byte{0x1D, 0x63}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			tt.fn(&b)
			if got := b.Bytes(); !bytes.Equal(got, tt.want) {
				t.Errorf("got  % X\nwant % X", got, tt.want)
			}
		})
	}
}

// TestValidation checks that out-of-range parameters are rejected without
// writing anything.
func TestValidation(t *testing.T) {
	tests := []struct {
		name string
		fn   func(b *Builder) error
	}{
		{"test print paper", func(b *Builder) error { return b.ExecuteTestPrint(3, 1) }},
		{"test print pattern", func(b *Builder) error { return b.ExecuteTestPrint(0, 0) }},
		{"align", func(b *Builder) error { return b.SetAlign(3) }},
		{"tabs not ascending", func(b *Builder) error { return b.SetTabPositions(8, 8) }},
		{"tabs zero", func(b *Builder) error { return b.SetTabPositions(0) }},
		{"tabs too many", func(b *Builder) error { return b.SetTabPositions(make([]uint8, 33)...) }},
		{"font", func(b *Builder) error { return b.SelectFont(3) }},
		{"size zero", func(b *Builder) error { return b.SetCharacterSize(0, 1) }},
		{"size nine", func(b *Builder) error { return b.SetCharacterSize(1, 9) }},
		{"underline", func(b *Builder) error { return b.SetUnderline(3) }},
		{"charset", func(b *Builder) error { return b.SelectInternationalCharset(16) }},
		{"chinese", func(b *Builder) error { return b.SelectChineseEncoding(2) }},
		{"glyph width", func(b *Builder) error { return b.DefineUserCharacters('A', Glyph{Width: 13, Data: make([]byte, 39)}) }},
		{"glyph data", func(b *Builder) error { return b.DefineUserCharacters('A', Glyph{Width: 2, Data: make([]byte, 5)}) }},
		{"glyph code", func(b *Builder) error { return b.DefineUserCharacters(0x7E, Glyph{}, Glyph{}) }},
		{"cancel glyph", func(b *Builder) error { return b.CancelUserCharacter(0x7F) }},
		{"kanji code", func(b *Builder) error { return b.DefineKanjiCharacter(0xFD, 0xA1, make([]byte, 72)) }},
		{"kanji data", func(b *Builder) error { return b.DefineKanjiCharacter(0xFE, 0xA1, make([]byte, 32)) }},
		{"bit image mode", func(b *Builder) error { return b.PrintBitImage(2, []byte{1}) }},
		{"bit image length", func(b *Builder) error { return b.PrintBitImage(BitImage24DotSingle, []byte{1, 2}) }},
		{"bit image width", func(b *Builder) error { return b.PrintBitImage(BitImage8DotDouble, make([]byte, 1024)) }},
		{"downloaded size", func(b *Builder) error { return b.DefineDownloadedBitImage(40, 40, make([]byte, 40*40*8)) }},
		{"downloaded data", func(b *Builder) error { return b.DefineDownloadedBitImage(1, 1, make([]byte, 7)) }},
		{"raster width", func(b *Builder) error { return b.PrintRasterBitImage(0, 129, make([]byte, 129)) }},
		{"raster height", func(b *Builder) error { return b.PrintRasterBitImage(0, 1, make([]byte, 4096)) }},
		{"raster scale", func(b *Builder) error { return b.PrintRasterBitImage(4, 1, []byte{1}) }},
		{"nv number", func(b *Builder) error { return b.PrintNVBitImage(0, 0) }},
		{"nv none", func(b *Builder) error { return b.DefineNVBitImages() }},
		{"nv size", func(b *Builder) error {
			return b.DefineNVBitImages(NVBitImage{X: 1, Y: 289, Data: make([]byte, 289*8)})
		}},
		{"nv capacity", func(b *Builder) error {
			img := NVBitImage{X: 128, Y: 100, Data: make([]byte, 128*100*8)}
			return b.DefineNVBitImages(img, img)
		}},
		{"hri position", func(b *Builder) error { return b.SetHRIPosition(4) }},
		{"hri font", func(b *Builder) error { return b.SetHRIFont(FontC) }},
		{"barcode height", func(b *Builder) error { return b.SetBarcodeHeight(0) }},
		{"barcode width", func(b *Builder) error { return b.SetBarcodeWidth(7) }},
		{"upca length", func(b *Builder) error { return b.PrintBarcode(BarcodeUPCA, "1234") }},
		{"upca digits", func(b *Builder) error { return b.PrintBarcode(BarcodeUPCA, "12345678901A") }},
		{"ean13 length", func(b *Builder) error { return b.PrintBarcode(BarcodeEAN13NUL, "12345678901") }},
		{"code39 chars", func(b *Builder) error { return b.PrintBarcode(BarcodeCode39, "abc") }},
		{"itf odd", func(b *Builder) error { return b.PrintBarcode(BarcodeITF, "123") }},
		{"codabar chars", func(b *Builder) error { return b.PrintBarcode(BarcodeCodabar, "E") }},
		{"code93 non-ascii", func(b *Builder) error { return b.PrintBarcode(BarcodeCode93, "é") }},
		{"code128 no set", func(b *Builder) error { return b.PrintBarcode(BarcodeCode128, "ABC") }},
		{"code128 bad escape", func(b *Builder) error { return b.PrintBarcode(BarcodeCode128, "{BA{X") }},
		{"code128 trailing escape", func(b *Builder) error { return b.PrintBarcode(BarcodeCode128, "{BA{") }},
		{"barcode system", func(b *Builder) error { return b.PrintBarcode(7, "1") }},
		{"barcode too long", func(b *Builder) error { return b.PrintBarcode(BarcodeCode39, strings.Repeat("A", 256)) }},
		{"2d type", func(b *Builder) error { return b.Select2DBarcodeType(2) }},
		{"2d empty", func(b *Builder) error { return b.Print2DBarcode(0, 0, 0, "") }},
		{"qr version", func(b *Builder) error { return b.PrintQRCode("x", 41, QRErrorL, 4) }},
		{"qr module", func(b *Builder) error { return b.PrintQRCode("x", 0, QRErrorL, 9) }},
		{"pdf417 columns", func(b *Builder) error { return b.PrintPDF417("x", 31, 0, 2) }},
		{"pdf417 security", func(b *Builder) error { return b.PrintPDF417("x", 1, 9, 2) }},
		{"pdf417 ratio", func(b *Builder) error { return b.PrintPDF417("x", 1, 0, 6) }},
		{"cut mode", func(b *Builder) error { return b.Cut(2) }},
		{"pulse pin", func(b *Builder) error { return b.GeneratePulse(2, 1, 1) }},
		{"beep", func(b *Builder) error { return b.Beep(0, 1) }},
		{"macro size", func(b *Builder) error {
			return b.DefineMacro(func(m *Builder) error { m.Raw(make([]byte, MaxMacroSize+1)...); return nil })
		}},
		{"page direction", func(b *Builder) error { return b.SetPageDirection(4) }},
		{"page area", func(b *Builder) error { return b.SetPageArea(0, 0, 0, 10) }},
		{"counter digits", func(b *Builder) error { return b.SetCounterPrintMode(6, 0) }},
		{"counter align", func(b *Builder) error { return b.SetCounterPrintMode(1, 3) }},
		{"printer id", func(b *Builder) error { return b.TransmitPrinterID(3) }},
		{"process id", func(b *Builder) error { return b.SetProcessIDResponse([4]byte{'a', 'b', 'c', 0x7F}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			err := tt.fn(&b)
			if !errors.Is(err, ErrInvalidArgument) {
				t.Fatalf("got error %v, want ErrInvalidArgument", err)
			}
			if b.Len() != 0 {
				t.Errorf("wrote % X on error", b.Bytes())
			}
		})
	}
}

func TestText(t *testing.T) {
	tests := []struct {
		name string
		cp   *CodePage
		in   string
		want []byte
	}{
		{"ascii", nil, "Hello\n", []byte("Hello\n")},
		{"cp437 default", nil, "café ─", []byte{'c', 'a', 'f', 0x82, ' ', 0xC4}},
		{"fallback", nil, "“hi” — ok…", []byte(`"hi" - ok...`)},
		{"unknown", nil, "日", []byte("?")},
		{"cp1252", new(CodePageWPC1252), "€", []byte{0x1B, 0x74, 16, 0x80}},
		{"unmapped code page", new(CodePageThai), "é", []byte{0x1B, 0x74, 26, '?'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Builder
			if tt.cp != nil {
				b.SelectCodePage(*tt.cp)
			}
			b.Text(tt.in)
			if got := b.Bytes(); !bytes.Equal(got, tt.want) {
				t.Errorf("got % X, want % X", got, tt.want)
			}
		})
	}
}

func TestLayout(t *testing.T) {
	var b Builder
	if got := b.CharactersPerLine(); got != 48 {
		t.Errorf("Font A on 80 mm: got %d columns, want 48", got)
	}
	_ = b.SelectFont(FontB)
	if got := b.CharactersPerLine(); got != 64 {
		t.Errorf("Font B on 80 mm: got %d columns, want 64", got)
	}
	_ = b.SetCharacterSize(2, 2)
	if got := b.CharactersPerLine(); got != 32 {
		t.Errorf("Font B double width: got %d columns, want 32", got)
	}
	b.Initialize()
	if got := b.CharactersPerLine(); got != 48 {
		t.Errorf("after ESC @: got %d columns, want 48", got)
	}
	b.Reset()

	b.Columns("Total", "$4.50")
	if got, want := string(b.Bytes()), "Total"+strings.Repeat(" ", 38)+"$4.50\n"; got != want {
		t.Errorf("Columns: got %q, want %q", got, want)
	}
	b.Reset()
	b.SetPaperWidth(PaperWidth58mm)
	b.HorizontalRule('-')
	if got, want := string(b.Bytes()), strings.Repeat("-", 36)+"\n"; got != want {
		t.Errorf("HorizontalRule: got %q, want %q", got, want)
	}
}

func TestCode128(t *testing.T) {
	tests := map[string]string{
		"123456": "{C\x0c\x22\x38",
		"12345":  "{B12345",
		"12":     "{B12",
		"a{b":    "{Ba{{b",
	}
	for in, want := range tests {
		if got := Code128(in); got != want {
			t.Errorf("Code128(%q) = %q, want %q", in, got, want)
		}
		var b Builder
		if err := b.PrintBarcode(BarcodeCode128, Code128(in)); err != nil {
			t.Errorf("Code128(%q) is not printable: %v", in, err)
		}
	}
}
