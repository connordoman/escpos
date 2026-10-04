// Package escpostest decodes ESC/POS command streams into a readable form,
// for testing code that builds print jobs:
//
//	b := escpos.NewBuilder(escpos.PaperWidth80mm)
//	b.SetEmphasis(true)
//	b.Textln("TOTAL")
//	escpostest.Describe(b.Bytes()) // "<ESC E 1>TOTAL⏎\n"
//
// It understands every command package escpos emits, plus the real-time
// commands, so a stream decodes without losing its place. Text is decoded
// with the code page the stream selects (ESC t), or as GBK in Kanji mode.
package escpostest

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/connordoman/escpos"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// Token is a run of text or one command.
type Token struct {
	// Text is printable text, decoded, when Cmd is empty.
	Text string

	// Cmd is the command in the reference's notation, such as "ESC E",
	// "GS ( k" or "LF". Unknown commands are "ESC 0xNN" and so on.
	Cmd string

	// Args are the command's numeric parameters, in order.
	Args []int

	// Data is the command's payload: image, bar code or symbol data,
	// glyphs, or the rest of the stream if the command is truncated.
	Data []byte

	// Truncated reports that the stream ended inside the command.
	Truncated bool
}

// IsText reports whether t is text rather than a command.
func (t Token) IsText() bool { return t.Cmd == "" }

// String formats t as in [Describe].
func (t Token) String() string {
	if t.IsText() {
		return t.Text
	}
	if t.Cmd == "LF" {
		return "⏎\n"
	}
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(t.Cmd)
	for _, a := range t.Args {
		b.WriteByte(' ')
		b.WriteString(strconv.Itoa(a))
	}
	switch {
	case t.Cmd == "GS v 0" && len(t.Args) == 3:
		fmt.Fprintf(&b, " (%d×%d dots)", t.Args[1]*8, t.Args[2])
	case t.Cmd == "ESC *" || t.Cmd == "GS *" || t.Cmd == "FS q" || t.Cmd == "ESC &" || t.Cmd == "FS 2":
		if len(t.Data) > 0 {
			fmt.Fprintf(&b, " (%d bytes)", len(t.Data))
		}
	case len(t.Data) > 0:
		if printable(t.Data) {
			fmt.Fprintf(&b, " %q", t.Data)
		} else {
			fmt.Fprintf(&b, " (%d bytes)", len(t.Data))
		}
	}
	if t.Truncated {
		b.WriteString(" …truncated")
	}
	b.WriteByte('>')
	return b.String()
}

func printable(d []byte) bool {
	if !utf8.Valid(d) {
		return false
	}
	for _, c := range string(d) {
		if c < 0x20 && c != '\n' || c == 0x7F {
			return false
		}
	}
	return true
}

// Describe renders a stream as text: printed text as is, line feeds as
// "⏎" plus a newline, and other commands as <CMD args> with any payload
// summarised.
func Describe(b []byte) string {
	var out strings.Builder
	for _, t := range Decode(b) {
		out.WriteString(t.String())
	}
	return out.String()
}

// Lines returns the text printed on each line (ended by LF), without
// commands. Text after the last LF is not included.
func Lines(b []byte) []string {
	var lines []string
	var cur strings.Builder
	for _, t := range Decode(b) {
		switch {
		case t.IsText():
			cur.WriteString(t.Text)
		case t.Cmd == "LF":
			lines = append(lines, cur.String())
			cur.Reset()
		}
	}
	return lines
}

// Commands returns the tokens for command cmd, such as "GS V".
func Commands(b []byte, cmd string) []Token {
	var out []Token
	for _, t := range Decode(b) {
		if t.Cmd == cmd {
			out = append(out, t)
		}
	}
	return out
}

// decoder walks a stream.
type decoder struct {
	b     []byte
	i     int
	out   []Token
	text  []byte
	cp    escpos.CodePage
	kanji bool
}

// Decode splits a stream into text and commands.
func Decode(b []byte) []Token {
	d := &decoder{b: b}
	for d.i < len(d.b) {
		d.step()
	}
	d.flushText()
	return d.out
}

func (d *decoder) flushText() {
	if len(d.text) == 0 {
		return
	}
	var s string
	if d.kanji {
		if out, err := simplifiedchinese.GBK.NewDecoder().Bytes(d.text); err == nil {
			s = string(out)
		}
	}
	if s == "" {
		var sb strings.Builder
		for _, c := range d.text {
			sb.WriteRune(d.cp.Decode(c))
		}
		s = sb.String()
	}
	d.out = append(d.out, Token{Text: s})
	d.text = d.text[:0]
}

// take returns the next n bytes, or what is left and false.
func (d *decoder) take(n int) ([]byte, bool) {
	if n < 0 || d.i+n > len(d.b) {
		rest := d.b[d.i:]
		d.i = len(d.b)
		return rest, false
	}
	out := d.b[d.i : d.i+n]
	d.i += n
	return out, true
}

func ints(b []byte) []int {
	out := make([]int, len(b))
	for i, c := range b {
		out[i] = int(c)
	}
	return out
}

