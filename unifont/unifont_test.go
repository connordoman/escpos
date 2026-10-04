package unifont

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connordoman/escpos"
	"github.com/connordoman/escpos/escpostest"
)

func ink(img *image.Gray) int {
	n := 0
	for _, p := range img.Pix {
		if p < 128 {
			n++
		}
	}
	return n
}

func render(t *testing.T, text string, o Options) *image.Gray {
	t.Helper()
	img, err := Render(text, escpos.PaperWidth80mm, o)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestRender(t *testing.T) {
	text := "Hello, 世界! 😀👍🏽🎉🍕★♥ café naïve é\nこんにちは、プリンターです。日本語の折り返しを確認するための長い文章です。\nΣωκράτης Привет ✓ → ┌─┐"
	img := render(t, text, Options{})
	if img.Bounds().Dx() != escpos.PaperWidth80mm || img.Bounds().Dy() < 3*32 {
		t.Errorf("bounds %v", img.Bounds())
	}
	if dir := os.Getenv("RENDER_OUT"); dir != "" {
		f, _ := os.Create(filepath.Join(dir, "unifont.png"))
		png.Encode(f, img)
		f.Close()
	}
}

func TestCells(t *testing.T) {
	for s, want := range map[string]int{"abc": 3, "日本": 4, "😀": 2, "é": 1, "a‍b": 2} {
		if got := Cells(s); got != want {
			t.Errorf("Cells(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestLineHeightAndWrap(t *testing.T) {
	zero := 0
	one := render(t, "x", Options{Scale: 2, LineGap: &zero})
	if h := one.Bounds().Dy(); h != 32 {
		t.Errorf("one line at scale 2: %d dots, want 32", h)
	}
	// 576 dots at scale 2 is 36 cells: 37 characters wrap onto a second line.
	two := render(t, strings.Repeat("x", 37), Options{Scale: 2, LineGap: &zero})
	if h := two.Bounds().Dy(); h != 64 {
		t.Errorf("37 cells at scale 2: %d dots, want 64", h)
	}
	cut := render(t, strings.Repeat("x", 37), Options{Scale: 2, LineGap: &zero, NoWrap: true})
	if h := cut.Bounds().Dy(); h != 32 {
		t.Errorf("NoWrap: %d dots, want 32", h)
	}
	// Japanese: 。 must not start a line.
	jp := render(t, strings.Repeat("あ", 36)+"。", Options{Scale: 1, LineGap: &zero})
	if h := jp.Bounds().Dy(); h != 32 {
		t.Errorf("kinsoku: %d lines", h/16)
	}
}

func TestWeights(t *testing.T) {
	zero, two := 0, 2
	light := ink(render(t, "Thank you", Options{Scale: 1.5, Weight: &zero}))
	normal := ink(render(t, "Thank you", Options{Scale: 1.5}))
	heavy := ink(render(t, "Thank you", Options{Scale: 1.5, Weight: &two}))
	if !(light < normal && normal < heavy) {
		t.Errorf("ink by weight 0/default/2: %d %d %d", light, normal, heavy)
	}
	shaded := ink(render(t, "😀🎉★", Options{Scale: 1.5}))
	solid := ink(render(t, "😀🎉★", Options{Scale: 1.5, SolidEmoji: true}))
	if shaded >= solid {
		t.Errorf("shaded emoji (%d) not lighter than solid (%d)", shaded, solid)
	}
	inv := render(t, "x", Options{Invert: true})
	if inv.Pix[0] != 0 {
		t.Error("inverted background is not black")
	}
}

func TestPrint(t *testing.T) {
	b := escpos.NewBuilder(escpos.PaperWidth80mm)
	if err := Print(b, "😀 ok", Options{}); err != nil {
		t.Fatal(err)
	}
	cmds := escpostest.Commands(b.Bytes(), "GS v 0")
	if len(cmds) != 1 || cmds[0].Args[1] != 72 {
		t.Errorf("got %s", escpostest.Describe(b.Bytes()))
	}
}

func TestControlCharactersRemoved(t *testing.T) {
	a := render(t, "ab", Options{})
	b := render(t, "a\x1b\x00b", Options{})
	if ink(a) != ink(b) {
		t.Error("control characters were drawn")
	}
}

func TestSuffix(t *testing.T) {
	zero := 0
	// 36 cells at scale 2 with a 2-cell suffix leaves 34 for text.
	img := render(t, strings.Repeat("x", 35), Options{Scale: 2, LineGap: &zero, Suffix: " │"})
	if h := img.Bounds().Dy(); h != 64 {
		t.Errorf("text did not wrap short of the suffix: %d lines", h/32)
	}
	// The bar is in the last cell on every line: column 568–575.
	for y := 0; y < 64; y++ {
		inked := false
		for x := 560; x < 576; x++ {
			inked = inked || img.GrayAt(x, y).Y < 128
		}
		if y%32 > 2 && y%32 < 30 && !inked {
			t.Fatalf("no bar at row %d", y)
		}
	}
}

func TestScaleXY(t *testing.T) {
	zero := 0
	img := render(t, strings.Repeat("x", 48), Options{ScaleX: 1.5, ScaleY: 2, LineGap: &zero})
	if h := img.Bounds().Dy(); h != 32 {
		t.Errorf("48 cells at ScaleX 1.5 should fit one 32-dot line, got %d dots", h)
	}
	img = render(t, strings.Repeat("x", 37), Options{Scale: 2, LineGap: &zero})
	if h := img.Bounds().Dy(); h != 64 {
		t.Errorf("37 cells at scale 2 should wrap, got %d dots", h)
	}
}

func TestEdgeScale(t *testing.T) {
	zero := 0
	// A bar prefix at 1.5 occupies 24 dots ("│ "), then text at 2.
	img := render(t, "x", Options{Scale: 2, EdgeScaleX: 1.5, FirstPrefix: "│ ", Suffix: " │", LineGap: &zero})
	inked := func(x0, x1 int) bool {
		for y := 4; y < 28; y++ {
			for x := x0; x < x1; x++ {
				if img.GrayAt(x, y).Y < 128 {
					return true
				}
			}
		}
		return false
	}
	if !inked(0, 12) || inked(12, 24) {
		t.Error("prefix bar not in the first 12-dot cell")
	}
	if !inked(24, 40) {
		t.Error("text does not start after the 24-dot prefix")
	}
	if !inked(564, 576) || inked(552, 564) {
		t.Error("suffix bar not in the last 12-dot cell")
	}
}
