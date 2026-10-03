package escpos

// Kanji (double-byte character) commands. These only have an effect on
// models with Chinese or Japanese firmware.

// KanjiPrintMode is a set of flags for [Builder.SetKanjiPrintMode].
type KanjiPrintMode byte

const (
	KanjiDoubleWidth  KanjiPrintMode = 1 << 2
	KanjiDoubleHeight KanjiPrintMode = 1 << 3
	KanjiUnderline    KanjiPrintMode = 1 << 7
)

// SetKanjiPrintMode sets double width, double height and underline for Kanji
// characters (FS ! n).
func (b *Builder) SetKanjiPrintMode(m KanjiPrintMode) { b.cmd(FS, '!', byte(m)) }

// SelectKanjiMode makes the printer treat Kanji codes as two-byte characters
// (FS &).
func (b *Builder) SelectKanjiMode() { b.cmd(FS, '&') }

// CancelKanjiMode makes the printer treat every byte as a single character
// (FS .).
func (b *Builder) CancelKanjiMode() { b.cmd(FS, '.') }

// SetKanjiUnderline sets underline mode for Kanji characters (FS - n).
func (b *Builder) SetKanjiUnderline(u Underline) error {
	if !u.valid() {
		return invalid("FS -", "underline %d out of range 0–2", u)
	}
	b.cmd(FS, '-', byte(u))
	return nil
}

// KanjiGlyphSize is the number of bytes in a user-defined Kanji character:
// 24 columns of 3 bytes (24×24 dots) for roll paper.
const KanjiGlyphSize = 72

// DefineKanjiCharacter defines a user-defined Kanji character for the code
// c1 c2 (FS 2 c1 c2 d1...dk). On Chinese models c1 must be 0xFE and c2 must
// be within 0xA1–0xFE. data is 24 columns of 3 bytes, laid out like
// [Glyph.Data].
func (b *Builder) DefineKanjiCharacter(c1, c2 byte, data []byte) error {
	if c1 != 0xFE || c2 < 0xA1 || c2 > 0xFE {
		return invalid("FS 2", "code %#x %#x outside FE A1–FE FE", c1, c2)
	}
	if len(data) != KanjiGlyphSize {
		return invalid("FS 2", "got %d bytes, want %d", len(data), KanjiGlyphSize)
	}
	b.cmd(FS, '2', c1, c2)
	b.cmd(data...)
	return nil
}

// SetKanjiSpacing sets the left- and right-side Kanji character spacing in
// motion units (FS S n1 n2).
func (b *Builder) SetKanjiSpacing(left, right uint8) { b.cmd(FS, 'S', left, right) }

// SetKanjiQuadruple turns quadruple-size mode for Kanji characters on or off
// (FS W n).
func (b *Builder) SetKanjiQuadruple(on bool) { b.cmd(FS, 'W', boolByte(on)) }
