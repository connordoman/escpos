package escpos

// MaxMacroSize is the largest macro the printer stores, in bytes.
const MaxMacroSize = 2048

// ToggleMacroDefinition starts macro definition, or ends it if one is in
// progress (GS :). Everything sent in between is stored as the macro instead
// of being executed. Prefer [Builder.DefineMacro].
func (b *Builder) ToggleMacroDefinition() { b.cmd(GS, ':') }

// DefineMacro records the commands appended by fn as the printer's macro,
// wrapping them in GS : ... GS :. The macro survives ESC @ but not power off.
func (b *Builder) DefineMacro(fn func(m *Builder) error) error {
	m := &Builder{paperWidth: b.paperWidth, font: b.font, widthMul: b.widthMul, codePage: b.codePage, encoder: b.encoder}
	if err := fn(m); err != nil {
		return err
	}
	if m.Len() > MaxMacroSize {
		return invalid("GS :", "macro is %d bytes, maximum is %d", m.Len(), MaxMacroSize)
	}
	b.cmd(GS, ':')
	b.Append(m)
	b.cmd(GS, ':')
	return nil
}

// ExecuteMacro runs the defined macro times times, waiting wait×100 ms before
// each run (GS ^ r t m). If waitForButton is set, the printer instead blinks
// the paper-out LED after each wait and runs the macro once each time the
// FEED button is pressed.
func (b *Builder) ExecuteMacro(times, wait uint8, waitForButton bool) {
	b.cmd(GS, '^', times, wait, boolByte(waitForButton))
}
