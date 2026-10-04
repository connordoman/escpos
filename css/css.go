// Package css styles printed text with a subset of CSS, written like React's
// CSSProperties: camelCase names, string values, and numbers meaning pixels
// where CSS takes a length.
//
//	style := css.Properties{FontWeight: "bold", FontSize: "48px", TextAlign: "center"}
//	c, err := style.Resolve(css.Computed{})
//	w.SetAlign(c.Align)
//	w.Text("TOTAL", c.Style) // with a layout.Writer
//
// Every supported property keeps its CSS meaning, so the same style object
// renders sensibly in a browser on top of the escpos.css stylesheet
// ([Stylesheet]), which reproduces the printer's default state with one CSS
// pixel per printer dot. Font sizes are relative to the printer's base
// height: 24px is normal size and 48px double, in either font.
//
// Supported properties: fontFamily ("Font A", "Font B"), fontSize,
// fontWeight, textDecoration, textDecorationLine, textDecorationStyle,
// textDecorationThickness, textAlign, color, backgroundColor (black on white
// or inverted), lineHeight, letterSpacing, textTransform, whiteSpace,
// transform (scale and right-angle rotation), marginLeft, width and
// borderStyle (for elements drawn with a box, solid or double lines).
// Values a thermal printer cannot reproduce, such as italics or red text,
// are errors from [Properties.Resolve].
package css

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
)

// Stylesheet is escpos.css, which renders HTML the way the printer prints
// it in its default state. Serve it next to previews of print jobs.
//
//go:embed escpos.css
var Stylesheet []byte

// Value is a CSS value: a string, or in JSON a number, which means pixels
// for lengths as in React.
type Value string

// UnmarshalJSON accepts a string or a number.
func (v *Value) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*v = Value(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return &Error{Value: string(b), Reason: "want a string or a number"}
	}
	*v = Value(n)
	return nil
}

// Properties is a set of CSS declarations. Empty fields are unset.
type Properties struct {
	FontFamily              Value `json:"fontFamily,omitempty"`
	FontSize                Value `json:"fontSize,omitempty"`
	FontWeight              Value `json:"fontWeight,omitempty"`
	TextDecoration          Value `json:"textDecoration,omitempty"`
	TextDecorationLine      Value `json:"textDecorationLine,omitempty"`
	TextDecorationStyle     Value `json:"textDecorationStyle,omitempty"`
	TextDecorationThickness Value `json:"textDecorationThickness,omitempty"`
	TextAlign               Value `json:"textAlign,omitempty"`
	Color                   Value `json:"color,omitempty"`
	BackgroundColor         Value `json:"backgroundColor,omitempty"`
	LineHeight              Value `json:"lineHeight,omitempty"`
	LetterSpacing           Value `json:"letterSpacing,omitempty"`
	TextTransform           Value `json:"textTransform,omitempty"`
	WhiteSpace              Value `json:"whiteSpace,omitempty"`
	Transform               Value `json:"transform,omitempty"`
	MarginLeft              Value `json:"marginLeft,omitempty"`
	Width                   Value `json:"width,omitempty"`
	BorderStyle             Value `json:"borderStyle,omitempty"`
}

// field is a property's camelCase name and value.
type field struct {
	name string
	v    *Value
}

func (p *Properties) fields() []field {
	return []field{
		{"fontFamily", &p.FontFamily},
		{"fontSize", &p.FontSize},
		{"fontWeight", &p.FontWeight},
		{"textDecoration", &p.TextDecoration},
		{"textDecorationLine", &p.TextDecorationLine},
		{"textDecorationStyle", &p.TextDecorationStyle},
		{"textDecorationThickness", &p.TextDecorationThickness},
		{"textAlign", &p.TextAlign},
		{"color", &p.Color},
		{"backgroundColor", &p.BackgroundColor},
		{"lineHeight", &p.LineHeight},
		{"letterSpacing", &p.LetterSpacing},
		{"textTransform", &p.TextTransform},
		{"whiteSpace", &p.WhiteSpace},
		{"transform", &p.Transform},
		{"marginLeft", &p.MarginLeft},
		{"width", &p.Width},
		{"borderStyle", &p.BorderStyle},
	}
}

// lengths are the properties whose bare numbers mean pixels.
var lengths = map[string]bool{
	"fontSize": true, "textDecorationThickness": true, "letterSpacing": true,
	"marginLeft": true, "width": true,
}

// Names lists the supported properties in camelCase.
func Names() []string {
	var p Properties
	var out []string
	for _, f := range p.fields() {
		out = append(out, f.name)
	}
	return out
}

func kebab(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.IsUpper(r) {
			b.WriteByte('-')
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func camel(name string) string {
	name = strings.TrimSpace(name)
	if !strings.Contains(name, "-") {
		return name // already camelCase
	}
	var b strings.Builder
	up := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r == '-':
			up = true
		case up:
			b.WriteRune(unicode.ToUpper(r))
			up = false
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// String formats the set properties as CSS declarations, such as
// "font-weight: bold; text-align: center", for a style attribute.
func (p Properties) String() string {
	var parts []string
	for _, f := range p.fields() {
		v := strings.TrimSpace(string(*f.v))
		if v == "" {
			continue
		}
		if lengths[f.name] {
			if _, err := strconv.ParseFloat(v, 64); err == nil && v != "0" {
				v += "px"
			}
		}
		parts = append(parts, kebab(f.name)+": "+v)
	}
	return strings.Join(parts, "; ")
}

// ParseDeclarations parses CSS declarations as found in an HTML style
// attribute, such as "font-weight: bold; text-align: center". Property
// names may be kebab-case or camelCase. Declarations for unsupported
// properties are returned in ignored rather than failing, since HTML often
// carries styles a printer has no use for. Values are checked by
// [Properties.Resolve].
func ParseDeclarations(s string) (p Properties, ignored []string) {
	fields := p.fields()
	for decl := range strings.SplitSeq(s, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			if strings.TrimSpace(decl) != "" {
				ignored = append(ignored, strings.TrimSpace(decl))
			}
			continue
		}
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "!important"))
		c := camel(name)
		found := false
		for _, f := range fields {
			if f.name == c {
				*f.v, found = Value(value), true
				break
			}
		}
		if !found {
			ignored = append(ignored, strings.TrimSpace(decl))
		}
	}
	return p, ignored
}
