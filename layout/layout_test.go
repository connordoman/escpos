package layout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/connordoman/escpos"
	"github.com/connordoman/escpos/escpostest"
)

// lines returns the printed lines without commands.
func lines(b []byte) []string { return escpostest.Lines(b) }

func newWriter() *Writer { return New(escpos.NewBuilder(escpos.PaperWidth80mm)) }

func TestParagraphWraps(t *testing.T) {
	w := newWriter()
	text := strings.Repeat("word ", 30)
	w.Paragraph([]Span{{Text: text}}, []Span{{Text: "- "}}, []Span{{Text: "  "}})
	got := lines(w.B.Bytes())
	if len(got) < 3 || !strings.HasPrefix(got[0], "- word") || !strings.HasPrefix(got[1], "  word") {
		t.Fatalf("got %q", got)
	}
	for _, l := range got {
		if len([]rune(l)) > 48 {
			t.Errorf("line longer than 48 columns: %q", l)
		}
	}
}

func TestDoubleWidthWrapsAtHalf(t *testing.T) {
	w := newWriter()
	w.Text(strings.Repeat("ab ", 20), Style{Width: 2})
	for _, l := range lines(w.B.Bytes()) {
		if len(l) > 24 {
			t.Errorf("double-width line longer than 24 columns: %q", l)
		}
	}
}

func TestLongWordSplits(t *testing.T) {
	w := newWriter()
	w.Text(strings.Repeat("x", 100), Style{})
	got := lines(w.B.Bytes())
	if len(got) != 3 || len(got[0]) != 48 || len(got[2]) != 4 {
		t.Errorf("got %q", got)
	}
}

func TestApplySendsOnlyChanges(t *testing.T) {
	w := newWriter()
	w.Apply(Style{Bold: true})
	w.Apply(Style{Bold: true})
	w.Apply(Style{Bold: true, Underline: 1})
	want := []byte{escpos.ESC, 'E', 1, escpos.ESC, '-', 1}
	if got := w.B.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("got % X, want % X", got, want)
	}
}

func TestKeyValue(t *testing.T) {
	w := newWriter()
	w.KeyValue("Subtotal", "$16.50", '.', Style{}, Style{})
	w.KeyValue(strings.Repeat("long item name ", 5), "$1.00", ' ', Style{}, Style{})
	got := lines(w.B.Bytes())
	if got[0] != "Subtotal ................................ $16.50" {
		t.Errorf("leader line %q", got[0])
	}
	last := got[len(got)-1]
	if len(last) != 48 || !strings.HasSuffix(last, " $1.00") {
		t.Errorf("wrapped key's last line %q", last)
	}
}

func TestTable(t *testing.T) {
	w := newWriter()
	w.Table(Table{
		Headers: []string{"Item", "Qty"},
		Aligns:  []escpos.Align{escpos.AlignLeft, escpos.AlignRight},
		Rows:    [][]string{{"Coffee", "2"}, {strings.Repeat("Bagel ", 12), "10"}},
	})
	got := lines(w.B.Bytes())
	if got[0] != "Item                                         Qty" || !strings.HasPrefix(got[1], "────") {
		t.Errorf("header %q / %q", got[0], got[1])
	}
	for _, l := range got {
		if len([]rune(l)) > 48 {
			t.Errorf("line longer than 48 columns: %q", l)
		}
	}
	if !strings.HasSuffix(got[3], " 10") || len(got) < 5 {
		t.Errorf("wrapped row %q", got[3:])
	}
}

func TestColumnsAndBox(t *testing.T) {
	w := newWriter()
	w.Columns(Style{}, false, Column{Text: "Left", Weight: 2}, Column{Text: "Right", Align: escpos.AlignRight})
	w.Box("Boxed", Style{}, escpos.AlignCenter, false)
	got := lines(w.B.Bytes())
	if !strings.HasPrefix(got[0], "Left") || !strings.HasSuffix(got[0], "Right") || len(got[0]) != 48 {
		t.Errorf("columns %q", got[0])
	}
	if got[1] != "┌"+strings.Repeat("─", 46)+"┐" || !strings.HasPrefix(got[2], "│ ") || len([]rune(got[2])) != 48 {
		t.Errorf("box %q", got[1:])
	}
	if !bytes.Contains(w.B.Bytes(), []byte{escpos.ESC, '3', 24}) || !bytes.HasSuffix(w.B.Bytes(), []byte{'\n', escpos.ESC, '2'}) {
		t.Error("box does not tighten and restore line spacing")
	}
}

func TestMeasureUsesActiveCodePage(t *testing.T) {
	w := newWriter()
	if n := w.Measure("€"); n != 3 { // CP437 prints "EUR"
		t.Errorf("CP437: %d", n)
	}
	w.B.SelectCodePage(escpos.CodePageWPC1252)
	if n := w.Measure("€"); n != 1 {
		t.Errorf("Windows-1252: %d", n)
	}
	if n := w.Measure("a\x1bb"); n != 2 {
		t.Errorf("control characters counted: %d", n)
	}
}

func TestSpacingWidensCharacters(t *testing.T) {
	w := newWriter()
	w.Text(strings.Repeat("ab ", 20), Style{Spacing: 4}) // 16-dot cells: 36 per line
	for _, l := range lines(w.B.Bytes()) {
		if len(l) > 36 {
			t.Errorf("line longer than 36 columns: %q", l)
		}
	}
	if !bytes.Contains(w.B.Bytes(), []byte{escpos.ESC, ' ', 4}) {
		t.Error("no ESC SP 4")
	}
}
