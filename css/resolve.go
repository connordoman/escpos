package css

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/connordoman/escpos"
	"github.com/connordoman/escpos/layout"
)

// BaseFontSize is the CSS font size, in pixels, of characters at their
// normal size. Larger sizes are rounded to whole multiples of it.
const BaseFontSize = 24

// DefaultLineHeight is the printer's default line spacing in dots (ESC 2).
const DefaultLineHeight = 30

// Computed is the printer's view of a style after inheritance.
type Computed struct {
	// Style is the character style for a layout.Writer.
	Style layout.Style

	// Size is the multiplier from fontSize (1–8), before transform scales
	// it into Style.Width and Style.Height.
	Size int

	Align escpos.Align

	// LineHeight is the line spacing in dots; 0 means the printer default
	// (DefaultLineHeight, ESC 2).
	LineHeight int

	// TextTransform is "uppercase", "lowercase", "capitalize" or empty;
	// apply it with Computed.Text.
	TextTransform string

	// NoWrap is set by whiteSpace: nowrap and pre.
	NoWrap bool

	// MarginLeft and Width are the left margin and printable width in dots
	// for the block (GS L, GS W). They are not inherited; 0 Width means the
	// rest of the paper.
	MarginLeft, Width int
}

// Text applies the computed text-transform to s.
func (c Computed) Text(s string) string {
	switch c.TextTransform {
	case "uppercase":
		return strings.ToUpper(s)
	case "lowercase":
		return strings.ToLower(s)
	case "capitalize":
		var b strings.Builder
		start := true
		for _, r := range s {
			if start && unicode.IsLetter(r) {
				r = unicode.ToTitle(r)
			}
			start = unicode.IsSpace(r)
			b.WriteRune(r)
		}
		return b.String()
	}
	return s
}

// Error is an invalid or unsupported property value.
type Error struct {
	Property string // camelCase
	Value    string
	Reason   string
}

func (e *Error) Error() string {
	if e.Property == "" {
		return fmt.Sprintf("css: %q: %s", e.Value, e.Reason)
	}
	return fmt.Sprintf("css: %s: %q: %s", e.Property, e.Value, e.Reason)
}

