package main

import (
	"fmt"
	"io"
	"slices"

	"github.com/connordoman/escpos"
	"github.com/spf13/cobra"
)

// A verification check prints one command's sample with a line describing
// what should appear, so a printed sheet can be compared item by item.
type verifyCheck struct {
	cmd    string // command(s) under test
	expect string // what correct output looks like
	build  func(b *escpos.Builder) error
}

func newVerifyCommand(opts *options) *cobra.Command {
	var list, cuts bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Print a numbered checklist that exercises each command",
		Long: `Print a checklist sheet: each numbered item names the command under test and
the expected result, followed by the sample. Compare the paper against the
expectations to see which commands the printer supports.

--cuts appends one item per cut command. Each one cuts the paper, so it
produces several separate pieces.

Not covered, because they have lasting side effects or need extra hardware:
FS q / FS p (NV images, writes flash), GS ( A (test print, resets the
printer), DC2 T (self-test page), ESC p / DLE DC4 (cash drawer), GS FF
(black-mark paper), ESC c 5 (panel buttons) and the FS Kanji commands
(Chinese firmware only; see "demo kanji").`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks := verifyChecks
			if cuts {
				checks = append(slices.Clone(checks), cutChecks...)
			}
			if list {
				printChecks(cmd.OutOrStdout(), checks)
				return nil
			}
			job, err := buildVerifyJob(opts, checks)
			if err != nil {
				return err
			}
			return sendJob(cmd.Context(), opts, job, false)
		},
	}
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list the checks without printing")
	cmd.Flags().BoolVar(&cuts, "cuts", false, "also test every cut command (cuts the paper several times)")
	return cmd
}

func printChecks(w io.Writer, checks []verifyCheck) {
	for i, c := range checks {
		fmt.Fprintf(w, "%2d  %-22s %s\n", i+1, c.cmd, c.expect)
	}
}

func buildVerifyJob(opts *options, checks []verifyCheck) (*escpos.Builder, error) {
	b := escpos.NewBuilder(opts.paperWidth)
	b.Initialize()
	sectionTitle(b, "verify")
	b.Textln("Compare each sample with its 'expect' line.")
	for i, c := range checks {
		b.LineFeed()
		b.SetEmphasis(true)
		b.Textf("%d. %s\n", i+1, c.cmd)
		b.SetEmphasis(false)
		_ = b.SelectFont(escpos.FontB)
		for _, line := range wrap("expect: "+c.expect, b.CharactersPerLine()) {
			b.Textln(line)
		}
		_ = b.SelectFont(escpos.FontA)
		if err := c.build(b); err != nil {
			return nil, fmt.Errorf("check %d (%s): %w", i+1, c.cmd, err)
		}
		b.Initialize()
	}
	b.LineFeed()
	b.Textln("End of checklist.")
	b.FeedAndCut(0)
	return b, nil
}

