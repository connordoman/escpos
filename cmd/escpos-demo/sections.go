package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/connordoman/escpos"
)

// check collects errors from builder calls so sections read top to bottom.
type check struct{ err error }

func (c *check) do(err error) { c.err = errors.Join(c.err, err) }

func headerSection(b *escpos.Builder) error {
	var c check
	c.do(b.SetAlign(escpos.AlignCenter))
	c.do(b.SetCharacterSize(2, 2))
	b.SetEmphasis(true)
	b.Textln("LOREM CAFÉ")
	b.SetEmphasis(false)
	c.do(b.SetCharacterSize(1, 1))
	b.Textln("123 Ipsum Street, Dolor City")
	b.Textln("Tel. (555) 010-2026")
	b.Textln(time.Now().Format("Mon 2 Jan 2006 15:04"))
	b.FeedLines(1)

	c.do(b.SetCharacterSize(1, 2))
	b.Textln("Double height")
	c.do(b.SetCharacterSize(2, 1))
	b.Textln("Double width")
	c.do(b.SetCharacterSize(3, 3))
	b.Textln("3x3")
	c.do(b.SetCharacterSize(1, 1))

	b.SetReverse(true)
	b.Textln(" ORDER #0042 ")
	b.SetReverse(false)
	c.do(b.SetAlign(escpos.AlignLeft))
	return c.err
}

func textSection(b *escpos.Builder) error {
	var c check
	c.do(b.SelectFont(escpos.FontA))
	b.Textln("Font A (12x24): Lorem ipsum dolor sit amet")
	c.do(b.SelectFont(escpos.FontB))
	b.Textln("Font B (9x17): Lorem ipsum dolor sit amet, consectetur")
	c.do(b.SelectFont(escpos.FontA))

	b.SetEmphasis(true)
	b.Textln("Emphasized (bold)")
	b.SetEmphasis(false)
	b.SetDoubleStrike(true)
	b.Textln("Double-strike")
	b.SetDoubleStrike(false)
	c.do(b.SetUnderline(escpos.UnderlineSingle))
	b.Textln("Underline, 1 dot")
	c.do(b.SetUnderline(escpos.UnderlineDouble))
	b.Textln("Underline, 2 dots")
	c.do(b.SetUnderline(escpos.UnderlineOff))
	b.SetReverse(true)
	b.Textln(" White on black ")
	b.SetReverse(false)

	b.SelectPrintMode(escpos.PrintModeFontB | escpos.PrintModeEmphasized | escpos.PrintModeUnderline)
	b.Textln("ESC ! combined: Font B + bold + underline")
	b.SelectPrintMode(0)

	b.SetUpsideDown(true)
	b.Textln("Upside down")
	b.SetUpsideDown(false)
	b.SetRotate90(true)
	b.Textln("Rotated 90")
	b.SetRotate90(false)

	for _, n := range []uint8{0, 4, 8} {
		b.SetCharacterSpacing(n)
		b.Textf("Spacing %d: ipsum\n", n)
	}
	b.SetCharacterSpacing(0)

	b.SetLineSpacing(18)
	b.Textln("Tight line spacing (18 units)")
	b.Textln("Tight line spacing (18 units)")
	b.SetLineSpacing(60)
	b.Textln("Loose line spacing (60 units)")
	b.Textln("Loose line spacing (60 units)")
	b.DefaultLineSpacing()

	for _, a := range []struct {
		align escpos.Align
		name  string
	}{{escpos.AlignLeft, "Left"}, {escpos.AlignCenter, "Centre"}, {escpos.AlignRight, "Right"}} {
		c.do(b.SetAlign(a.align))
		b.Textln(a.name + " aligned")
	}
	c.do(b.SetAlign(escpos.AlignLeft))

	for w := uint8(1); w <= 4; w++ {
		c.do(b.SetCharacterSize(w, w))
		b.Textf("%dx", w)
	}
	c.do(b.SetCharacterSize(1, 1))
	b.LineFeed()
	return c.err
}

