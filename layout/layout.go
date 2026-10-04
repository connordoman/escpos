// Package layout prints styled, word-wrapped text with an [escpos.Builder]:
// paragraphs with hanging indents, receipt lines with dot leaders, tables,
// columns, boxes and rules.
//
// Text is measured in printer dots using each character's font and width
// multiplier, and characters are counted as the active code page prints them
// (see [escpos.CodePage.Printable]), so lines fill the paper exactly. A
// [Writer] remembers the style it last set and only sends the commands
// needed to change it.
//
// Measuring assumes single-byte code pages; it does not account for a text
// encoder set with [escpos.Builder.SetTextEncoder].
package layout

import (
	"strings"
	"unicode"

	"github.com/connordoman/escpos"
)

// Style is a combination of character attributes.
type Style struct {
	Bold         bool
	Underline    uint8 // 0, 1 or 2 dots
	DoubleStrike bool
	Invert       bool // white on black
	FontB        bool
	Width        uint8 // multiplier 1–8; 0 means 1
	Height       uint8 // multiplier 1–8; 0 means 1
	UpsideDown   bool
}

func (s Style) width() uint8  { return min(max(s.Width, 1), 8) }
func (s Style) height() uint8 { return min(max(s.Height, 1), 8) }

func (s Style) font() escpos.Font {
	if s.FontB {
		return escpos.FontB
	}
	return escpos.FontA
}

// CharWidth is the width of one character in dots.
func (s Style) CharWidth() int { return s.font().Width() * int(s.width()) }

// CharHeight is the height of one character in dots.
func (s Style) CharHeight() int {
	h := 24
	if s.FontB {
		h = 17
	}
	return h * int(s.height())
}

// BarLineSpacing returns the line spacing, in dots, at which box-drawing
// characters such as │ in style s join the lines above and below. At the
// default spacing (ESC 2) vertical bars print as dashes.
func BarLineSpacing(s Style) uint8 { return uint8(min(s.CharHeight(), 255)) }

// Span is text in one style.
type Span struct {
	Text  string
	Style Style
}

// Join returns the text of spans without their styles.
func Join(spans []Span) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

// Writer prints styled text with a Builder.
type Writer struct {
	B *escpos.Builder

	// Width is the printable width in dots. It starts as the builder's
	// paper width; narrow it after setting margins.
	Width int

	cur   Style
	align escpos.Align
}

// New returns a writer for b. It assumes the printer is in its default
// state, as after [escpos.Builder.Initialize].
func New(b *escpos.Builder) *Writer {
	return &Writer{B: b, Width: b.PaperWidth()}
}

// Reset records that the printer was returned to its default state, for
// example by [escpos.Builder.Initialize].
func (w *Writer) Reset() {
	w.cur = Style{}
	w.align = escpos.AlignLeft
}

// Style returns the style currently set on the printer.
func (w *Writer) Style() Style { return w.cur }

// Apply switches the printer to style s, sending only what changed.
func (w *Writer) Apply(s Style) {
	c := w.cur
	b := w.B
	if s.FontB != c.FontB {
		b.SelectFont(s.font())
	}
	if s.Bold != c.Bold {
		b.SetEmphasis(s.Bold)
	}
	if s.Underline != c.Underline {
		b.SetUnderline(escpos.Underline(min(s.Underline, 2)))
	}
	if s.DoubleStrike != c.DoubleStrike {
		b.SetDoubleStrike(s.DoubleStrike)
	}
	if s.Invert != c.Invert {
		b.SetReverse(s.Invert)
	}
	if s.width() != c.width() || s.height() != c.height() {
		b.SetCharacterSize(s.width(), s.height())
	}
	if s.UpsideDown != c.UpsideDown {
		b.SetUpsideDown(s.UpsideDown)
	}
	w.cur = s
}

// SetAlign sets the justification of following lines.
func (w *Writer) SetAlign(a escpos.Align) {
	if a != w.align {
		w.B.SetAlign(a)
		w.align = a
	}
}

// Align returns the current justification.
func (w *Writer) Align() escpos.Align { return w.align }

// Printable returns s as the active code page prints it.
func (w *Writer) Printable(s string) string {
	out, _ := w.B.ActiveCodePage().Printable(s)
	return out
}

// Measure returns the number of character cells s occupies when printed.
func (w *Writer) Measure(s string) int { return len([]rune(w.Printable(s))) }

// atom is one printed character.
type atom struct {
	r     rune
	style Style
}

func (w *Writer) atoms(spans []Span) []atom {
	var out []atom
	for _, sp := range spans {
		for _, r := range w.Printable(sp.Text) {
			if r == '\r' {
				continue
			}
			if r == '\t' {
				r = ' '
			}
			out = append(out, atom{r, sp.Style})
		}
	}
	return out
}

func atomsWidth(a []atom) int {
	n := 0
	for _, x := range a {
		n += x.style.CharWidth()
	}
	return n
}

// Paragraph prints spans word-wrapped to the writer's width. Newlines in
// the text start new lines. first is printed before the first line and rest
// before each following line, for bullets, numbering and quote bars.
func (w *Writer) Paragraph(spans []Span, first, rest []Span) {
	all := w.atoms(spans)
	pf, pr := w.atoms(first), w.atoms(rest)
	start := 0
	for i := 0; i <= len(all); i++ {
		if i < len(all) && all[i].r != '\n' {
			continue
		}
		for _, line := range wrapAtoms(all[start:i], w.Width-atomsWidth(pf), w.Width-atomsWidth(pr)) {
			w.printAtoms(append(append([]atom{}, pf...), line...), true)
			pf = pr
		}
		start = i + 1
	}
}

