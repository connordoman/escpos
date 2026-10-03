package escpos

// Initialize clears the print buffer and resets the printer modes to their
// power-on state (ESC @). The receive buffer and macro definition are kept.
func (b *Builder) Initialize() {
	b.cmd(ESC, '@')
	b.resetState()
}

// Tab moves the print position to the next horizontal tab position (HT).
func (b *Builder) Tab() { b.cmd(HT) }

// LineFeed prints the buffered line and feeds one line (LF).
func (b *Builder) LineFeed() { b.cmd(LF) }

// CarriageReturn behaves like [Builder.LineFeed] when automatic line feed is
// enabled and is ignored otherwise (CR).
func (b *Builder) CarriageReturn() { b.cmd(CR) }

// FeedUnits prints the buffered data and feeds the paper n motion units
// (ESC J n), 0.125 mm each by default.
func (b *Builder) FeedUnits(n uint8) { b.cmd(ESC, 'J', n) }

// FeedLines prints the buffered data and feeds n lines at the current line
// spacing (ESC d n).
func (b *Builder) FeedLines(n uint8) { b.cmd(ESC, 'd', n) }

// SetPeripheralDevice enables or disables the printer (ESC = n). While
// disabled the printer ignores everything except real-time commands and
// ESC = itself.
func (b *Builder) SetPeripheralDevice(printerEnabled bool) {
	b.cmd(ESC, '=', boolByte(printerEnabled))
}

// PrintTestPage prints the printer's self-test page (DC2 T).
func (b *Builder) PrintTestPage() { b.cmd(DC2, 'T') }

// TestPaper selects the paper for [Builder.ExecuteTestPrint].
type TestPaper byte

const (
	TestPaperBasic TestPaper = 0 // basic sheet (paper roll)
	TestPaperRoll  TestPaper = 1 // paper roll
	TestPaperRoll2 TestPaper = 2 // paper roll
)

// TestPattern selects the pattern for [Builder.ExecuteTestPrint].
type TestPattern byte

const (
	TestPatternHexDump TestPattern = 1 // hexadecimal dump
	TestPatternStatus  TestPattern = 2 // printer status print
	TestPatternRolling TestPattern = 3 // rolling pattern print
)

// ExecuteTestPrint prints a test pattern on the given paper
// (GS ( A pL pH n m). The printer resets itself afterwards, clearing
// user-defined characters, downloaded bit images and macros, and cuts the
// paper.
func (b *Builder) ExecuteTestPrint(paper TestPaper, pattern TestPattern) error {
	if paper > 2 && (paper < 48 || paper > 50) {
		return invalid("GS ( A", "paper %d out of range 0–2", paper)
	}
	if (pattern < 1 || pattern > 3) && (pattern < 49 || pattern > 51) {
		return invalid("GS ( A", "pattern %d out of range 1–3", pattern)
	}
	b.cmd(GS, '(', 'A', 2, 0, byte(paper), byte(pattern))
	return nil
}

// SetPanelButtons enables or disables the panel (FEED) button (ESC c 5 n).
func (b *Builder) SetPanelButtons(enabled bool) {
	b.cmd(ESC, 'c', '5', boolByte(!enabled))
}

// SetMotionUnits sets the horizontal and vertical motion units to 1/x and 1/y
// inch (GS P x y). Zero selects the default for that axis (x = 200, y = 400).
func (b *Builder) SetMotionUnits(x, y uint8) { b.cmd(GS, 'P', x, y) }
