package css

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/connordoman/escpos"
	"github.com/connordoman/escpos/layout"
)

func resolve(t *testing.T, p Properties) Computed {
	t.Helper()
	c, err := p.Resolve(Computed{})
	if err != nil {
		t.Fatalf("%+v: %v", p, err)
	}
	return c
}

func TestResolve(t *testing.T) {
	for _, tc := range []struct {
		p    Properties
		want layout.Style
	}{
		{Properties{FontWeight: "bold"}, layout.Style{Bold: true}},
		{Properties{FontWeight: "700"}, layout.Style{Bold: true}},
		{Properties{FontWeight: "900"}, layout.Style{Bold: true, DoubleStrike: true}},
		{Properties{FontSize: "48px"}, layout.Style{Width: 2, Height: 2}},
		{Properties{FontSize: "48"}, layout.Style{Width: 2, Height: 2}}, // React: number = px
		{Properties{FontSize: "3em"}, layout.Style{Width: 3, Height: 3}},
		{Properties{FontSize: "500px"}, layout.Style{Width: 8, Height: 8}},
		{Properties{FontSize: "48px", Transform: "scaleX(0.5)"}, layout.Style{Width: 1, Height: 2}},
		{Properties{Transform: "scale(2, 3)"}, layout.Style{Width: 2, Height: 3}},
		{Properties{Transform: "rotate(180deg)"}, layout.Style{Width: 1, Height: 1, UpsideDown: true}},
		{Properties{Transform: "rotate(90deg)"}, layout.Style{Width: 1, Height: 1, Rotate90: true}},
		{Properties{FontFamily: `"Font B", monospace`}, layout.Style{FontB: true}},
		{Properties{TextDecoration: "underline"}, layout.Style{Underline: 1}},
		{Properties{TextDecoration: "underline double"}, layout.Style{Underline: 2}},
		{Properties{TextDecoration: "underline", TextDecorationThickness: "2px"}, layout.Style{Underline: 2}},
		{Properties{BackgroundColor: "black", Color: "white"}, layout.Style{Invert: true}},
		{Properties{LetterSpacing: "3px"}, layout.Style{Spacing: 3}},
	} {
		got := resolve(t, tc.p).Style
		// Width and height are 1 unless set; normalise for comparison.
		if tc.want.Width == 0 && got.Width == 1 && got.Height == 1 && tc.p.Transform == "" {
			tc.want.Width, tc.want.Height = got.Width, got.Height
		}
		if got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.p, got, tc.want)
		}
	}
}

func TestBlockProperties(t *testing.T) {
	c := resolve(t, Properties{
		TextAlign: "center", LineHeight: "1.5", TextTransform: "uppercase",
		WhiteSpace: "nowrap", MarginLeft: "96", Width: "384px",
	})
	if c.Align != escpos.AlignCenter || c.LineHeight != 36 || !c.NoWrap || c.MarginLeft != 96 || c.Width != 384 {
		t.Errorf("got %+v", c)
	}
	if c.Text("hello world") != "HELLO WORLD" {
		t.Error("uppercase not applied")
	}
	if (Computed{TextTransform: "capitalize"}).Text("hello big world") != "Hello Big World" {
		t.Error("capitalize")
	}
	if c := resolve(t, Properties{LineHeight: "40px"}); c.LineHeight != 40 {
		t.Errorf("lineHeight 40px: %d", c.LineHeight)
	}
}

func TestInheritance(t *testing.T) {
	parent := resolve(t, Properties{FontWeight: "bold", TextAlign: "right", MarginLeft: "50"})
	child, err := (&Properties{TextDecoration: "underline"}).Resolve(parent)
	if err != nil {
		t.Fatal(err)
	}
	if !child.Style.Bold || child.Style.Underline != 1 || child.Align != escpos.AlignRight {
		t.Errorf("inherited properties lost: %+v", child)
	}
	if child.MarginLeft != 0 {
		t.Error("marginLeft is not inherited in CSS")
	}
	big := resolve(t, Properties{FontSize: "48px"})
	rel, _ := (&Properties{FontSize: "0.5em"}).Resolve(big)
	if rel.Size != 1 {
		t.Errorf("0.5em of double size: %d", rel.Size)
	}
}

func TestErrors(t *testing.T) {
	_, err := (&Properties{
		Color: "red", TextDecoration: "line-through", Transform: "skew(10deg)",
		FontFamily: "Comic Sans", FontWeight: "bold",
	}).Resolve(Computed{})
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v", err)
	}
	for _, prop := range []string{"color", "textDecoration", "transform", "fontFamily"} {
		if !strings.Contains(err.Error(), prop+":") {
			t.Errorf("no error for %s in %v", prop, err)
		}
	}
	// Valid declarations still apply alongside errors.
	c, _ := (&Properties{Color: "red", FontWeight: "bold"}).Resolve(Computed{})
	if !c.Style.Bold {
		t.Error("valid declaration dropped")
	}
	if _, err := (&Properties{Transform: "rotate(45deg)"}).Resolve(Computed{}); err == nil {
		t.Error("45° rotation accepted")
	}
}

func TestJSON(t *testing.T) {
	var p Properties
	if err := json.Unmarshal([]byte(`{"fontSize": 48, "fontWeight": 700, "textAlign": "center"}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.FontSize != "48" || p.FontWeight != "700" {
		t.Errorf("got %+v", p)
	}
	if got := p.String(); got != "font-size: 48px; font-weight: 700; text-align: center" {
		t.Errorf("String() = %q", got)
	}
}

func TestParseDeclarations(t *testing.T) {
	p, ignored := ParseDeclarations("font-weight: bold; textAlign: center !important; display: flex; background-color:#000")
	if p.FontWeight != "bold" || p.TextAlign != "center" || p.BackgroundColor != "#000" {
		t.Errorf("got %+v", p)
	}
	if len(ignored) != 1 || ignored[0] != "display: flex" {
		t.Errorf("ignored %q", ignored)
	}
	if !strings.Contains(string(Stylesheet), `font-family: "Font B"`) {
		t.Error("stylesheet not embedded")
	}
	if len(Names()) != 18 {
		t.Errorf("Names() = %v", Names())
	}
}

func TestBorderStyle(t *testing.T) {
	c := resolve(t, Properties{BorderStyle: "double"})
	if c.Border != "double" {
		t.Errorf("got %q", c.Border)
	}
	child, _ := (&Properties{}).Resolve(c)
	if child.Border != "" {
		t.Error("borderStyle is not inherited in CSS")
	}
	if _, err := (&Properties{BorderStyle: "dotted"}).Resolve(Computed{}); err == nil {
		t.Error("dotted accepted")
	}
}
