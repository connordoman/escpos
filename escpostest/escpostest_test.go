package escpostest

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"github.com/connordoman/escpos"
)

// TestEveryCommandDecodes runs every Builder method, each followed by a
// "|" marker. If the decoder got any command's length wrong, payload bytes
// would show up as text or a marker would be swallowed.
func TestEveryCommandDecodes(t *testing.T) {
	b := escpos.NewBuilder(escpos.PaperWidth80mm)
	steps := []func() error{
		func() error { b.Initialize(); return nil },
		func() error { b.Tab(); return nil },
		func() error { b.CarriageReturn(); return nil },
		func() error { b.FeedUnits(30); return nil },
		func() error { b.FeedLines(2); return nil },
		func() error { b.SetPeripheralDevice(true); return nil },
		func() error { b.PrintTestPage(); return nil },
		func() error { return b.ExecuteTestPrint(escpos.TestPaperRoll, escpos.TestPatternRolling) },
		func() error { b.SetPanelButtons(false); return nil },
		func() error { b.SetMotionUnits(180, 180); return nil },
		func() error { return b.SetAlign(escpos.AlignCenter) },
		func() error { b.DefaultLineSpacing(); return nil },
		func() error { b.SetLineSpacing(24); return nil },
		func() error { b.SetLeftMargin(300); return nil },
		func() error { b.SetPrintAreaWidth(400); return nil },
		func() error { b.SetAbsolutePosition(260); return nil },
		func() error { b.SetRelativePosition(-10); return nil },
		func() error { return b.SetTabPositions(8, 16, 24) },
		func() error { return b.SelectFont(escpos.FontB) },
		func() error { b.SelectPrintMode(escpos.PrintModeEmphasized | escpos.PrintModeDoubleWidth); return nil },
		func() error { return b.SetCharacterSize(2, 3) },
		func() error { b.SetEmphasis(true); return nil },
		func() error { b.SetDoubleStrike(true); return nil },
		func() error { return b.SetUnderline(escpos.UnderlineDouble) },
		func() error { b.SetReverse(true); return nil },
		func() error { b.SetRotate90(true); return nil },
		func() error { b.SetUpsideDown(true); return nil },
		func() error { b.SetCharacterSpacing(2); return nil },
		func() error { return b.SelectInternationalCharset(escpos.CharsetUK) },
		func() error { return b.SelectChineseEncoding(escpos.ChineseEncodingGBK) },
		func() error { b.SelectUserDefinedCharset(true); return nil },
		func() error {
			return b.DefineUserCharacters('A', escpos.Glyph{Width: 2, Data: []byte{0x1B, '|', 3, 4, 5, 6}})
		},
		func() error { return b.CancelUserCharacter('A') },
		func() error { b.SetKanjiPrintMode(escpos.KanjiUnderline); return nil },
		func() error { b.SelectKanjiMode(); return nil },
		func() error { b.CancelKanjiMode(); return nil },
		func() error { return b.SetKanjiUnderline(escpos.UnderlineSingle) },
		func() error { return b.DefineKanjiCharacter(0xFE, 0xA1, bytes.Repeat([]byte{'|'}, 72)) },
		func() error { b.SetKanjiSpacing(1, 2); return nil },
		func() error { b.SetKanjiQuadruple(true); return nil },
		func() error { return b.PrintBitImage(escpos.BitImage24DotSingle, bytes.Repeat([]byte{0x1D}, 9)) },
		func() error { return b.DefineDownloadedBitImage(1, 1, bytes.Repeat([]byte{'|'}, 8)) },
		func() error { return b.PrintDownloadedBitImage(0) },
		func() error { return b.PrintRasterBitImage(0, 2, []byte{0x1B, '|', 0x0A, 0}) },
		func() error { return b.PrintImage(image.NewGray(image.Rect(0, 0, 16, 3)), escpos.ImageOptions{}) },
		func() error { return b.PrintNVBitImage(1, 0) },
		func() error {
			return b.DefineNVBitImages(escpos.NVBitImage{X: 1, Y: 1, Data: bytes.Repeat([]byte{'|'}, 8)})
		},
		func() error { return b.SetHRIPosition(escpos.HRIBelow) },
		func() error { return b.SetHRIFont(escpos.FontB) },
		func() error { return b.SetBarcodeHeight(60) },
		func() error { return b.SetBarcodeWidth(2) },
		func() error { b.SetBarcodeLeftSpace(4); return nil },
		func() error { return b.PrintBarcode(escpos.BarcodeCode128, escpos.Code128("A|B")) },
		func() error { return b.PrintBarcode(escpos.BarcodeCode39NUL, "ABC") },
		func() error { return b.Select2DBarcodeType(escpos.Barcode2DQRCode) },
		func() error { return b.PrintQRCodeESCZ("x|y", 0, escpos.QRErrorM, 4) },
		func() error { return b.PrintPDF417("pdf|417", 4, 2, 3) },
		func() error { return b.PrintQRCode("https://example.com/|", escpos.QRErrorH, 6) },
		func() error { return b.SelectQRCodeModel(escpos.QRModel2) },
		func() error { b.PrintStoredQRCode(); return nil },
		func() error { b.EnterPageMode(); return nil },
		func() error { return b.SetPageArea(0, 0, 576, 300) },
		func() error { return b.SetPageDirection(escpos.PageBottomToTop) },
		func() error { b.SetAbsoluteVerticalPosition(20); return nil },
		func() error { b.SetRelativeVerticalPosition(-5); return nil },
		func() error { b.PrintPage(); return nil },
		func() error { b.PrintPageAndExit(); return nil },
		func() error { b.EnterStandardMode(); return nil },
		func() error { b.FeedToMark(); return nil },
		func() error { return b.Cut(escpos.CutPartial) },
		func() error { b.FeedAndCut(10); return nil },
		func() error { b.FeedAndFullCut(0); return nil },
		func() error { b.CutImmediate(); return nil },
		func() error { b.PartialCutImmediate(); return nil },
		func() error { return b.GeneratePulse(escpos.DrawerPin5, 50, 250) },
		func() error { return b.Beep(1, 2) },
		func() error { return b.DefineMacro(func(m *escpos.Builder) error { m.SetEmphasis(true); return nil }) },
		func() error { b.ExecuteMacro(2, 10, false); return nil },
		func() error { return b.SetCounterPrintMode(5, escpos.CounterAlignRightZeros) },
		func() error { b.SetCounterModeA(1, 100, 1, 1); return nil },
		func() error { b.SetCounter(42); return nil },
		func() error {
			v, step := uint16(7), uint8(2)
			b.SetCounterModeB(escpos.CounterModeB{A: &v, Step: &step})
			return nil
		},
		func() error { b.PrintCounter(); return nil },
		func() error { b.TransmitStatus(); return nil },
		func() error { b.SetAutoStatusBack(escpos.ASBPaperSensor); return nil },
		func() error { return b.TransmitPrinterID(escpos.PrinterInfoFirmware) },
		func() error { return b.SetProcessIDResponse([4]byte{'J', 'O', 'B', '|'}) },
		func() error { b.SelectCodePage(escpos.CodePageWPC1252); return nil },
		func() error { b.Raw(escpos.DLE, escpos.EOT, 1, escpos.DLE, escpos.DC4, 1, 0, 1); return nil },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		b.Text("|")
	}

	markers := 0
	for _, tok := range Decode(b.Bytes()) {
		if tok.Truncated {
			t.Errorf("truncated: %v", tok)
		}
		if tok.IsText() {
			if strings.Trim(tok.Text, "|") != "" {
				t.Errorf("payload decoded as text: %q", tok.Text)
			}
			markers += len(tok.Text)
		}
	}
	if markers != len(steps) {
		t.Errorf("found %d markers, want %d:\n%s", markers, len(steps), Describe(b.Bytes()))
	}
}

