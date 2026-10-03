package escpos

// EnterPageMode switches from standard mode to page mode (ESC L). In page
// mode data is laid out in the area set by [Builder.SetPageArea] and printed
// all at once by [Builder.PrintPage] or [Builder.PrintPageAndExit].
func (b *Builder) EnterPageMode() { b.cmd(ESC, 'L') }

// EnterStandardMode switches from page mode back to standard mode, discarding
// data buffered in page mode (ESC S).
func (b *Builder) EnterStandardMode() { b.cmd(ESC, 'S') }

// PrintPage prints everything buffered in page mode and stays in page mode
// (ESC FF). The buffered data and page settings are kept.
func (b *Builder) PrintPage() { b.cmd(ESC, FF) }

// PrintPageAndExit prints everything buffered in page mode, clears it and
// returns to standard mode (FF). It only has an effect in page mode.
func (b *Builder) PrintPageAndExit() { b.cmd(FF) }

// PageDirection is a print direction and starting corner for
// [Builder.SetPageDirection].
type PageDirection byte

const (
	PageLeftToRight PageDirection = 0 // starts upper left
	PageBottomToTop PageDirection = 1 // starts lower left
	PageRightToLeft PageDirection = 2 // starts lower right
	PageTopToBottom PageDirection = 3 // starts upper right
)

// SetPageDirection selects the print direction and starting position in
// page mode (ESC T n).
func (b *Builder) SetPageDirection(d PageDirection) error {
	if d > 3 && (d < 48 || d > 51) {
		return invalid("ESC T", "direction %d out of range 0–3", d)
	}
	b.cmd(ESC, 'T', byte(d))
	return nil
}

// SetPageArea sets the printing area in page mode (ESC W xL xH yL yH dxL dxH
// dyL dyH): origin (x, y) and size width × height, in motion units. Width and
// height must be non-zero.
func (b *Builder) SetPageArea(x, y, width, height uint16) error {
	if width == 0 || height == 0 {
		return invalid("ESC W", "area %d×%d must be non-zero", width, height)
	}
	xl, xh := le16(x)
	yl, yh := le16(y)
	wl, wh := le16(width)
	hl, hh := le16(height)
	b.cmd(ESC, 'W', xl, xh, yl, yh, wl, wh, hl, hh)
	return nil
}

// SetAbsoluteVerticalPosition sets the vertical print position in page mode
// to units from the starting position set by [Builder.SetPageDirection]
// (GS $ nL nH).
func (b *Builder) SetAbsoluteVerticalPosition(units uint16) {
	l, h := le16(units)
	b.cmd(GS, '$', l, h)
}

// SetRelativeVerticalPosition moves the vertical print position in page mode
// by units; negative values move up (GS \ nL nH).
func (b *Builder) SetRelativeVerticalPosition(units int16) {
	l, h := le16(uint16(units))
	b.cmd(GS, '\\', l, h)
}

// FeedToMark feeds black-mark paper to the print starting position (GS FF).
// It only has an effect when the black mark sensor is enabled.
func (b *Builder) FeedToMark() { b.cmd(GS, FF) }
