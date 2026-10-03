package escpos

// DrawerPin selects a pin of the drawer kick-out connector (the RJ11 socket).
type DrawerPin byte

const (
	DrawerPin2 DrawerPin = 0 // first drawer
	DrawerPin5 DrawerPin = 1 // second drawer
)

// GeneratePulse outputs a pulse on the drawer kick-out connector
// (ESC p m t1 t2). The pulse is on for onTime×2 ms and off for offTime×2 ms.
func (b *Builder) GeneratePulse(pin DrawerPin, onTime, offTime uint8) error {
	if pin > 1 && pin != 48 && pin != 49 {
		return invalid("ESC p", "pin %d is not 0, 1, 48 or 49", pin)
	}
	b.cmd(ESC, 'p', byte(pin), onTime, offTime)
	return nil
}

// OpenCashDrawer kicks the drawer on pin with a 100 ms pulse followed by a
// 500 ms pause, which suits most solenoid drawers.
func (b *Builder) OpenCashDrawer(pin DrawerPin) error {
	return b.GeneratePulse(pin, 50, 250)
}

// Beep sounds the buzzer (ESC B n t): tone n of 1–9 for time t of 1–9. The
// reference gives no units for either parameter.
func (b *Builder) Beep(n, t uint8) error {
	if n < 1 || n > 9 || t < 1 || t > 9 {
		return invalid("ESC B", "n=%d t=%d out of range 1–9", n, t)
	}
	b.cmd(ESC, 'B', n, t)
	return nil
}

// Buzz sounds the buzzer for duration×100 ms (ESC ( A 4 0 48 0 1 t). This
// command is not in the RP32x reference; it is carried over from the
// connordoman/pos project, where it worked on an RP326.
func (b *Builder) Buzz(duration uint8) {
	b.cmd(ESC, '(', 'A', 4, 0, 0x30, 0, 1, duration)
}