func le16(lo, hi byte) int { return int(lo) | int(hi)<<8 }

// emit records a command with n numeric arguments.
func (d *decoder) emit(cmd string, n int) Token {
	args, ok := d.take(n)
	t := Token{Cmd: cmd, Args: ints(args), Truncated: !ok}
	d.out = append(d.out, t)
	return t
}

// emitData records a command whose payload has the given length.
func (d *decoder) emitData(cmd string, args []int, n int) {
	data, ok := d.take(n)
	d.out = append(d.out, Token{Cmd: cmd, Args: args, Data: data, Truncated: !ok})
}

func (d *decoder) step() {
	c := d.b[d.i]
	switch c {
	case escpos.ESC, escpos.GS, escpos.FS, escpos.DLE, escpos.DC2:
		d.flushText()
		d.i++
		if d.i >= len(d.b) {
			d.out = append(d.out, Token{Cmd: names[c], Truncated: true})
			return
		}
		sub := d.b[d.i]
		d.i++
		switch c {
		case escpos.ESC:
			d.esc(sub)
		case escpos.GS:
			d.gs(sub)
		case escpos.FS:
			d.fs(sub)
		case escpos.DLE:
			d.dle(sub)
		case escpos.DC2:
			d.out = append(d.out, Token{Cmd: "DC2 " + char(sub)})
		}
	case escpos.LF, escpos.CR, escpos.HT, escpos.FF, escpos.NUL:
		d.flushText()
		d.i++
		d.out = append(d.out, Token{Cmd: names[c]})
	default:
		if c < 0x20 || c == 0x7F {
			d.flushText()
			d.i++
			d.out = append(d.out, Token{Cmd: fmt.Sprintf("0x%02X", c)})
			return
		}
		d.text = append(d.text, c)
		d.i++
	}
}

var names = map[byte]string{
	escpos.ESC: "ESC", escpos.GS: "GS", escpos.FS: "FS", escpos.DLE: "DLE", escpos.DC2: "DC2",
	escpos.LF: "LF", escpos.CR: "CR", escpos.HT: "HT", escpos.FF: "FF", escpos.NUL: "NUL",
	escpos.EOT: "EOT", escpos.ENQ: "ENQ", escpos.DC4: "DC4",
}

// char writes a command byte as in the reference: a letter or symbol as
// itself, a control code by name, anything else in hex.
func char(c byte) string {
	if n, ok := names[c]; ok && c < 0x20 {
		return n
	}
	if c == ' ' {
		return "SP"
	}
	if c > 0x20 && c < 0x7F {
		return string(c)
	}
	return fmt.Sprintf("0x%02X", c)
}

func (d *decoder) esc(c byte) {
	cmd := "ESC " + char(c)
	switch c {
	case '@':
		d.cp, d.kanji = 0, false
		d.emit(cmd, 0)
	case '2', 'L', 'S', 'i', 'm', escpos.FF:
		d.emit(cmd, 0)
	case ' ', '!', '%', '-', '3', '=', '?', 'E', 'G', 'J', 'M', 'R', 'T', 'V', 'a', 'd', '{', '9', 'c', 'r':
		t := d.emit(cmd, 1)
		if c == 'c' && len(t.Args) == 1 { // ESC c 3/4/5 n
			d.out = d.out[:len(d.out)-1]
			d.emit("ESC c "+char(byte(t.Args[0])), 1)
		}
	case 't':
		t := d.emit(cmd, 1)
		if len(t.Args) == 1 {
			d.cp = escpos.CodePage(t.Args[0])
		}
	case '$', '\\', 'B':
		d.emit(cmd, 2)
	case 'p':
		d.emit(cmd, 3)
	case 'W':
		d.emit(cmd, 8)
	case '*':
		h, ok := d.take(3)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Data: h, Truncated: true})
			return
		}
		per := 1
		if h[0] == 32 || h[0] == 33 {
			per = 3
		}
		d.emitData(cmd, ints(h), le16(h[1], h[2])*per)
	case '&':
		h, ok := d.take(3) // y c1 c2
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Data: h, Truncated: true})
			return
		}
		start := d.i
		for range max(int(h[2])-int(h[1])+1, 0) {
			w, ok := d.take(1)
			if !ok {
				break
			}
			d.take(int(h[0]) * int(w[0]))
		}
		d.out = append(d.out, Token{Cmd: cmd, Args: ints(h), Data: d.b[start:d.i]})
	case 'D':
		start := d.i
		for d.i < len(d.b) && d.b[d.i] != escpos.NUL {
			d.i++
		}
		args := ints(d.b[start:d.i])
		_, ok := d.take(1)
		d.out = append(d.out, Token{Cmd: cmd, Args: args, Truncated: !ok})
	case 'Z':
		h, ok := d.take(5) // m n k dL dH
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Data: h, Truncated: true})
			return
		}
		d.emitData(cmd, ints(h[:3]), le16(h[3], h[4]))
	case '(':
		d.paren(cmd)
	default:
		d.out = append(d.out, Token{Cmd: cmd})
	}
}

