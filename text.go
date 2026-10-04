package escpos

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/unicode/norm"
)

// CodePage is a character code table selectable with ESC t.
type CodePage byte

// Character code tables, from the ESC t table. Values 11–14 are reserved.
const (
	CodePagePC437      CodePage = 0  // CP437 [U.S.A., Standard Europe]
	CodePageKatakana   CodePage = 1  // Katakana
	CodePagePC850      CodePage = 2  // CP850 [Multilingual]
	CodePagePC860      CodePage = 3  // CP860 [Portuguese]
	CodePagePC863      CodePage = 4  // CP863 [Canadian-French]
	CodePagePC865      CodePage = 5  // CP865 [Nordic]
	CodePageWPC1251    CodePage = 6  // WCP1251 [Cyrillic]
	CodePagePC866      CodePage = 7  // CP866 Cyrillic #2
	CodePageMIK        CodePage = 8  // MIK [Cyrillic/Bulgarian]
	CodePageCP755      CodePage = 9  // CP755 [East Europe, Latvian 2]
	CodePageIran       CodePage = 10 // Iran
	CodePagePC862      CodePage = 15 // CP862 [Hebrew]
	CodePageWPC1252    CodePage = 16 // WCP1252 Latin I
	CodePageWPC1253    CodePage = 17 // WCP1253 [Greek]
	CodePagePC852      CodePage = 18 // CP852 [Latin 2]
	CodePagePC858      CodePage = 19 // CP858 Multilingual Latin I + Euro
	CodePageIranII     CodePage = 20 // Iran II
	CodePageLatvian    CodePage = 21 // Latvian
	CodePagePC864      CodePage = 22 // CP864 [Arabic]
	CodePageISO8859_1  CodePage = 23 // ISO-8859-1 [West Europe]
	CodePagePC737      CodePage = 24 // CP737 [Greek]
	CodePageWPC1257    CodePage = 25 // WCP1257 [Baltic]
	CodePageThai       CodePage = 26 // Thai
	CodePagePC720      CodePage = 27 // CP720 [Arabic]
	CodePagePC855      CodePage = 28 // CP855
	CodePagePC857      CodePage = 29 // CP857 [Turkish]
	CodePageWPC1250    CodePage = 30 // WCP1250 [Central Europe]
	CodePagePC775      CodePage = 31 // CP775
	CodePageWPC1254    CodePage = 32 // WCP1254 [Turkish]
	CodePageWPC1255    CodePage = 33 // WCP1255 [Hebrew]
	CodePageWPC1256    CodePage = 34 // WCP1256 [Arabic]
	CodePageWPC1258    CodePage = 35 // WCP1258 [Vietnam]
	CodePageISO8859_2  CodePage = 36 // ISO-8859-2 [Latin 2]
	CodePageISO8859_3  CodePage = 37 // ISO-8859-3 [Latin 3]
	CodePageISO8859_4  CodePage = 38 // ISO-8859-4 [Baltic]
	CodePageISO8859_5  CodePage = 39 // ISO-8859-5 [Cyrillic]
	CodePageISO8859_6  CodePage = 40 // ISO-8859-6 [Arabic]
	CodePageISO8859_7  CodePage = 41 // ISO-8859-7 [Greek]
	CodePageISO8859_8  CodePage = 42 // ISO-8859-8 [Hebrew]
	CodePageISO8859_9  CodePage = 43 // ISO-8859-9 [Turkish]
	CodePageISO8859_15 CodePage = 44 // ISO-8859-15
	CodePageThai2      CodePage = 45 // Thai2
	CodePagePC856      CodePage = 46 // CP856
	CodePagePC874      CodePage = 47 // CP874
)

