package escpos

// Font is a character font.
type Font byte

const (
	FontA Font = 0 // 12×24 dots, 48 columns on 80 mm paper
	FontB Font = 1 // 9×17 dots, 64 columns on 80 mm paper

	// FontC is not listed in the RP32x reference but was accepted by the
	// printer this package was developed against. It has the same width as
	// FontB.
	FontC Font = 2
)

// Width returns the width of one character of the font in dots, excluding
// character spacing.
func (f Font) Width() int {
	if f == FontA {
		return 12
	}
	return 9
}

// normalize maps the ASCII digit forms ('0', '1', '2') to 0, 1, 2.
func (f Font) normalize() (Font, bool) {
	if f >= '0' {
		f -= '0'
	}
	return f, f <= FontC
}

// SelectFont selects the character font (ESC M n).
func (b *Builder) SelectFont(f Font) error {
	nf, ok := f.normalize()
	if !ok {
		return invalid("ESC M", "font %d out of range", f)
	}
	b.cmd(ESC, 'M', byte(f))
	b.font = nf
	return nil
}

// PrintMode is a set of flags for [Builder.SelectPrintMode].
type PrintMode byte

const (
	PrintModeFontB        PrintMode = 1 << 0 // Font B instead of Font A
	PrintModeEmphasized   PrintMode = 1 << 3
	PrintModeDoubleHeight PrintMode = 1 << 4
	PrintModeDoubleWidth  PrintMode = 1 << 5
	PrintModeUnderline    PrintMode = 1 << 7
)

// SelectPrintMode sets font, emphasis, double height, double width and
// underline in one command (ESC ! n). Flags that are not set are turned off.
func (b *Builder) SelectPrintMode(m PrintMode) {
	b.cmd(ESC, '!', byte(m))
	b.font = FontA
	if m&PrintModeFontB != 0 {
		b.font = FontB
	}
	b.widthMul = 1
	if m&PrintModeDoubleWidth != 0 {
		b.widthMul = 2
	}
}

// SetCharacterSize sets the character width and height multipliers, each
// from 1 to 8 (GS ! n).
func (b *Builder) SetCharacterSize(width, height uint8) error {
	if width < 1 || width > 8 || height < 1 || height > 8 {
		return invalid("GS !", "size %d×%d out of range 1–8", width, height)
	}
	b.cmd(GS, '!', (width-1)<<4|(height-1))
	b.widthMul = int(width)
	return nil
}

// SetEmphasis turns emphasized (bold) mode on or off (ESC E n).
func (b *Builder) SetEmphasis(on bool) { b.cmd(ESC, 'E', boolByte(on)) }

// SetDoubleStrike turns double-strike mode on or off (ESC G n). On this
// printer the output is identical to emphasized mode.
func (b *Builder) SetDoubleStrike(on bool) { b.cmd(ESC, 'G', boolByte(on)) }

// Underline is an underline thickness.
type Underline byte

const (
	UnderlineOff    Underline = 0
	UnderlineSingle Underline = 1 // 1 dot thick
	UnderlineDouble Underline = 2 // 2 dots thick
)

func (u Underline) valid() bool { return u <= 2 || (u >= 48 && u <= 50) }

// SetUnderline sets underline mode (ESC - n).
func (b *Builder) SetUnderline(u Underline) error {
	if !u.valid() {
		return invalid("ESC -", "underline %d out of range 0–2", u)
	}
	b.cmd(ESC, '-', byte(u))
	return nil
}

// SetReverse turns white/black reverse printing on or off (GS B n).
func (b *Builder) SetReverse(on bool) { b.cmd(GS, 'B', boolByte(on)) }

// SetRotate90 turns 90° clockwise rotation of characters on or off (ESC V n).
func (b *Builder) SetRotate90(on bool) { b.cmd(ESC, 'V', boolByte(on)) }

// SetUpsideDown turns upside-down (180°) printing on or off (ESC { n). It
// only takes effect at the beginning of a line in standard mode.
func (b *Builder) SetUpsideDown(on bool) { b.cmd(ESC, '{', boolByte(on)) }

// SetCharacterSpacing sets the right-side character spacing to n motion units
// (ESC SP n).
func (b *Builder) SetCharacterSpacing(n uint8) { b.cmd(ESC, ' ', n) }

