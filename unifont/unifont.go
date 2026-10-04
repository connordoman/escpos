// Package unifont prints any Unicode text, including emoji, CJK, Cyrillic,
// Greek, symbols and box drawing, by drawing it as an image with GNU
// Unifont, a bitmap font covering every assigned code point with 8×16 and
// 16×16 glyphs. Its pixel lettering suits a thermal print head, and it
// works on printers whose own fonts are limited to one code page.
//
//	err := unifont.Print(b, "Thank you! ありがとう 🙏", unifont.Options{Scale: 1.5})
//
// Lines wrap at spaces, and between CJK characters following Japanese
// line-breaking rules. Arabic and Hebrew are drawn left to right without
// joining, and emoji are monochrome without skin-tone or ZWJ combinations.
//
// The font adds about 1.7 MB to programs that import this package, and
// nothing to those that do not. It is distributed under the SIL Open Font
// License; see the NOTICE file.
package unifont

import (
	"image"
	"image/color"
	"math"
	"strings"
	"unicode"

	"github.com/connordoman/escpos"
	"golang.org/x/text/unicode/norm"
)

// Options controls how text is drawn.
type Options struct {
	// Scale is the number of dots per font pixel, from 1 to 8. Unifont is
	// 16 pixels tall, so 1.5 matches a printer's 12×24 Font A and 2 (the
	// default) is a little larger.
	Scale float64
	Bold  bool
	// Weight is the number of extra dots added to the right of every
	// pixel of ordinary (non-emoji) glyphs, thickening their strokes. nil
	// means automatic: 1 below scale 3, where Unifont's 1-pixel strokes
	// would otherwise print fainter than the printer's own fonts.
	Weight *int
	// SolidEmoji draws emoji solid black. By default their solid areas are
	// shaded with a 50% checkerboard, keeping outlines intact, since
	// Unifont's emoji are dense and otherwise print much heavier than text.
	SolidEmoji bool
	// Invert prints white text on black.
	Invert bool
	Align  escpos.Align
	// LineGap is the space between lines in dots; nil means one font pixel.
	// Set it to 0 to make box-drawing characters such as │ join up.
	LineGap *int
	// NoWrap disables word wrapping; long lines are cut off.
	NoWrap bool
	// FirstPrefix and RestPrefix are drawn before the first and following
	// lines of each paragraph, for bullets and quote bars, like
	// layout.Writer.Paragraph.
	FirstPrefix, RestPrefix string
}

func (o Options) scale() float64 {
	if o.Scale <= 0 {
		return 2
	}
	return min(max(o.Scale, 1), 8)
}

// glyphCell is one base character and the combining marks drawn over it.
type glyphCell struct {
	r     rune
	marks []rune
	adv   int // font pixels
}

func cellsOf(f *Font, s string) []glyphCell {
	var out []glyphCell
	for _, r := range s {
		if r == '\t' {
			r = ' '
		}
		if Invisible(r) {
			continue
		}
		if _, ok := f.Combining(r); ok {
			if len(out) > 0 {
				out[len(out)-1].marks = append(out[len(out)-1].marks, r)
			}
			continue
		}
		g, ok := f.Glyph(r)
		if !ok {
			r = '�'
			g, _ = f.Glyph(r)
		}
		out = append(out, glyphCell{r: r, adv: g.Width})
	}
	return out
}

func cellsWidth(c []glyphCell) int {
	n := 0
	for _, x := range c {
		n += x.adv
	}
	return n
}

// Characters that should not start or end a line (Japanese kinsoku rules,
// which also suit Chinese and Korean).
const (
	noBreakBefore = "、。，．,.:;!?！？：；)]}）］｝〕〉》」』】〙〗〟’”ーぁぃぅぇぉっゃゅょゎァィゥェォッャュョヮヵヶ・々〻ゝゞヽヾ…‥"
	noBreakAfter  = "([{（［｛〔〈《「『【〘〖〝‘“"
)

// breakable reports whether a line may break after c. Spaces and hyphens
// allow breaks, and so does any wide (CJK, emoji) character, since those
// scripts do not separate words with spaces.
func breakable(c, next glyphCell) bool {
	if strings.ContainsRune(noBreakBefore, next.r) || strings.ContainsRune(noBreakAfter, c.r) {
		return false
	}
	return unicode.IsSpace(c.r) || c.r == '-' || c.r == '/' || c.adv > 8 || next.adv > 8
}

func wrapGlyphs(line []glyphCell, firstWidth, restWidth int, wrap bool) [][]glyphCell {
	if !wrap || len(line) == 0 {
		return [][]glyphCell{line}
	}
	var lines [][]glyphCell
	limit := firstWidth
	var cur []glyphCell
	curW, lastBreak := 0, -1
	for i, c := range line {
		if curW+c.adv > limit && len(cur) > 0 {
			if unicode.IsSpace(c.r) {
				lines = append(lines, cur)
				cur, curW, lastBreak, limit = nil, 0, -1, restWidth
				continue
			}
			if lastBreak > 0 {
				lines = append(lines, cur[:lastBreak])
				cur = append([]glyphCell{}, cur[lastBreak:]...)
			} else {
				lines = append(lines, cur)
				cur = nil
			}
			curW, lastBreak, limit = cellsWidth(cur), -1, restWidth
		}
		if unicode.IsSpace(c.r) && len(cur) == 0 && len(lines) > 0 {
			continue
		}
		cur = append(cur, c)
		curW += c.adv
		if i+1 < len(line) && breakable(c, line[i+1]) {
			lastBreak = len(cur)
		}
	}
	return append(lines, cur)
}