// Resolve computes the style for an element whose parent's computed style
// is parent (the zero Computed for the root). Inherited properties, which
// are all but marginLeft and width, start from the parent's values, as in
// CSS. The error joins one *Error per bad declaration; valid declarations
// still apply.
func (p *Properties) Resolve(parent Computed) (Computed, error) {
	c := parent
	c.MarginLeft, c.Width = 0, 0
	if c.Size == 0 {
		c.Size = 1
	}
	if p == nil {
		return c, nil
	}
	var errs []error
	fail := func(prop string, v Value, reason string) {
		errs = append(errs, &Error{Property: prop, Value: string(v), Reason: reason})
	}

	// Font size first: line-height and transform depend on it.
	if v := norm(p.FontFamily); v != "" {
		switch font, ok := fontFamily(v); {
		case ok:
			c.Style.FontB = font
		default:
			fail("fontFamily", p.FontFamily, `want "Font A" or "Font B"`)
		}
	}
	scaleX, scaleY := 1.0, 1.0
	if v := norm(p.FontSize); v != "" {
		px, err := length(v, true, float64(c.Size*BaseFontSize))
		switch {
		case v == "medium" || v == "normal":
			c.Size = 1
		case err != nil:
			fail("fontSize", p.FontSize, err.Error())
		default:
			c.Size = clamp(int(math.Round(px/BaseFontSize)), 1, 8)
		}
		c.Style.Width, c.Style.Height = uint8(c.Size), uint8(c.Size)
	}
	if v := norm(p.Transform); v != "" {
		sx, sy, rot, err := transform(v)
		if err != nil {
			fail("transform", p.Transform, err.Error())
		} else {
			scaleX, scaleY = sx, sy
			c.Style.Rotate90 = rot == 90 || rot == 270
			c.Style.UpsideDown = rot == 180 || rot == 270
			c.Style.Width = uint8(clamp(int(math.Round(float64(c.Size)*scaleX)), 1, 8))
			c.Style.Height = uint8(clamp(int(math.Round(float64(c.Size)*scaleY)), 1, 8))
		}
	}

	if v := norm(p.FontWeight); v != "" {
		bold, heavy, err := fontWeight(v, c.Style.Bold)
		if err != nil {
			fail("fontWeight", p.FontWeight, err.Error())
		} else {
			c.Style.Bold, c.Style.DoubleStrike = bold, heavy
		}
	}

	// Underline: the shorthand, then the longhands that refine it.
	if v := norm(p.TextDecoration); v != "" {
		u, err := decoration(v, c.Style.Underline)
		if err != nil {
			fail("textDecoration", p.TextDecoration, err.Error())
		} else {
			c.Style.Underline = u
		}
	}
	if v := norm(p.TextDecorationLine); v != "" {
		switch v {
		case "none":
			c.Style.Underline = 0
		case "underline":
			c.Style.Underline = max(c.Style.Underline, 1)
		default:
			fail("textDecorationLine", p.TextDecorationLine, "only underline and none can be printed")
		}
	}
	if v := norm(p.TextDecorationStyle); v != "" {
		switch v {
		case "solid":
			if c.Style.Underline == 2 {
				c.Style.Underline = 1
			}
		case "double":
			if c.Style.Underline > 0 {
				c.Style.Underline = 2
			}
		default:
			fail("textDecorationStyle", p.TextDecorationStyle, "only solid and double (a 2-dot line) can be printed")
		}
	}
	if v := norm(p.TextDecorationThickness); v != "" && v != "auto" && v != "from-font" {
		px, err := length(v, true, 0)
		switch {
		case err != nil:
			fail("textDecorationThickness", p.TextDecorationThickness, err.Error())
		case c.Style.Underline > 0:
			c.Style.Underline = uint8(clamp(int(math.Round(px)), 1, 2))
		}
	}

	if v := norm(p.TextAlign); v != "" {
		switch v {
		case "left", "start":
			c.Align = escpos.AlignLeft
		case "center":
			c.Align = escpos.AlignCenter
		case "right", "end":
			c.Align = escpos.AlignRight
		default:
			fail("textAlign", p.TextAlign, "want left, center or right")
		}
	}

	if v := norm(p.Color); v != "" {
		if _, err := shade(v); err != nil {
			fail("color", p.Color, err.Error())
		}
	}
	if v := norm(p.BackgroundColor); v != "" {
		dark, err := shade(v)
		if err != nil {
			fail("backgroundColor", p.BackgroundColor, err.Error())
		} else {
			c.Style.Invert = dark
		}
	}

	if v := norm(p.LineHeight); v != "" {
		charHeight := float64(BaseFontSize * int(max(c.Style.Height, 1)))
		switch n, err := strconv.ParseFloat(v, 64); {
		case v == "normal":
			c.LineHeight = 0
		case err == nil: // unitless: a multiple of the font size
			c.LineHeight = clamp(int(math.Round(n*charHeight)), 0, 255)
		default:
			px, err := length(v, false, charHeight)
			if err != nil {
				fail("lineHeight", p.LineHeight, err.Error())
			} else {
				c.LineHeight = clamp(int(math.Round(px)), 0, 255)
			}
		}
	}
	if v := norm(p.LetterSpacing); v != "" {
		if v == "normal" {
			c.Style.Spacing = 0
		} else if px, err := length(v, true, float64(BaseFontSize)); err != nil || px < 0 {
			fail("letterSpacing", p.LetterSpacing, "want a non-negative length such as 2px")
		} else {
			c.Style.Spacing = uint8(clamp(int(math.Round(px)), 0, 255))
		}
	}

	if v := norm(p.TextTransform); v != "" {
		switch v {
		case "none":
			c.TextTransform = ""
		case "uppercase", "lowercase", "capitalize":
			c.TextTransform = v
		default:
			fail("textTransform", p.TextTransform, "want none, uppercase, lowercase or capitalize")
		}
	}
	if v := norm(p.WhiteSpace); v != "" {
		switch v {
		case "normal", "pre-wrap", "pre-line", "break-spaces":
			c.NoWrap = false
		case "nowrap", "pre":
			c.NoWrap = true
		default:
			fail("whiteSpace", p.WhiteSpace, "want normal, nowrap, pre, pre-wrap or pre-line")
		}
	}

	if v := norm(p.MarginLeft); v != "" {
		if px, err := length(v, true, 0); err != nil || px < 0 {
			fail("marginLeft", p.MarginLeft, "want a non-negative length in px")
		} else {
			c.MarginLeft = clamp(int(math.Round(px)), 0, 65535)
		}
	}
	if v := norm(p.Width); v != "" && v != "auto" {
		if px, err := length(v, true, 0); err != nil || px <= 0 {
			fail("width", p.Width, "want a positive length in px, or auto")
		} else {
			c.Width = clamp(int(math.Round(px)), 1, 65535)
		}
	}
	return c, errors.Join(errs...)
}

func norm(v Value) string { return strings.ToLower(strings.TrimSpace(string(v))) }

func clamp(n, lo, hi int) int { return min(max(n, lo), hi) }

// fontFamily picks the first printer font named in a font-family list.
func fontFamily(v string) (fontB, ok bool) {
	for name := range strings.SplitSeq(v, ",") {
		switch strings.Trim(strings.TrimSpace(name), `"'`) {
		case "font a", "a", "monospace":
			return false, true
		case "font b", "b":
			return true, true
		}
	}
	return false, false
}

