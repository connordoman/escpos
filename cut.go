package escpos

// CutMode is a cut mode for [Builder.Cut].
type CutMode byte

const (
	// CutPartial cuts the paper leaving one point uncut.
	CutPartial CutMode = 1

	// CutFull is not listed in the RP32x reference, which says only partial
	// cuts are available, but it is the standard ESC/POS full cut and was
	// accepted by the printer this package was developed against.
	CutFull CutMode = 0
)

// Cut cuts the paper at the current position (GS V m). It only takes effect
// at the beginning of a line. Note that the cutter sits above the print
// head, so text just printed has not reached it yet; use
// [Builder.FeedAndCut] to feed it out first.
func (b *Builder) Cut(m CutMode) error {
	if m > 1 && m != 48 && m != 49 {
		return invalid("GS V", "cut mode %d is not 0, 1, 48 or 49", m)
	}
	b.cmd(GS, 'V', byte(m))
	return nil
}

// FeedAndCut feeds the paper to the cutting position plus n motion units and
// makes a partial cut (GS V 66 n). With n = 0 it feeds just far enough to cut
// below the last printed line.
func (b *Builder) FeedAndCut(n uint8) { b.cmd(GS, 'V', 66, n) }

// FeedAndFullCut is like [Builder.FeedAndCut] but requests a full cut
// (GS V 65 n). It is not listed in the RP32x reference; printers without a
// full cut make a partial cut instead.
func (b *Builder) FeedAndFullCut(n uint8) { b.cmd(GS, 'V', 65, n) }

// CutImmediate cuts the paper with the single-byte legacy command (ESC i).
// On this printer it makes a partial cut.
func (b *Builder) CutImmediate() { b.cmd(ESC, 'i') }

// PartialCutImmediate makes a partial cut with the single-byte legacy command
// (ESC m).
func (b *Builder) PartialCutImmediate() { b.cmd(ESC, 'm') }