var verifyChecks = []verifyCheck{
	{"LF, CR", "'AAA' and 'BBB' on one line ('AAABBB') if auto line feed is off, or on two lines if it is on", func(b *escpos.Builder) error {
		b.Text("AAA")
		b.CarriageReturn()
		b.Textln("BBB")
		return nil
	}},
	{"ESC M", "three lines: Font A, then narrower Font B, then Font C (undocumented; may match A or B)", func(b *escpos.Builder) error {
		var c check
		for _, f := range []struct {
			font escpos.Font
			name string
		}{{escpos.FontA, "A"}, {escpos.FontB, "B"}, {escpos.FontC, "C"}} {
			c.do(b.SelectFont(f.font))
			b.Textln("Font " + f.name + ": The quick brown fox")
		}
		return c.err
	}},
	{"ESC !", "one line in Font B, bold, double width and underlined", func(b *escpos.Builder) error {
		b.SelectPrintMode(escpos.PrintModeFontB | escpos.PrintModeEmphasized | escpos.PrintModeDoubleWidth | escpos.PrintModeUnderline)
		b.Textln("Combined mode")
		return nil
	}},
	{"ESC E, ESC G", "'bold' and 'double-strike' both darker than 'normal'", func(b *escpos.Builder) error {
		b.Text("normal ")
		b.SetEmphasis(true)
		b.Text("bold")
		b.SetEmphasis(false)
		b.Text(" ")
		b.SetDoubleStrike(true)
		b.Textln("double-strike")
		return nil
	}},
	{"ESC -", "a thin underline, then a thicker one", func(b *escpos.Builder) error {
		var c check
		c.do(b.SetUnderline(escpos.UnderlineSingle))
		b.Textln("1-dot underline")
		c.do(b.SetUnderline(escpos.UnderlineDouble))
		b.Textln("2-dot underline")
		return c.err
	}},
	{"GS !", "'2x1' twice as wide, '1x2' twice as tall, '3x3' triple, 'W' 6x6", func(b *escpos.Builder) error {
		var c check
		for _, s := range [][2]uint8{{2, 1}, {1, 2}, {3, 3}} {
			c.do(b.SetCharacterSize(s[0], s[1]))
			b.Textf("%dx%d ", s[0], s[1])
		}
		c.do(b.SetCharacterSize(6, 6))
		b.Textln("W")
		return c.err
	}},
	{"GS B", "white text on a black band", func(b *escpos.Builder) error {
		b.SetReverse(true)
		b.Textln(" REVERSE ")
		return nil
	}},
	{"ESC V", "'ROTATED' printed sideways, turned 90 degrees clockwise", func(b *escpos.Builder) error {
		b.SetRotate90(true)
		b.Textln("ROTATED")
		return nil
	}},
	{"ESC {", "'UPSIDE DOWN' rotated 180 degrees", func(b *escpos.Builder) error {
		b.SetUpsideDown(true)
		b.Textln("UPSIDE DOWN")
		return nil
	}},
	{"ESC SP", "second line's letters spaced visibly wider", func(b *escpos.Builder) error {
		b.Textln("SPACING")
		b.SetCharacterSpacing(12)
		b.Textln("SPACING")
		return nil
	}},
	{"ESC 3, ESC 2", "two tightly packed lines, two widely spaced lines, then two at normal spacing", func(b *escpos.Builder) error {
		b.SetLineSpacing(16)
		b.Textln("tight")
		b.Textln("tight")
		b.SetLineSpacing(80)
		b.Textln("wide")
		b.Textln("wide")
		b.DefaultLineSpacing()
		b.Textln("default")
		b.Textln("default")
		return nil
	}},
	{"ESC a", "'left', 'centre' and 'right' at those positions", func(b *escpos.Builder) error {
		var c check
		for _, a := range []struct {
			align escpos.Align
			text  string
		}{{escpos.AlignLeft, "left"}, {escpos.AlignCenter, "centre"}, {escpos.AlignRight, "right"}} {
			c.do(b.SetAlign(a.align))
			b.Textln(a.text)
		}
		return c.err
	}},
	{"GS L", "text indented about 12 mm from the left edge", func(b *escpos.Builder) error {
		b.SetLeftMargin(96)
		b.Textln("indented by GS L")
		return nil
	}},
	{"GS W", "text wrapped into a column about 25 mm wide", func(b *escpos.Builder) error {
		b.SetPrintAreaWidth(200)
		b.Textln("narrow print area wraps this sentence into short lines")
		return nil
	}},
	{"ESC $, ESC \\", "'|' at the left edge, 'A' about 30 mm in, 'B' about 6 mm after A", func(b *escpos.Builder) error {
		b.Text("|")
		b.SetAbsolutePosition(240)
		b.Text("A")
		b.SetRelativePosition(48)
		b.Textln("B")
		return nil
	}},
	{"ESC D, HT", "'1', '2' and '3' starting at columns 1, 11 and 21", func(b *escpos.Builder) error {
		if err := b.SetTabPositions(10, 20); err != nil {
			return err
		}
		b.Textln("1\t2\t3")
		b.Textln("123456789012345678901234")
		return nil
	}},
	{"ESC J, ESC d", "a gap of about 10 mm, then a gap of 3 blank lines", func(b *escpos.Builder) error {
		b.Text("before ESC J")
		b.FeedUnits(80)
		b.Text("before ESC d")
		b.FeedLines(3)
		b.Textln("after")
		return nil
	}},
	{"GS P", "a gap of about 12.7 mm (0.5 inch): ESC J 50 with the vertical unit set to 1/100 inch", func(b *escpos.Builder) error {
		b.SetMotionUnits(0, 100)
		b.Text("before")
		b.FeedUnits(50)
		b.SetMotionUnits(0, 0)
		b.Textln("after")
		return nil
	}},
	{"ESC t", "'cafe' with an accented e, a euro sign, and a box-drawing line", func(b *escpos.Builder) error {
		b.Textln("CP437: café ─────")
		b.SelectCodePage(escpos.CodePageWPC1252)
		b.Textln("WPC1252: €5")
		return nil
	}},
	{"ESC R", "a pound sign (£) where '#' was sent", func(b *escpos.Builder) error {
		if err := b.SelectInternationalCharset(escpos.CharsetUK); err != nil {
			return err
		}
		b.Textln("UK set: #")
		return nil
	}},
	{"ESC &, ESC %, ESC ?", "a smiley after 'custom:', then a normal '~' after 'cancelled:'", func(b *escpos.Builder) error {
		_, _, data := escpos.NewBitmap(smileyGlyph(), escpos.ImageOptions{}).Columns()
		if err := b.DefineUserCharacters('~', escpos.Glyph{Width: 12, Data: data[:36]}); err != nil {
			return err
		}
		b.SelectUserDefinedCharset(true)
		b.Textln("custom: ~")
		if err := b.CancelUserCharacter('~'); err != nil {
			return err
		}
		b.Textln("cancelled: ~")
		return nil
	}},
	{"ESC *", "a sine wave band (24 dots tall)", func(b *escpos.Builder) error {
		_, _, band := escpos.NewBitmap(waveImage(384, 24), escpos.ImageOptions{}).Columns()
		if err := b.PrintBitImage(escpos.BitImage24DotDouble, band); err != nil {
			return err
		}
		b.LineFeed()
		return nil
	}},
	{"GS *, GS /", "the small logo twice: normal size, then double width and height", func(b *escpos.Builder) error {
		x, y, data := escpos.NewBitmap(logoImage(64, 48), escpos.ImageOptions{}).Columns()
		var c check
		c.do(b.DefineDownloadedBitImage(uint8(x), uint8(y), data))
		c.do(b.PrintDownloadedBitImage(escpos.ScaleNormal))
		c.do(b.PrintDownloadedBitImage(escpos.ScaleQuadruple))
		return c.err
	}},
	{"GS v 0", "a framed logo with a ring and three bars", func(b *escpos.Builder) error {
		return b.PrintImage(logoImage(288, 96), escpos.ImageOptions{})
	}},
	{"GS k (NUL form)", "an EAN-8 bar code with digits below", func(b *escpos.Builder) error {
		var c check
		c.do(b.SetHRIPosition(escpos.HRIBelow))
		c.do(b.SetBarcodeHeight(50))
		c.do(b.PrintBarcode(escpos.BarcodeEAN8NUL, "9638507"))
		return c.err
	}},
	{"GS k, GS H, GS f, GS h, GS w", "a short CODE128 bar code with small digits above and below", func(b *escpos.Builder) error {
		var c check
		c.do(b.SetHRIPosition(escpos.HRIBoth))
		c.do(b.SetHRIFont(escpos.FontB))
		c.do(b.SetBarcodeHeight(40))
		c.do(b.SetBarcodeWidth(2))
		c.do(b.PrintBarcode(escpos.BarcodeCode128, escpos.Code128("12345678")))
		return c.err
	}},
	{"GS x", "the same EAN-8 bar code with digits, shifted about 25 mm to the right", func(b *escpos.Builder) error {
		var c check
		c.do(b.SetHRIPosition(escpos.HRIBelow)) // ESC @ between checks turns HRI off
		c.do(b.SetBarcodeHeight(50))
		b.SetBarcodeLeftSpace(200)
		c.do(b.PrintBarcode(escpos.BarcodeEAN8, "9638507"))
		return c.err
	}},
	{"GS ( k", "a square QR code", func(b *escpos.Builder) error {
		return b.PrintQRCode("https://github.com/connordoman/escpos", escpos.QRErrorM, 5)
	}},
	{"GS Z 0, ESC Z", "a wide PDF417 bar code", func(b *escpos.Builder) error {
		return b.PrintPDF417("RP326 PDF417", 3, 2, 3)
	}},
	{"ESC L, ESC W, ESC T, GS $, ESC FF, FF", "a block printed in page mode: 'top left', 'at (200, 90)' lower and to the right, and 'sideways' running bottom to top", func(b *escpos.Builder) error {
		var c check
		b.EnterPageMode()
		c.do(b.SetPageArea(0, 0, 576, 180))
		c.do(b.SetPageDirection(escpos.PageLeftToRight))
		b.Textln("top left")
		b.SetAbsoluteVerticalPosition(90)
		b.SetAbsolutePosition(200)
		b.Textln("at (200, 90)")
		c.do(b.SetPageDirection(escpos.PageBottomToTop))
		b.Textln("sideways")
		b.PrintPageAndExit()
		return c.err
	}},
	{"GS \\ (page mode)", "'second' printed about 20 mm below 'first', leaving a clear gap", func(b *escpos.Builder) error {
		var c check
		b.EnterPageMode()
		c.do(b.SetPageArea(0, 0, 576, 240))
		b.Text("first")
		b.SetRelativeVerticalPosition(160)
		b.SetAbsolutePosition(0)
		b.Text("second")
		b.PrintPageAndExit()
		return c.err
	}},
	{"ESC S", "only 'kept' prints; 'discarded' was buffered in page mode and thrown away", func(b *escpos.Builder) error {
		b.EnterPageMode()
		b.Text("discarded")
		b.EnterStandardMode()
		b.Textln("kept")
		return nil
	}},
	{"GS C 0/1/2, GS c", "'00007', '00008', '00009' on three lines", func(b *escpos.Builder) error {
		if err := b.SetCounterPrintMode(5, escpos.CounterAlignRightZeros); err != nil {
			return err
		}
		b.SetCounterModeA(1, 65535, 1, 1)
		b.SetCounter(7)
		for range 3 {
			b.PrintCounter()
			b.LineFeed()
		}
		return nil
	}},
	{"GS C ;", "'50', '60' on two lines (value 50, step 10) per the reference; the RP326 prints '60', '70'", func(b *escpos.Builder) error {
		if err := b.SetCounterPrintMode(0, 0); err != nil {
			return err
		}
		b.SetCounterModeB(escpos.CounterModeB{A: new(uint16(1)), B: new(uint16(999)), Step: new(uint8(10)), Repeat: new(uint8(1)), Value: new(uint8(50))})
		for range 2 {
			b.PrintCounter()
			b.LineFeed()
		}
		return nil
	}},
	{"GS :, GS ^", "'macro run' printed 3 times", func(b *escpos.Builder) error {
		if err := b.DefineMacro(func(m *escpos.Builder) error { m.Textln("macro run"); return nil }); err != nil {
			return err
		}
		b.ExecuteMacro(3, 1, false)
		return nil
	}},
	{"ESC =", "only 'enabled again' prints; 'MUST NOT PRINT' was sent while disabled", func(b *escpos.Builder) error {
		b.SetPeripheralDevice(false)
		b.Textln("MUST NOT PRINT")
		b.SetPeripheralDevice(true)
		b.Textln("enabled again")
		return nil
	}},
	{"ESC B (beep)", "a short beep", func(b *escpos.Builder) error {
		if err := b.Beep(1, 2); err != nil {
			return err
		}
		b.Textln("beep sent")
		return nil
	}},
}

// cutChecks each end in a cut, so every one produces a separate piece.
var cutChecks = []verifyCheck{
	{"GS V 66 n", "feeds 10 mm past the cut position, then a partial cut below this item", func(b *escpos.Builder) error {
		b.FeedAndCut(80)
		return nil
	}},
	{"GS V 65 n (not in reference)", "feeds and makes a full cut, or a partial one if full cuts are unsupported", func(b *escpos.Builder) error {
		b.FeedAndFullCut(0)
		return nil
	}},
	{"GS V 1", "partial cut a few lines below this item", func(b *escpos.Builder) error {
		b.FeedLines(5)
		return b.Cut(escpos.CutPartial)
	}},
	{"GS V 0 (not in reference)", "full cut a few lines below this item", func(b *escpos.Builder) error {
		b.FeedLines(5)
		return b.Cut(escpos.CutFull)
	}},
	{"ESC i", "a cut a few lines below this item (reference: partial)", func(b *escpos.Builder) error {
		b.FeedLines(5)
		b.CutImmediate()
		return nil
	}},
	{"ESC m", "partial cut a few lines below this item", func(b *escpos.Builder) error {
		b.FeedLines(5)
		b.PartialCutImmediate()
		return nil
	}},
}