// length parses a CSS length in pixels. Bare numbers are pixels when
// unitless is set (React's convention); em and % are relative to em.
func length(v string, unitless bool, em float64) (float64, error) {
	num := func(s string) (float64, error) { return strconv.ParseFloat(strings.TrimSpace(s), 64) }
	switch {
	case strings.HasSuffix(v, "px"):
		return num(strings.TrimSuffix(v, "px"))
	case strings.HasSuffix(v, "rem"):
		n, err := num(strings.TrimSuffix(v, "rem"))
		return n * BaseFontSize, err
	case strings.HasSuffix(v, "em"):
		n, err := num(strings.TrimSuffix(v, "em"))
		return n * em, err
	case strings.HasSuffix(v, "%"):
		n, err := num(strings.TrimSuffix(v, "%"))
		return n / 100 * em, err
	case unitless:
		if n, err := num(v); err == nil {
			return n, nil
		}
	}
	if n, err := num(v); err == nil && n == 0 {
		return 0, nil
	}
	return 0, errors.New("want a length in px, em, rem or %")
}

func fontWeight(v string, parentBold bool) (bold, heavy bool, err error) {
	switch v {
	case "normal", "lighter":
		return false, false, nil
	case "bold":
		return true, false, nil
	case "bolder":
		return true, parentBold, nil
	}
	n, perr := strconv.Atoi(v)
	if perr != nil || n < 1 || n > 1000 {
		return false, false, errors.New("want normal, bold, bolder, lighter or 1–1000")
	}
	// 600 and up print bold; 800 and up add double-strike for extra weight.
	return n >= 600, n >= 800, nil
}

// decoration parses the text-decoration shorthand into an underline
// thickness in dots.
func decoration(v string, current uint8) (uint8, error) {
	u := uint8(0)
	line := false
	thickness := uint8(0)
	for tok := range strings.FieldsSeq(v) {
		switch tok {
		case "none":
			line = true
		case "underline":
			line, u = true, 1
		case "solid":
		case "double":
			thickness = 2
		case "auto", "from-font":
		default:
			if px, err := length(tok, false, 0); err == nil {
				thickness = uint8(clamp(int(math.Round(px)), 1, 2))
				continue
			}
			return current, fmt.Errorf("%q cannot be printed; only underline, none, double and a thickness of 1px or 2px", tok)
		}
	}
	if !line {
		return current, errors.New("give underline or none")
	}
	if u > 0 && thickness > 0 {
		u = thickness
	}
	return u, nil
}

// shade reports whether a colour is dark (prints black) or light (paper).
// Thermal printers print one colour, so anything else is an error.
func shade(v string) (dark bool, err error) {
	switch v {
	case "black", "#000", "#000000", "#000f", "#000000ff", "rgb(0,0,0)", "rgb(0, 0, 0)", "currentcolor", "inherit":
		return true, nil
	case "white", "#fff", "#ffffff", "#ffff", "#ffffffff", "rgb(255,255,255)", "rgb(255, 255, 255)", "transparent", "initial":
		return false, nil
	}
	return false, errors.New("thermal printers print black on white; use black, white or transparent")
}

// transform parses scale() and rotate() functions.
func transform(v string) (sx, sy float64, rot int, err error) {
	sx, sy = 1, 1
	if v == "none" {
		return
	}
	rest := v
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		open := strings.IndexByte(rest, '(')
		end := strings.IndexByte(rest, ')')
		if open <= 0 || end < open {
			return 1, 1, 0, errors.New("want scale(), scaleX(), scaleY() or rotate()")
		}
		name, args := rest[:open], strings.Split(rest[open+1:end], ",")
		rest = rest[end+1:]
		nums := make([]float64, len(args))
		for i, a := range args {
			if name == "rotate" {
				continue
			}
			if nums[i], err = strconv.ParseFloat(strings.TrimSpace(a), 64); err != nil || nums[i] <= 0 {
				return 1, 1, 0, fmt.Errorf("%s needs positive numbers", name)
			}
		}
		switch name {
		case "scale":
			sx, sy = nums[0], nums[0]
			if len(nums) > 1 {
				sy = nums[1]
			}
		case "scalex":
			sx = nums[0]
		case "scaley":
			sy = nums[0]
		case "rotate":
			a := strings.TrimSpace(args[0])
			var deg float64
			switch {
			case strings.HasSuffix(a, "deg"):
				deg, err = strconv.ParseFloat(strings.TrimSuffix(a, "deg"), 64)
			case strings.HasSuffix(a, "turn"):
				deg, err = strconv.ParseFloat(strings.TrimSuffix(a, "turn"), 64)
				deg *= 360
			default:
				err = errors.New("bad angle")
			}
			r := int(math.Mod(math.Mod(deg, 360)+360, 360))
			if err != nil || float64(r) != math.Mod(math.Mod(deg, 360)+360, 360) || r%90 != 0 {
				return 1, 1, 0, errors.New("printers rotate by 90deg, 180deg or 270deg only")
			}
			rot = r
		default:
			return 1, 1, 0, fmt.Errorf("%s() cannot be printed; use scale(), scaleX(), scaleY() or rotate()", name)
		}
	}
	return sx, sy, rot, nil
}
