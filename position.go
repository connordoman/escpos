package escpos

// Align is a justification for [Builder.SetAlign].
type Align byte

const (
	AlignLeft   Align = 0
	AlignCenter Align = 1
	AlignRight  Align = 2
)

// SetAlign selects left, centre or right justification (ESC a n). It only
// takes effect at the beginning of a line in standard mode.
func (b *Builder) SetAlign(a Align) error {
	if a > 2 && (a < 48 || a > 50) {
		return invalid("ESC a", "justification %d out of range 0–2", a)
	}
	b.cmd(ESC, 'a', byte(a))
	return nil
}

// DefaultLineSpacing selects the default line spacing of 30 motion units,
// 3.75 mm (ESC 2).
func (b *Builder) DefaultLineSpacing() { b.cmd(ESC, '2') }

// SetLineSpacing sets the line spacing to n motion units (ESC 3 n).
func (b *Builder) SetLineSpacing(n uint8) { b.cmd(ESC, '3', n) }

// SetLeftMargin sets the left margin in motion units (GS L nL nH). It only
// takes effect at the beginning of a line in standard mode.
func (b *Builder) SetLeftMargin(units uint16) {
	l, h := le16(units)
	b.cmd(GS, 'L', l, h)
}

// SetPrintAreaWidth sets the printing area width in motion units
// (GS W nL nH). See the PaperWidth constants for the defaults.
func (b *Builder) SetPrintAreaWidth(units uint16) {
	l, h := le16(units)
	b.cmd(GS, 'W', l, h)
}

// SetAbsolutePosition sets the print position to units from the beginning of
// the line (ESC $ nL nH).
func (b *Builder) SetAbsolutePosition(units uint16) {
	l, h := le16(units)
	b.cmd(ESC, '$', l, h)
}

// SetRelativePosition moves the print position by units from the current
// position; negative values move left (ESC \ nL nH).
func (b *Builder) SetRelativePosition(units int16) {
	l, h := le16(uint16(units))
	b.cmd(ESC, '\\', l, h)
}

// SetTabPositions sets horizontal tab stops at the given columns
// (ESC D n1...nk NUL). Columns must be strictly ascending, between 1 and 255,
// and at most 32 may be given. Calling it with no columns clears all tab
// stops. The default is every 8 columns.
func (b *Builder) SetTabPositions(columns ...uint8) error {
	if len(columns) > 32 {
		return invalid("ESC D", "%d tab positions given, maximum is 32", len(columns))
	}
	for i, c := range columns {
		if c == 0 {
			return invalid("ESC D", "tab position must be at least 1")
		}
		if i > 0 && c <= columns[i-1] {
			return invalid("ESC D", "tab positions must be strictly ascending")
		}
	}
	b.cmd(ESC, 'D')
	b.cmd(columns...)
	b.cmd(NUL)
	return nil
}
