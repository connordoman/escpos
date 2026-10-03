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

// Beep sounds the buzzer (ESC B n t), with n and t each from 1 to 9. The
// reference calls n the tone and t the time but gives no units. On the RP326,
// ESC B 1 2 gave a single beep of about 0.3 s.
func (b *Builder) Beep(n, t uint8) error {
	if n < 1 || n > 9 || t < 1 || t > 9 {
		return invalid("ESC B", "n=%d t=%d out of range 1–9", n, t)
	}
	b.cmd(ESC, 'B', n, t)
	return nil
}