// charmaps maps code pages to the standard tables used to encode text for
// them. Code pages without an entry are encoded as ASCII only.
var charmaps = map[CodePage]*charmap.Charmap{
	CodePagePC437:      charmap.CodePage437,
	CodePagePC850:      charmap.CodePage850,
	CodePagePC860:      charmap.CodePage860,
	CodePagePC863:      charmap.CodePage863,
	CodePagePC865:      charmap.CodePage865,
	CodePageWPC1251:    charmap.Windows1251,
	CodePagePC866:      charmap.CodePage866,
	CodePagePC862:      charmap.CodePage862,
	CodePageWPC1252:    charmap.Windows1252,
	CodePageWPC1253:    charmap.Windows1253,
	CodePagePC852:      charmap.CodePage852,
	CodePagePC858:      charmap.CodePage858,
	CodePageISO8859_1:  charmap.ISO8859_1,
	CodePageWPC1257:    charmap.Windows1257,
	CodePagePC855:      charmap.CodePage855,
	CodePageWPC1250:    charmap.Windows1250,
	CodePageWPC1254:    charmap.Windows1254,
	CodePageWPC1255:    charmap.Windows1255,
	CodePageWPC1256:    charmap.Windows1256,
	CodePageWPC1258:    charmap.Windows1258,
	CodePageISO8859_2:  charmap.ISO8859_2,
	CodePageISO8859_3:  charmap.ISO8859_3,
	CodePageISO8859_4:  charmap.ISO8859_4,
	CodePageISO8859_5:  charmap.ISO8859_5,
	CodePageISO8859_6:  charmap.ISO8859_6,
	CodePageISO8859_7:  charmap.ISO8859_7,
	CodePageISO8859_8:  charmap.ISO8859_8,
	CodePageISO8859_9:  charmap.ISO8859_9,
	CodePageISO8859_15: charmap.ISO8859_15,
	CodePagePC874:      charmap.Windows874,
}

// transliterations replaces characters missing from a code page with ASCII,
// for those that Unicode decomposition does not reduce to ASCII.
var transliterations = map[rune]string{
	'‐': "-", '‑': "-", '‒': "-", '–': "-", '—': "-", '―': "-", '−': "-",
	'‘': "'", '’': "'", '‚': ",", '‛': "'", '′': "'", '‹': "<", '›': ">",
	'“': "\"", '”': "\"", '„': "\"", '‟': "\"", '″': "\"", '«': "<<", '»': ">>",
	'•': "*", '·': ".", '‣': ">", '◦': "o", '…': "...", '⁄': "/",
	'×': "x", '÷': "/", '±': "+/-", '°': "deg", '©': "(c)", '®': "(R)", '™': "TM",
	'€': "EUR", '£': "GBP", '¥': "JPY", '¢': "c", '¡': "!", '¿': "?", '§': "S",
	'¶': "P", '†': "+", '‡': "++", '→': "->", '←': "<-", '↑': "^", '↓': "v",
	'⇒': "=>", '⇐': "<=", '≤': "<=", '≥': ">=", '≠': "!=", '≈': "~", '∞': "inf",
	'ß': "ss", 'æ': "ae", 'Æ': "AE", 'œ': "oe", 'Œ': "OE", 'ø': "o", 'Ø': "O",
	'ł': "l", 'Ł': "L", 'đ': "d", 'Đ': "D", 'ð': "d", 'Ð': "D", 'þ': "th", 'Þ': "Th",
	'ı': "i", 'ħ': "h", 'Ħ': "H", 'ŧ': "t", 'Ŧ': "T", 'ŋ': "ng", 'Ŋ': "NG",
	'✓': "v", '✔': "v", '✗': "x", '✘': "x", '☐': "[ ]", '☑': "[x]", '☒': "[x]",
	'─': "-", '━': "-", '│': "|", '┃': "|", '═': "=", '║': "|",
}

// transliterate returns an ASCII rendering of r, or "" if there is none.
func transliterate(r rune) string {
	if r < utf8.RuneSelf {
		return string(r)
	}
	if t, ok := transliterations[r]; ok {
		return t
	}
	if unicode.IsSpace(r) {
		return " "
	}
	var out strings.Builder
	for _, d := range norm.NFKD.String(string(r)) {
		switch {
		case d < utf8.RuneSelf:
			out.WriteRune(d)
		case unicode.Is(unicode.Mn, d):
		default:
			if t, ok := transliterations[d]; ok {
				out.WriteString(t)
			} else {
				return ""
			}
		}
	}
	return out.String()
}

// zeroWidth reports whether r prints nothing on its own: combining marks
// left over after NFC normalisation and format characters such as joiners.
func zeroWidth(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf)
}

// sanitize normalises s to NFC and removes control characters other than
// LF, CR and HT, so text can never inject commands such as ESC or GS.
func sanitize(s string) string {
	s = norm.NFC.String(strings.ToValidUTF8(s, "\uFFFD"))
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, s)
}