func TestDescribe(t *testing.T) {
	b := escpos.NewBuilder(escpos.PaperWidth80mm)
	b.Initialize()
	b.SetEmphasis(true)
	b.Textln("café")
	b.SelectCodePage(escpos.CodePageWPC1252)
	b.Text("€5")
	b.FeedAndCut(0)
	want := "<ESC @><ESC E 1>café⏎\n<ESC t 16>€5<GS V 66 0>"
	if got := Describe(b.Bytes()); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestPayloadSummaries(t *testing.T) {
	b := escpos.NewBuilder(escpos.PaperWidth80mm)
	b.PrintQRCode("https://example.com", escpos.QRErrorM, 4)
	b.PrintBarcode(escpos.BarcodeEAN13, "4006381333931")
	b.PrintImage(image.NewGray(image.Rect(0, 0, 32, 5)), escpos.ImageOptions{})
	d := Describe(b.Bytes())
	for _, want := range []string{
		`<GS ( k 49 80 48 "https://example.com">`,
		`<GS ( k 49 81 48>`,
		`<GS k 67 "4006381333931">`,
		`<GS v 0 0 4 5 (32×5 dots)>`,
	} {
		if !strings.Contains(d, want) {
			t.Errorf("%s lacks %s", d, want)
		}
	}
}

func TestLinesAndCommands(t *testing.T) {
	b := escpos.NewBuilder(escpos.PaperWidth80mm)
	b.SetEmphasis(true)
	b.Textln("one")
	b.SetEmphasis(false)
	b.Text("t")
	b.Textln("wo")
	b.Text("unfinished")
	if got := Lines(b.Bytes()); len(got) != 2 || got[0] != "one" || got[1] != "two" {
		t.Errorf("Lines = %q", got)
	}
	if got := Commands(b.Bytes(), "ESC E"); len(got) != 2 || got[1].Args[0] != 0 {
		t.Errorf("Commands = %v", got)
	}
}

func TestTruncated(t *testing.T) {
	toks := Decode([]byte{escpos.GS, 'v', '0', 0, 10, 0, 10, 0, 1, 2, 3})
	if len(toks) != 1 || !toks[0].Truncated {
		t.Errorf("got %v", toks)
	}
}