func codePageSection(b *escpos.Builder) error {
	var c check
	b.Textln("CP437 (default):")
	b.Textln("  Café, naïve, jalapeño, Ångström")
	b.Textln("  ½ ¼ ° ± £ ¥ ß µ")
	b.Textln("  ┌────────┬────────┐")
	b.Textln("  │ ipsum  │ dolor  │")
	b.Textln("  └────────┴────────┘")

	b.SelectCodePage(escpos.CodePageWPC1252)
	b.Textln("WPC1252: €4.50 · Größe · façade")
	b.SelectCodePage(escpos.CodePagePC866)
	b.Textln("CP866: Привет, мир")
	b.SelectCodePage(escpos.CodePageWPC1253)
	b.Textln("WPC1253: Καλημέρα κόσμε")
	b.SelectCodePage(escpos.CodePagePC437)

	b.Textln("Fallbacks: “quotes” — dash… emoji 🙂")

	c.do(b.SelectInternationalCharset(escpos.CharsetUK))
	b.Textln("ESC R UK: # prints as a pound sign")
	c.do(b.SelectInternationalCharset(escpos.CharsetUSA))
	return c.err
}

func receiptSection(b *escpos.Builder) error {
	var c check
	items := []struct {
		qty   int
		name  string
		cents int
	}{
		{2, "Lorem latte", 450},
		{1, "Ipsum croissant", 375},
		{3, "Dolor biscotti", 125},
		{1, "Sit amet sandwich", 1095},
	}

	// Tab stops at columns 5 and 36 give a quantity / item / price table.
	c.do(b.SetTabPositions(5, 36))
	b.SetEmphasis(true)
	b.Textln("Qty\tItem\tPrice")
	b.SetEmphasis(false)
	b.HorizontalRule('─')
	total := 0
	for _, it := range items {
		line := it.qty * it.cents
		total += line
		b.Textf("%d\t%s\t%s\n", it.qty, it.name, money(line))
	}
	b.HorizontalRule('─')

	tax := total * 5 / 100
	b.Columns("Subtotal", money(total))
	b.Columns("Tax (5%)", money(tax))
	b.SetEmphasis(true)
	c.do(b.SetCharacterSize(1, 2))
	b.Columns("TOTAL", money(total+tax))
	c.do(b.SetCharacterSize(1, 1))
	b.SetEmphasis(false)
	b.FeedLines(1)

	b.SetLeftMargin(48)
	b.Textln("Left margin of 48 dots: lorem ipsum dolor sit amet, consectetur adipiscing elit.")
	b.SetLeftMargin(0)

	b.Text("ESC $:")
	b.SetAbsolutePosition(240)
	b.Text("at 240")
	b.SetRelativePosition(48)
	b.Textln("+48 (ESC \\)")
	return c.err
}

func money(cents int) string { return fmt.Sprintf("$%d.%02d", cents/100, cents%100) }

func lipsumSection(b *escpos.Builder) error {
	var c check
	for _, p := range lipsum[:2] {
		for _, line := range wrap(p, b.CharactersPerLine()) {
			b.Textln(line)
		}
		b.LineFeed()
	}
	c.do(b.SelectFont(escpos.FontB))
	c.do(b.SetAlign(escpos.AlignRight))
	for _, line := range wrap(lipsum[2], b.CharactersPerLine()) {
		b.Textln(line)
	}
	c.do(b.SetAlign(escpos.AlignLeft))
	c.do(b.SelectFont(escpos.FontA))
	return c.err
}