// paren decodes "ESC (" and "GS (" commands: fn pL pH followed by the
// parameter bytes, which become arguments, except that the data of GS ( k's
// store function (cn 80 m d...) is kept as data.
func (d *decoder) paren(prefix string) {
	h, ok := d.take(3)
	if !ok {
		d.out = append(d.out, Token{Cmd: prefix, Data: h, Truncated: true})
		return
	}
	cmd := prefix + " " + char(h[0])
	n := le16(h[1], h[2])
	body, ok := d.take(n)
	t := Token{Cmd: cmd, Truncated: !ok}
	split := n
	if h[0] == 'k' && len(body) >= 3 && body[1] == 80 {
		split = 3
	}
	split = min(split, len(body))
	t.Args, t.Data = ints(body[:split]), body[split:]
	d.out = append(d.out, t)
}

func (d *decoder) gs(c byte) {
	cmd := "GS " + char(c)
	switch c {
	case ':', 'c', escpos.FF:
		d.emit(cmd, 0)
	case '!', '/', 'B', 'H', 'I', 'Z', 'a', 'f', 'h', 'r', 'w', 'x':
		d.emit(cmd, 1)
	case '$', '\\', 'L', 'P', 'W':
		d.emit(cmd, 2)
	case '^':
		d.emit(cmd, 3)
	case 'V':
		m, ok := d.take(1)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Truncated: true})
			return
		}
		if m[0] == 65 || m[0] == 66 {
			n, ok := d.take(1)
			d.out = append(d.out, Token{Cmd: cmd, Args: ints(append([]byte{m[0]}, n...)), Truncated: !ok})
			return
		}
		d.out = append(d.out, Token{Cmd: cmd, Args: ints(m)})
	case '*':
		h, ok := d.take(2)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Data: h, Truncated: true})
			return
		}
		d.emitData(cmd, ints(h), int(h[0])*int(h[1])*8)
	case 'v':
		h, ok := d.take(6) // '0' m xL xH yL yH
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Data: h, Truncated: true})
			return
		}
		x, y := le16(h[2], h[3]), le16(h[4], h[5])
		d.emitData("GS v "+char(h[0]), []int{int(h[1]), x, y}, x*y)
	case 'k':
		m, ok := d.take(1)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Truncated: true})
			return
		}
		if m[0] <= 6 { // NUL-terminated form
			start := d.i
			for d.i < len(d.b) && d.b[d.i] != escpos.NUL {
				d.i++
			}
			data := d.b[start:d.i]
			_, ok := d.take(1)
			d.out = append(d.out, Token{Cmd: cmd, Args: ints(m), Data: data, Truncated: !ok})
			return
		}
		n, ok := d.take(1)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Args: ints(m), Truncated: true})
			return
		}
		d.emitData(cmd, ints(m), int(n[0]))
	case 'C':
		sub, ok := d.take(1)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Truncated: true})
			return
		}
		switch sub[0] {
		case '0':
			d.emit("GS C 0", 2)
		case '1':
			d.emit("GS C 1", 6)
		case '2':
			d.emit("GS C 2", 2)
		case ';': // five decimal fields, each ended by ';'
			start := d.i
			for fields := 0; fields < 5 && d.i < len(d.b); d.i++ {
				if d.b[d.i] == ';' {
					fields++
				}
			}
			d.out = append(d.out, Token{Cmd: "GS C ;", Data: d.b[start:d.i]})
		default:
			d.out = append(d.out, Token{Cmd: "GS C " + char(sub[0])})
		}
	case '(':
		d.paren(cmd)
	default:
		d.out = append(d.out, Token{Cmd: cmd})
	}
}

func (d *decoder) fs(c byte) {
	cmd := "FS " + char(c)
	switch c {
	case '&':
		d.kanji = true
		d.emit(cmd, 0)
	case '.':
		d.kanji = false
		d.emit(cmd, 0)
	case '!', '-', 'W':
		d.emit(cmd, 1)
	case 'S', 'p':
		d.emit(cmd, 2)
	case '2':
		h, ok := d.take(2)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Data: h, Truncated: true})
			return
		}
		d.emitData(cmd, ints(h), 72)
	case 'q':
		n, ok := d.take(1)
		if !ok {
			d.out = append(d.out, Token{Cmd: cmd, Truncated: true})
			return
		}
		start := d.i
		for range int(n[0]) {
			h, ok := d.take(4)
			if !ok {
				break
			}
			d.take(le16(h[0], h[1]) * le16(h[2], h[3]) * 8)
		}
		d.out = append(d.out, Token{Cmd: cmd, Args: ints(n), Data: d.b[start:d.i]})
	default:
		d.out = append(d.out, Token{Cmd: cmd})
	}
}

func (d *decoder) dle(c byte) {
	cmd := "DLE " + char(c)
	switch c {
	case escpos.EOT, escpos.ENQ:
		d.emit(cmd, 1)
	case escpos.DC4:
		d.emit(cmd, 3)
	default:
		d.out = append(d.out, Token{Cmd: cmd})
	}
}