func trimTrailingSpace(c []glyphCell) []glyphCell {
	for len(c) > 0 && unicode.IsSpace(c[len(c)-1].r) {
		c = c[:len(c)-1]
	}
	return c
}

// normalize composes accents (NFC), turns CR LF and CR into LF and removes
// control characters other than LF and HT.
func normalize(s string) string {
	s = norm.NFC.String(strings.ToValidUTF8(s, "\uFFFD"))
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\r':
			return '\n'
		case r == '\n' || r == '\t' || !unicode.IsControl(r):
			return r
		}
		return -1
	}, s)
}

// Render draws text as a black and white image width dots wide, as tall as
// the text needs.
func Render(text string, width int, o Options) (*image.Gray, error) {
	f, err := Load()
	if err != nil {
		return nil, err
	}
	s := o.scale()
	avail := int(float64(width) / s) // in font pixels
	first, rest := cellsOf(f, o.FirstPrefix), cellsOf(f, o.RestPrefix)

	var lines [][]glyphCell
	for para := range strings.SplitSeq(normalize(text), "\n") {
		wrapped := wrapGlyphs(cellsOf(f, para), avail-cellsWidth(first), avail-cellsWidth(rest), !o.NoWrap)
		for i, l := range wrapped {
			prefix := rest
			if i == 0 {
				prefix = first
			}
			lines = append(lines, append(append([]glyphCell{}, prefix...), trimTrailingSpace(l)...))
		}
	}

	gap := int(math.Round(s))
	if o.LineGap != nil {
		gap = max(*o.LineGap, 0)
	}
	lineH := int(math.Round(Height*s)) + gap
	img := image.NewGray(image.Rect(0, 0, width, max(lineH*len(lines), 1)))
	bg, fg := color.Gray{Y: 255}, color.Gray{Y: 0}
	if o.Invert {
		bg, fg = fg, bg
	}
	for i := range img.Pix {
		img.Pix[i] = bg.Y
	}
	extra := 0
	if s < 3 {
		extra = 1
	}
	if o.Weight != nil {
		extra = max(*o.Weight, 0)
	}
	if o.Bold {
		extra += max(1, int(s/2))
	}
	for li, line := range lines {
		lw := int(math.Round(float64(cellsWidth(line)) * s))
		x0 := 0
		switch o.Align {
		case escpos.AlignCenter:
			x0 = (width - lw) / 2
		case escpos.AlignRight:
			x0 = width - lw
		}
		y0 := li*lineH + gap/2
		pen := 0
		for _, c := range line {
			g, _ := f.Glyph(c.r)
			ex, thin := extra, false
			if isEmoji(c.r) {
				ex, thin = 0, !o.SolidEmoji
				if o.Bold {
					ex = max(1, int(s/2))
				}
			}
			drawGlyph(img, g, x0, y0, pen, s, ex, thin, fg)
			for _, m := range c.marks {
				mg, ok := f.Glyph(m)
				if !ok {
					continue
				}
				off, _ := f.Combining(m)
				drawGlyph(img, mg, x0, y0, pen+c.adv+off, s, extra, false, fg)
			}
			pen += c.adv
		}
	}
	return img, nil
}

// isEmoji reports whether r is in one of the emoji and pictograph blocks.
func isEmoji(r rune) bool {
	return r >= 0x1F000 && r <= 0x1FAFF || r >= 0x2600 && r <= 0x27BF || r >= 0x2B00 && r <= 0x2BFF
}

// drawGlyph draws g at font-pixel column pen of a line starting at (x0, y0),
// scaling each font pixel to a block of dots. extra widens each block to
// the right. thin shades solid interiors with a 50% checkerboard, keeping
// outlines intact, so dense glyphs print grey instead of black.
func drawGlyph(img *image.Gray, g Glyph, x0, y0, pen int, s float64, extra int, thin bool, fg color.Gray) {
	b := img.Bounds()
	for gy := range Height {
		ya, yb := y0+int(float64(gy)*s), y0+int(float64(gy+1)*s)
		for gx := range g.Width {
			if !g.Set(gx, gy) {
				continue
			}
			if thin && (gx+gy)%2 == 1 && g.Set(gx-1, gy) && g.Set(gx+1, gy) && g.Set(gx, gy-1) && g.Set(gx, gy+1) {
				continue
			}
			xa := x0 + int(float64(pen+gx)*s)
			xb := x0 + int(float64(pen+gx+1)*s) + extra
			for y := max(ya, b.Min.Y); y < min(yb, b.Max.Y); y++ {
				row := img.Pix[y*img.Stride:]
				for x := max(xa, b.Min.X); x < min(xb, b.Max.X); x++ {
					row[x] = fg.Y
				}
			}
		}
	}
}

// Cells returns how many 8-pixel font cells s occupies: 1 for most
// characters and 2 for wide ones such as CJK and emoji. At scale 1.5 a
// cell is 12 dots, the width of a Font A character, which makes Cells a
// suitable measure for layout.Table.Layout.
func Cells(s string) int {
	f, err := Load()
	if err != nil {
		return len([]rune(s))
	}
	return cellsWidth(cellsOf(f, s)) / 8
}

// Print draws text the width of b's paper and prints it as a raster image.
func Print(b *escpos.Builder, text string, o Options) error {
	img, err := Render(text, b.PaperWidth(), o)
	if err != nil {
		return err
	}
	return b.PrintImage(img, escpos.ImageOptions{MaxWidth: b.PaperWidth()})
}
