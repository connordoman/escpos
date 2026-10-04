package layout

import (
	"slices"
	"strings"

	"github.com/connordoman/escpos"
)

// Table is a grid of plain-text cells. It fills the line, giving each
// column a share in proportion to its widest content; when the content is
// wider than the line, narrow columns keep their width and cells in the
// wide ones wrap.
type Table struct {
	Headers []string       // optional
	Aligns  []escpos.Align // per column, default left
	Rows    [][]string
	Border  bool // separate columns with │ instead of a space
	// Weights, if set, divides the width among columns in proportion
	// instead of sizing columns to their content.
	Weights []int
	// Compact sizes columns to their content instead of filling the line.
	Compact bool

	HeaderStyle Style
	Style       Style
}

func (t Table) columns() int {
	n := len(t.Headers)
	for _, r := range t.Rows {
		n = max(n, len(r))
	}
	return n
}

// Layout arranges the table into lines of at most width character cells,
// where measure gives the number of cells a string occupies. It is useful
// for drawing tables some other way, such as into an image.
func (t Table) Layout(width int, measure func(string) int) [][]Span {
	n := t.columns()
	if n == 0 {
		return nil
	}
	gap := 1
	sep := " "
	if t.Border {
		gap, sep = 3, " │ "
	}
	avail := max(width-gap*(n-1), n)

	natural := make([]int, n)
	consider := func(row []string) {
		for i, c := range row {
			natural[i] = max(natural[i], measure(c))
		}
	}
	consider(t.Headers)
	for _, r := range t.Rows {
		consider(r)
	}
	widths := fitColumns(natural, avail, !t.Compact)
	if len(t.Weights) == n {
		widths = weighColumns(t.Weights, avail)
	}

	var out [][]Span
	addRow := func(row []string, st Style) {
		wrapped := make([][]string, n)
		height := 1
		for i := range n {
			c := ""
			if i < len(row) {
				c = row[i]
			}
			wrapped[i] = WrapCells(c, widths[i], measure)
			height = max(height, len(wrapped[i]))
		}
		for l := range height {
			var line []Span
			for i := range n {
				if i > 0 {
					line = append(line, Span{sep, t.Style})
				}
				c := ""
				if l < len(wrapped[i]) {
					c = wrapped[i][l]
				}
				a := escpos.AlignLeft
				if i < len(t.Aligns) {
					a = t.Aligns[i]
				}
				line = append(line, Span{Pad(c, widths[i], a, measure), st})
			}
			out = append(out, line)
		}
	}
	if len(t.Headers) > 0 {
		addRow(t.Headers, t.HeaderStyle)
		var rule strings.Builder
		for i, w := range widths {
			if i > 0 {
				if t.Border {
					rule.WriteString("─┼─")
				} else {
					rule.WriteString(" ")
				}
			}
			rule.WriteString(strings.Repeat("─", w))
		}
		out = append(out, []Span{{rule.String(), t.Style}})
	}
	for _, r := range t.Rows {
		addRow(r, t.Style)
	}
	return out
}

// fitColumns fits natural column widths to avail. Content that is too
// wide shrinks: narrow columns keep what they need and wide ones share the
// rest. Content that is narrower grows to fill avail when fill is set, each
// column getting extra width in proportion to its natural width.
func fitColumns(natural []int, avail int, fill bool) []int {
	widths := make([]int, len(natural))
	total := 0
	for i, w := range natural {
		widths[i] = max(w, 1)
		total += widths[i]
	}
	if total <= avail {
		if fill && total < avail {
			grow(widths, total, avail-total)
		}
		return widths
	}
	fixed := make([]bool, len(natural))
	remaining, open := avail, len(natural)
	for changed := true; changed && open > 0; {
		changed = false
		share := remaining / open
		for i := range widths {
			if !fixed[i] && widths[i] <= share {
				fixed[i] = true
				remaining -= widths[i]
				open--
				changed = true
			}
		}
	}
	if open > 0 {
		share, extra := remaining/open, remaining%open
		for i := range widths {
			if !fixed[i] {
				widths[i] = max(share, 1)
				if extra > 0 {
					widths[i]++
					extra--
				}
			}
		}
	}
	return widths
}