// Text prints s in style st, word-wrapped.
func (w *Writer) Text(s string, st Style) {
	w.Paragraph([]Span{{s, st}}, nil, nil)
}

// Line prints spans on one line without wrapping; the printer wraps text
// that is too long.
func (w *Writer) Line(spans ...Span) {
	w.printAtoms(w.atoms(spans), true)
}

// printAtoms prints one line, optionally dropping trailing spaces (which
// would show when underlined or inverted).
func (w *Writer) printAtoms(line []atom, trim bool) {
	for trim && len(line) > 0 && line[len(line)-1].r == ' ' {
		line = line[:len(line)-1]
	}
	var run strings.Builder
	var st Style
	flush := func() {
		if run.Len() > 0 {
			w.Apply(st)
			w.B.Text(run.String())
			run.Reset()
		}
	}
	for i, a := range line {
		if i == 0 || a.style != st {
			flush()
			st = a.style
		}
		run.WriteRune(a.r)
	}
	flush()
	w.B.LineFeed()
}

// wrapAtoms breaks a line into lines no wider than firstWidth dots for the
// first line and restWidth after that. It breaks after spaces, hyphens and
// slashes where it can and inside words too long for a line.
func wrapAtoms(line []atom, firstWidth, restWidth int) [][]atom {
	if len(line) == 0 {
		return [][]atom{nil}
	}
	var lines [][]atom
	limit := firstWidth
	var cur []atom
	curW := 0
	lastBreak := -1 // index in cur just after the last break opportunity
	for _, a := range line {
		aw := a.style.CharWidth()
		if curW+aw > limit && len(cur) > 0 {
			if a.r == ' ' {
				lines = append(lines, cur)
				cur, curW, lastBreak, limit = nil, 0, -1, restWidth
				continue
			}
			if lastBreak > 0 {
				lines = append(lines, cur[:lastBreak])
				cur = append([]atom{}, cur[lastBreak:]...)
			} else {
				lines = append(lines, cur)
				cur = nil
			}
			curW, lastBreak, limit = atomsWidth(cur), -1, restWidth
		}
		if a.r == ' ' && len(cur) == 0 && len(lines) > 0 {
			continue // no leading spaces on wrapped lines
		}
		cur = append(cur, a)
		curW += aw
		if unicode.IsSpace(a.r) || a.r == '-' || a.r == '/' {
			lastBreak = len(cur)
		}
	}
	return append(lines, cur)
}

// Rule prints a line of r across the full width. '─' gives a solid line
// with the default code page.
func (w *Writer) Rule(r rune, st Style) {
	n := w.Width / st.CharWidth()
	w.Line(Span{strings.Repeat(string(r), max(n, 1)), st})
}

// KeyValue prints key on the left and value flush right, with the gap
// filled by leader (' ' for none, '.' for dot leaders), like a receipt line
// item. A key too long to share the line wraps, with the value on its last
// line.
func (w *Writer) KeyValue(key, value string, leader rune, keyStyle, valueStyle Style) {
	k, v := w.atoms([]Span{{key, keyStyle}}), w.atoms([]Span{{value, valueStyle}})
	vw := atomsWidth(v)
	cw := keyStyle.CharWidth()
	if vw+cw > w.Width {
		w.Paragraph([]Span{{key, keyStyle}}, nil, nil)
		a := w.align
		w.SetAlign(escpos.AlignRight)
		w.printAtoms(v, true)
		w.SetAlign(a)
		return
	}
	lines := wrapAtoms(k, w.Width-vw-cw, w.Width-vw-cw)
	for i, l := range lines {
		if i < len(lines)-1 {
			w.printAtoms(l, true)
			continue
		}
		gap := (w.Width - atomsWidth(l) - vw) / cw
		fill := make([]atom, 0, gap)
		for j := range gap {
			r := leader
			if j == 0 || j == gap-1 {
				r = ' '
			}
			fill = append(fill, atom{r, keyStyle})
		}
		w.printAtoms(append(append(append([]atom{}, l...), fill...), v...), false)
	}
}

// Box prints text word-wrapped inside a frame of box-drawing characters,
// single or double. Line spacing is tightened while the box prints so its
// sides are solid, then restored to the default.
func (w *Writer) Box(text string, st Style, align escpos.Align, double bool) {
	tl, tr, bl, br, h, v := "┌", "┐", "└", "┘", "─", "│"
	if double {
		tl, tr, bl, br, h, v = "╔", "╗", "╚", "╝", "═", "║"
	}
	cols := max(w.Width/st.CharWidth(), 5)
	inner := cols - 4
	frame := st
	frame.Underline, frame.Invert = 0, false
	saved := w.align
	w.SetAlign(escpos.AlignLeft)
	w.B.SetLineSpacing(BarLineSpacing(st))
	w.Line(Span{tl + strings.Repeat(h, cols-2) + tr, frame})
	for _, line := range WrapCells(text, inner, w.Measure) {
		w.printAtoms(w.atoms([]Span{
			{v + " ", frame}, {Pad(line, inner, align, w.Measure), st}, {" " + v, frame},
		}), false)
	}
	w.Line(Span{bl + strings.Repeat(h, cols-2) + br, frame})
	w.B.DefaultLineSpacing()
	w.SetAlign(saved)
}
