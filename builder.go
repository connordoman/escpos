package escpos

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
)

// ASCII control codes used by the command set.
const (
	NUL = 0x00
	EOT = 0x04
	ENQ = 0x05
	HT  = 0x09
	LF  = 0x0A
	FF  = 0x0C
	CR  = 0x0D
	DLE = 0x10
	DC2 = 0x12
	DC4 = 0x14
	ESC = 0x1B
	FS  = 0x1C
	GS  = 0x1D
)

// Printable widths, in dots, for the paper-width models listed under GS W.
const (
	PaperWidth80mm = 576 // 79.5 mm paper-width model (RP326 default)
	PaperWidth82mm = 640 // 82.5 mm paper-width model
	PaperWidth60mm = 448 // 60 mm paper-width model
	PaperWidth58mm = 432 // 58 mm paper-width model
)

// ErrInvalidArgument is wrapped by every error returned for a parameter that
// is outside the range the command set allows.
var ErrInvalidArgument = errors.New("escpos: invalid argument")

func invalid(cmd, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrInvalidArgument, cmd, fmt.Sprintf(format, args...))
}

// Builder accumulates ESC/POS commands. The zero value is ready to use and
// assumes an 80 mm (576 dot) printer in its power-on state.
//
// Methods that take parameters with a restricted range validate them and
// return an error wrapping [ErrInvalidArgument]; on error nothing is written.
//
// A Builder is not safe for concurrent use.
type Builder struct {
	buf []byte

	// Tracked printer state, used for text encoding and layout helpers.
	paperWidth int
	font       Font
	widthMul   int
	codePage   CodePage
	encoder    *encoding.Encoder
}

// NewBuilder returns an empty Builder for a printer whose printable width is
// paperWidth dots. A paperWidth of 0 selects [PaperWidth80mm].
func NewBuilder(paperWidth int) *Builder {
	return &Builder{paperWidth: paperWidth}
}

// Bytes returns a copy of the buffered commands.
func (b *Builder) Bytes() []byte {
	if len(b.buf) == 0 {
		return nil
	}
	out := make([]byte, len(b.buf))
	copy(out, b.buf)
	return out
}

// Len returns the number of buffered bytes.
func (b *Builder) Len() int { return len(b.buf) }

// Reset discards the buffered commands. Tracked printer state (font, code
// page, ...) is kept, since the printer itself is unaffected.
func (b *Builder) Reset() { b.buf = b.buf[:0] }

// Write appends raw bytes to the buffer. It implements [io.Writer] and never
// fails.
func (b *Builder) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// WriteByte appends a single raw byte. It implements [io.ByteWriter].
func (b *Builder) WriteByte(c byte) error {
	b.buf = append(b.buf, c)
	return nil
}

// Raw appends raw bytes to the buffer, for commands this package does not
// wrap.
func (b *Builder) Raw(p ...byte) { b.buf = append(b.buf, p...) }

// Append appends the contents of another Builder.
func (b *Builder) Append(other *Builder) { b.buf = append(b.buf, other.buf...) }

func (b *Builder) cmd(p ...byte) { b.buf = append(b.buf, p...) }

// PaperWidth returns the printable width in dots that layout helpers assume.
func (b *Builder) PaperWidth() int {
	if b.paperWidth <= 0 {
		return PaperWidth80mm
	}
	return b.paperWidth
}

// SetPaperWidth sets the printable width in dots that layout helpers assume.
// It does not send anything to the printer; see [Builder.SetPrintAreaWidth].
func (b *Builder) SetPaperWidth(dots int) { b.paperWidth = dots }

// CharactersPerLine returns how many characters of the current font and
// width multiplier fit on one line of the configured paper width, ignoring
// character spacing set by ESC SP.
func (b *Builder) CharactersPerLine() int {
	mul := max(b.widthMul, 1)
	return b.PaperWidth() / (b.font.Width() * mul)
}

// HorizontalRule prints a full-width line made of r followed by a line feed.
// The CP437 box-drawing character '─' gives a solid rule with the default
// code page.
func (b *Builder) HorizontalRule(r rune) {
	b.Textln(strings.Repeat(string(r), b.CharactersPerLine()))
}

// Columns prints left and right on one line, padding between them so that
// right is flush with the right edge, followed by a line feed. If they do not
// fit, they are separated by a single space and the printer wraps the line.
func (b *Builder) Columns(left, right string) {
	pad := b.CharactersPerLine() - len([]rune(left)) - len([]rune(right))
	b.Textln(left + strings.Repeat(" ", max(pad, 1)) + right)
}

// resetState mirrors the effect of ESC @ on tracked state.
func (b *Builder) resetState() {
	b.font = FontA
	b.widthMul = 1
	b.codePage = CodePagePC437
}

func le16(v uint16) (byte, byte) { return byte(v), byte(v >> 8) }

func boolByte(on bool) byte {
	if on {
		return 1
	}
	return 0
}
