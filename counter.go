package escpos

import "strconv"

// CounterAlign selects how [Builder.PrintCounter] pads the counter value.
type CounterAlign byte

const (
	CounterAlignRightSpaces CounterAlign = 0 // right-aligned, padded with spaces
	CounterAlignRightZeros  CounterAlign = 1 // right-aligned, padded with zeros
	CounterAlignLeftSpaces  CounterAlign = 2 // left-aligned, padded with spaces
)

// SetCounterPrintMode selects how the serial number counter is printed
// (GS C 0 n m). digits is the number of digits, 1–5, or 0 to print only as
// many digits as the value needs (align is then ignored).
func (b *Builder) SetCounterPrintMode(digits uint8, align CounterAlign) error {
	if digits > 5 {
		return invalid("GS C 0", "digits %d out of range 0–5", digits)
	}
	if align > 2 && (align < 48 || align > 50) {
		return invalid("GS C 0", "alignment %d out of range 0–2", align)
	}
	b.cmd(GS, 'C', '0', digits, byte(align))
	return nil
}

// SetCounterModeA selects the counter range and step (GS C 1 aL aH bL bH n r).
// The counter counts up from a to b when a < b, down from a to b when a > b,
// and wraps around at the end. step is the increment and repeat the number of
// times each value is printed before stepping. Counting stops when a = b or
// step or repeat is 0.
func (b *Builder) SetCounterModeA(a, bound uint16, step, repeat uint8) {
	al, ah := le16(a)
	bl, bh := le16(bound)
	b.cmd(GS, 'C', '1', al, ah, bl, bh, step, repeat)
}

// SetCounter sets the serial number counter value (GS C 2 nL nH).
func (b *Builder) SetCounter(v uint16) {
	l, h := le16(v)
	b.cmd(GS, 'C', '2', l, h)
}

// CounterModeB holds the parameters of [Builder.SetCounterModeB]. Nil fields
// are omitted, leaving the printer's current value unchanged.
type CounterModeB struct {
	A      *uint16 // range start (sa)
	B      *uint16 // range end (sb)
	Step   *uint8  // increment (sn)
	Repeat *uint8  // repetitions per value (sr)
	Value  *uint8  // counter value (sc)
}

// SetCounterModeB selects the counter range, step and value using decimal
// parameters (GS C ; sa ; sb ; sn ; sr ; sc ;). The counting rules are the
// same as for [Builder.SetCounterModeA].
//
// On the RP326, the first [Builder.PrintCounter] afterwards prints
// Value + Step rather than Value. [Builder.SetCounter] does not have this
// offset.
func (b *Builder) SetCounterModeB(m CounterModeB) {
	b.cmd(GS, 'C', ';')
	field := func(v *uint64) {
		if v != nil {
			b.buf = strconv.AppendUint(b.buf, *v, 10)
		}
		b.cmd(';')
	}
	field(u16(m.A))
	field(u16(m.B))
	field(u8(m.Step))
	field(u8(m.Repeat))
	field(u8(m.Value))
}

func u16(p *uint16) *uint64 {
	if p == nil {
		return nil
	}
	v := uint64(*p)
	return &v
}

func u8(p *uint8) *uint64 {
	if p == nil {
		return nil
	}
	v := uint64(*p)
	return &v
}

// PrintCounter places the current counter value in the print buffer, then
// steps the counter (GS c). The value prints with the next line.
func (b *Builder) PrintCounter() { b.cmd(GS, 'c') }