// Charset is an international character set for
// [Builder.SelectInternationalCharset].
type Charset byte

const (
	CharsetUSA           Charset = 0
	CharsetFrance        Charset = 1
	CharsetGermany       Charset = 2
	CharsetUK            Charset = 3
	CharsetDenmarkI      Charset = 4
	CharsetSweden        Charset = 5
	CharsetItaly         Charset = 6
	CharsetSpainI        Charset = 7
	CharsetJapan         Charset = 8
	CharsetNorway        Charset = 9
	CharsetDenmarkII     Charset = 10
	CharsetSpainII       Charset = 11
	CharsetLatinAmerica  Charset = 12
	CharsetKorea         Charset = 13
	CharsetSloveniaCroat Charset = 14
	CharsetChina         Charset = 15
)

// SelectInternationalCharset selects an international character set
// (ESC R n).
func (b *Builder) SelectInternationalCharset(c Charset) error {
	if c > 15 {
		return invalid("ESC R", "character set %d out of range 0–15", c)
	}
	b.cmd(ESC, 'R', byte(c))
	return nil
}

// ChineseEncoding is a Chinese code format for
// [Builder.SelectChineseEncoding].
type ChineseEncoding byte

const (
	ChineseEncodingGBK  ChineseEncoding = 0
	ChineseEncodingUTF8 ChineseEncoding = 1
	ChineseEncodingBIG5 ChineseEncoding = 3
)

// SelectChineseEncoding selects the Chinese code format (ESC 9 n). It is only
// meaningful on Chinese-firmware models. To send UTF-8 text unchanged, pair
// it with SetTextEncoder(encoding.Nop.NewEncoder()).
func (b *Builder) SelectChineseEncoding(e ChineseEncoding) error {
	if e != ChineseEncodingGBK && e != ChineseEncodingUTF8 && e != ChineseEncodingBIG5 {
		return invalid("ESC 9", "encoding %d is not 0, 1 or 3", e)
	}
	b.cmd(ESC, '9', byte(e))
	return nil
}

// SelectUserDefinedCharset selects (true) or cancels (false) the
// user-defined character set (ESC % n).
func (b *Builder) SelectUserDefinedCharset(on bool) { b.cmd(ESC, '%', boolByte(on)) }

// Glyph is one user-defined character for [Builder.DefineUserCharacters].
//
// Data holds Width columns of 3 bytes each, left to right. Within a column
// the first byte is the top 8 dots, most significant bit at the top.
type Glyph struct {
	Width uint8 // dots: at most 12 for Font A, 9 for Font B
	Data  []byte
}

// DefineUserCharacters defines user-defined characters for consecutive codes
// starting at first, in the currently selected font
// (ESC & y c1 c2 [x d1...d(y×x)]...). Codes must lie within 0x20–0x7E.
// Defining characters clears the downloaded bit image.
func (b *Builder) DefineUserCharacters(first byte, glyphs ...Glyph) error {
	const y = 3
	if len(glyphs) == 0 {
		return invalid("ESC &", "no glyphs given")
	}
	last := int(first) + len(glyphs) - 1
	if first < 0x20 || last > 0x7E {
		return invalid("ESC &", "codes %#x–%#x outside 0x20–0x7E", first, last)
	}
	for i, g := range glyphs {
		if g.Width > 12 {
			return invalid("ESC &", "glyph %d width %d exceeds 12", i, g.Width)
		}
		if len(g.Data) != y*int(g.Width) {
			return invalid("ESC &", "glyph %d has %d bytes, want %d", i, len(g.Data), y*int(g.Width))
		}
	}
	b.cmd(ESC, '&', y, first, byte(last))
	for _, g := range glyphs {
		b.cmd(g.Width)
		b.cmd(g.Data...)
	}
	return nil
}

// CancelUserCharacter deletes the user-defined pattern for code c in the
// current font (ESC ? n).
func (b *Builder) CancelUserCharacter(c byte) error {
	if c < 0x20 || c > 0x7E {
		return invalid("ESC ?", "code %#x outside 0x20–0x7E", c)
	}
	b.cmd(ESC, '?', c)
	return nil
}