// grow shares extra among widths in proportion to their size, giving any
// remainder to the widest columns first.
func grow(widths []int, total, extra int) {
	given := 0
	for i, w := range widths {
		add := extra * w / total
		widths[i] += add
		given += add
	}
	order := make([]int, len(widths))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return widths[b] - widths[a] })
	for k := 0; given < extra; k++ {
		widths[order[k%len(order)]]++
		given++
	}
}

func weighColumns(weights []int, avail int) []int {
	total := 0
	for _, w := range weights {
		total += max(w, 1)
	}
	widths := make([]int, len(weights))
	used := 0
	for i, w := range weights {
		widths[i] = max(avail*max(w, 1)/total, 1)
		used += widths[i]
	}
	for i := 0; used < avail; i = (i + 1) % len(widths) {
		widths[i]++
		used++
	}
	return widths
}

// WrapCells word-wraps s into lines of at most width cells, where measure
// gives the number of cells a string occupies. Words longer than a line are
// split.
func WrapCells(s string, width int, measure func(string) int) []string {
	var lines []string
	for para := range strings.SplitSeq(s, "\n") {
		var cur string
		for _, word := range strings.Fields(para) {
			for measure(word) > width {
				// Split a word that cannot fit on any line.
				if cur != "" {
					lines = append(lines, cur)
					cur = ""
				}
				head, tail := splitCells(word, width, measure)
				lines = append(lines, head)
				word = tail
			}
			switch {
			case cur == "":
				cur = word
			case measure(cur)+1+measure(word) <= width:
				cur += " " + word
			default:
				lines = append(lines, cur)
				cur = word
			}
		}
		lines = append(lines, cur)
	}
	return lines
}

func splitCells(s string, width int, measure func(string) int) (string, string) {
	n := 0
	for i, r := range s {
		n += measure(string(r))
		if n > width {
			if i == 0 {
				// Always make progress, even if one character is too wide.
				size := firstRuneLen(s)
				return s[:size], s[size:]
			}
			return s[:i], s[i:]
		}
	}
	return s, ""
}

// firstRuneLen returns the length in bytes of the first rune of s.
func firstRuneLen(s string) int {
	for i := range s {
		if i > 0 {
			return i
		}
	}
	return len(s)
}

// Pad pads s with spaces to width cells, aligned as a says.
func Pad(s string, width int, a escpos.Align, measure func(string) int) string {
	gap := max(width-measure(s), 0)
	switch a {
	case escpos.AlignRight:
		return strings.Repeat(" ", gap) + s
	case escpos.AlignCenter:
		return strings.Repeat(" ", gap/2) + s + strings.Repeat(" ", gap-gap/2)
	}
	return s + strings.Repeat(" ", gap)
}

// Table prints t. Bordered tables tighten line spacing so the column
// separators are solid, then restore the default.
func (w *Writer) Table(t Table) {
	cells := w.Width / t.Style.CharWidth()
	if t.Border {
		w.B.SetLineSpacing(BarLineSpacing(t.Style))
		defer w.B.DefaultLineSpacing()
	}
	for _, line := range t.Layout(cells, w.Measure) {
		w.printAtoms(w.atoms(line), true)
	}
}

// Column is one column of [Writer.Columns].
type Column struct {
	Text   string
	Align  escpos.Align
	Weight int // relative width; 0 means 1
}

// Columns prints text side by side, each column taking a share of the line
// in proportion to its weight and wrapping independently.
func (w *Writer) Columns(st Style, border bool, cols ...Column) {
	t := Table{Border: border, Style: st, Rows: [][]string{nil}}
	for _, c := range cols {
		t.Rows[0] = append(t.Rows[0], c.Text)
		t.Aligns = append(t.Aligns, c.Align)
		t.Weights = append(t.Weights, max(c.Weight, 1))
	}
	w.Table(t)
}