// Transliterate converts s to printable ASCII, rewriting characters where it
// can ("Café — naïve" becomes "Cafe - naive") and using '?' otherwise.
// Control characters other than LF, CR and HT are removed.
func Transliterate(s string) string {
	var b strings.Builder
	for _, r := range sanitize(s) {
		if zeroWidth(r) {
			continue
		}
		if t := transliterate(r); t != "" {
			b.WriteString(t)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

// Encodes reports whether code page cp has a character for r. ASCII is
// always encodable; beyond it, only code pages whose layout the package
// knows encode anything.
func (cp CodePage) Encodes(r rune) bool {
	if r < utf8.RuneSelf {
		return true
	}
	if cm := charmaps[cp]; cm != nil {
		_, ok := cm.EncodeRune(r)
		return ok
	}
	return false
}

// Decode returns the character byte c stands for in code page cp. Bytes
// above 0x7F in code pages whose layout the package does not know decode
// as U+FFFD.
func (cp CodePage) Decode(c byte) rune {
	if c < utf8.RuneSelf {
		return rune(c)
	}
	if cm := charmaps[cp]; cm != nil {
		return cm.DecodeByte(c)
	}
	return utf8.RuneError
}

// Printable returns s as [Builder.Text] prints it with code page cp: runes
// the code page has are kept, others are transliterated to ASCII or replaced
// with '?', and control characters are removed. complete is false if any
// rune had to be replaced with '?', meaning the printer's fonts cannot show
// s even approximately (emoji or CJK in a Latin code page, for example).
//
// Printable is useful for measuring text before printing it, since every
// returned rune occupies one character cell.
func (cp CodePage) Printable(s string) (printed string, complete bool) {
	var b strings.Builder
	complete = true
	for _, r := range sanitize(s) {
		switch {
		case zeroWidth(r):
		case cp.Encodes(r) && (r >= 0x20 || r == '\n' || r == '\r' || r == '\t'):
			b.WriteRune(r)
		default:
			if t := transliterate(r); t != "" {
				b.WriteString(t)
			} else {
				b.WriteByte('?')
				complete = false
			}
		}
	}
	return b.String(), complete
}

// SelectCodePage selects a character code table (ESC t n). Subsequent
// [Builder.Text] calls encode runes for this table where the package knows
// its layout; otherwise only ASCII is printed faithfully.
func (b *Builder) SelectCodePage(cp CodePage) {
	b.cmd(ESC, 't', byte(cp))
	b.codePage = cp
}

// SetTextEncoder overrides how [Builder.Text] encodes strings, for example
// with simplifiedchinese.GBK.NewEncoder() after [Builder.SelectKanjiMode].
// Passing nil restores code-page based encoding. Nothing is sent to the
// printer.
func (b *Builder) SetTextEncoder(e *encoding.Encoder) { b.encoder = e }

// ActiveCodePage returns the code page selected with [Builder.SelectCodePage]
// (or reset by [Builder.Initialize]).
func (b *Builder) ActiveCodePage() CodePage { return b.codePage }

// Text appends s encoded for the active code page. Runes the code page cannot
// represent are replaced with a close ASCII equivalent where one exists, or
// '?' otherwise; see [CodePage.Printable].
//
// Control characters other than LF, CR and HT are removed, so text from
// users can never smuggle printer commands (ESC, GS, ...) into a job. Use
// [Builder.Raw] to send bytes deliberately.
func (b *Builder) Text(s string) {
	s = sanitize(s)
	if b.encoder != nil {
		if out, err := b.encoder.String(s); err == nil {
			b.buf = append(b.buf, out...)
			return
		}
	}
	cm := charmaps[b.codePage]
	for _, r := range s {
		if r < utf8.RuneSelf {
			b.buf = append(b.buf, byte(r))
			continue
		}
		if cm != nil {
			if c, ok := cm.EncodeRune(r); ok {
				b.buf = append(b.buf, c)
				continue
			}
		}
		if zeroWidth(r) {
			continue
		}
		if alt := transliterate(r); alt != "" {
			b.buf = append(b.buf, alt...)
			continue
		}
		b.buf = append(b.buf, '?')
	}
}

// Textln appends s like [Builder.Text] followed by LF, which prints the line.
func (b *Builder) Textln(s string) {
	b.Text(s)
	b.cmd(LF)
}

// Textf formats according to a format specifier and appends the result like
// [Builder.Text].
func (b *Builder) Textf(format string, args ...any) {
	b.Text(fmt.Sprintf(format, args...))
}