// wrap breaks text into lines of at most width runes at word boundaries.
func wrap(text string, width int) []string {
	var lines []string
	var line strings.Builder
	n := 0
	for _, word := range strings.Fields(text) {
		wl := len([]rune(word))
		if n > 0 && n+1+wl > width {
			lines = append(lines, line.String())
			line.Reset()
			n = 0
		}
		if n > 0 {
			line.WriteByte(' ')
			n++
		}
		line.WriteString(word)
		n += wl
	}
	if n > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

func barcodeSection(b *escpos.Builder) error {
	var c check
	c.do(b.SetBarcodeHeight(60))
	c.do(b.SetBarcodeWidth(2))
	c.do(b.SetHRIPosition(escpos.HRIBelow))
	c.do(b.SetHRIFont(escpos.FontB))
	codes := []struct {
		name string
		sys  escpos.BarcodeSystem
		data string
	}{
		{"UPC-A", escpos.BarcodeUPCA, "03600029145"},
		{"UPC-E", escpos.BarcodeUPCE, "01234500006"},
		{"EAN-13", escpos.BarcodeEAN13, "590123412345"},
		{"EAN-8", escpos.BarcodeEAN8, "9638507"},
		{"CODE39", escpos.BarcodeCode39, "LOREM-42"},
		{"ITF", escpos.BarcodeITF, "12345678"},
		{"CODABAR", escpos.BarcodeCodabar, "A40156B"},
		{"CODE93", escpos.BarcodeCode93, "Ipsum93"},
		{"CODE128", escpos.BarcodeCode128, escpos.Code128("Dolor-2026")},
		{"CODE128 (set C)", escpos.BarcodeCode128, escpos.Code128("20261003")},
		{"EAN-8, NUL-terminated form", escpos.BarcodeEAN8NUL, "1234567"},
	}
	c.do(b.SetAlign(escpos.AlignCenter))
	for _, code := range codes {
		b.Textln(code.name)
		c.do(b.PrintBarcode(code.sys, code.data))
		b.FeedLines(1)
	}
	c.do(b.SetHRIPosition(escpos.HRIBoth))
	c.do(b.SetBarcodeWidth(3))
	b.Textln("HRI above and below, width 3")
	c.do(b.PrintBarcode(escpos.BarcodeCode39, "SIT"))
	b.LineFeed()
	c.do(b.SetAlign(escpos.AlignLeft))
	return c.err
}

func twoDSection(b *escpos.Builder) error {
	var c check
	c.do(b.SetAlign(escpos.AlignCenter))
	b.Textln("QR code (GS ( k), EC level M")
	c.do(b.PrintQRCode("https://github.com/connordoman/escpos", escpos.QRErrorM, 6))
	b.LineFeed()
	b.Textln("QR code (GS ( k), EC level H, module 4")
	c.do(b.PrintQRCode("Lorem ipsum dolor sit amet, consectetur adipiscing elit.", escpos.QRErrorH, 4))
	b.LineFeed()
	b.Textln("PDF417, 4 columns, security 2")
	c.do(b.SetBarcodeWidth(2))
	c.do(b.PrintPDF417("ORDER-0042|LOREM CAFE|$27.26", 4, 2, 3))
	b.LineFeed()
	c.do(b.SetAlign(escpos.AlignLeft))
	return c.err
}

func imageSection(b *escpos.Builder) error {
	var c check
	c.do(b.SetAlign(escpos.AlignCenter))

	b.Textln("Raster image (GS v 0), thresholded")
	c.do(b.PrintImage(logoImage(384, 128), escpos.ImageOptions{}))
	b.LineFeed()

	width := b.PaperWidth()
	b.Textln("Gradient, thresholded")
	c.do(b.PrintImage(gradientImage(width, 48), escpos.ImageOptions{}))
	b.Textln("Gradient, dithered")
	c.do(b.PrintImage(gradientImage(width, 48), escpos.ImageOptions{Dither: true}))
	b.LineFeed()

	b.Textln("Column image band (ESC * 33)")
	_, _, band := escpos.NewBitmap(waveImage(min(width, 1023), 24), escpos.ImageOptions{}).Columns()
	c.do(b.PrintBitImage(escpos.BitImage24DotDouble, band))
	b.LineFeed()

	b.Textln("Downloaded bit image (GS *, GS /): normal, quadruple")
	x, y, data := escpos.NewBitmap(logoImage(64, 48), escpos.ImageOptions{}).Columns()
	c.do(b.DefineDownloadedBitImage(uint8(x), uint8(y), data))
	c.do(b.PrintDownloadedBitImage(escpos.ScaleNormal))
	c.do(b.PrintDownloadedBitImage(escpos.ScaleQuadruple))
	b.LineFeed()

	c.do(b.SetAlign(escpos.AlignLeft))
	return c.err
}

func userCharSection(b *escpos.Builder) error {
	var c check
	// Replace '{', '|' and '}' in Font A with a smiley, heart and tick.
	var glyphs []escpos.Glyph
	for _, img := range []func() *grayImage{smileyGlyph, heartGlyph, tickGlyph} {
		_, _, data := escpos.NewBitmap(img(), escpos.ImageOptions{}).Columns()
		// Columns pads the 12-dot width to 16; keep the first 12 columns.
		glyphs = append(glyphs, escpos.Glyph{Width: 12, Data: data[:12*3]})
	}
	c.do(b.SelectFont(escpos.FontA))
	c.do(b.DefineUserCharacters('{', glyphs...))
	b.SelectUserDefinedCharset(true)
	b.Textln("User-defined: { | }  { | }  { | }")
	b.SelectUserDefinedCharset(false)
	b.Textln("Built-in:     { | }")
	for _, ch := range []byte("{|}") {
		c.do(b.CancelUserCharacter(ch))
	}
	return c.err
}

func pageModeSection(b *escpos.Builder) error {
	var c check
	width := uint16(b.PaperWidth())
	b.EnterPageMode()
	c.do(b.SetPageArea(0, 0, width, 260))

	c.do(b.SetPageDirection(escpos.PageLeftToRight))
	b.Textln("Left to right, from the top left")
	b.SetAbsoluteVerticalPosition(120)
	b.SetAbsolutePosition(width / 3)
	b.Textln("Placed at (1/3 width, 120)")

	c.do(b.SetPageDirection(escpos.PageBottomToTop))
	b.Textln("Bottom to top")
	c.do(b.SetPageDirection(escpos.PageTopToBottom))
	b.Textln("Top to bottom")
	c.do(b.SetPageDirection(escpos.PageRightToLeft))
	b.Textln("Right to left")

	b.PrintPageAndExit()
	return c.err
}

func counterSection(b *escpos.Builder) error {
	var c check
	c.do(b.SetCounterPrintMode(5, escpos.CounterAlignRightZeros))
	b.SetCounterModeA(1, 65535, 1, 1)
	b.SetCounter(42)
	for range 3 {
		b.Text("Ticket #")
		b.PrintCounter()
		b.LineFeed()
	}
	b.SetCounterModeB(escpos.CounterModeB{Step: new(uint8(5)), Value: new(uint8(100))})
	for range 2 {
		b.Text("Step 5: ")
		b.PrintCounter()
		b.LineFeed()
	}
	return c.err
}

func macroSection(b *escpos.Builder) error {
	var c check
	c.do(b.DefineMacro(func(m *escpos.Builder) error {
		m.Textln("  * printed by the macro *")
		return nil
	}))
	b.Textln("Macro defined; running it 3 times:")
	b.ExecuteMacro(3, 2, false)
	return c.err
}

func beepSection(b *escpos.Builder) error {
	var c check
	b.Textln("Beep (ESC B 2 2)")
	c.do(b.Beep(2, 2))
	return c.err
}

func drawerSection(b *escpos.Builder) error {
	b.Textln("Kicking the cash drawer on pin 2")
	return b.OpenCashDrawer(escpos.DrawerPin2)
}

func nvSection(b *escpos.Builder) error {
	var c check
	b.Textln("Storing NV bit image 1 (flash write)")
	nv := escpos.NewBitmap(logoImage(256, 96), escpos.ImageOptions{}).NVBitImage()
	c.do(b.DefineNVBitImages(nv))
	c.do(b.SetAlign(escpos.AlignCenter))
	c.do(b.PrintNVBitImage(1, escpos.ScaleNormal))
	c.do(b.SetAlign(escpos.AlignLeft))
	return c.err
}

func selfTestSection(b *escpos.Builder) error {
	b.PrintTestPage()
	return nil
}
