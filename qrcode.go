package escpos

// QR code commands using the standard ESC/POS GS ( k function set
// (cn = 49). They are not in the RP32x reference, but they are what the
// RP326's Epson-compatible firmware actually prints QR codes with (verified
// on firmware 7.03). The reference's ESC Z / GS Z route, in barcode.go,
// prints PDF417 on that printer instead.

// QRModel is a QR code model for [Builder.SelectQRCodeModel].
type QRModel byte

const (
	QRModel1 QRModel = 49
	QRModel2 QRModel = 50 // the common model, and the default
)

// MaxQRCodeData is the most data [Builder.StoreQRCodeData] accepts.
const MaxQRCodeData = 7089

func (b *Builder) qrFunction(fn byte, params ...byte) {
	l, h := le16(uint16(len(params) + 2))
	b.cmd(GS, '(', 'k', l, h, 49, fn)
	b.cmd(params...)
}

// SelectQRCodeModel selects the QR code model (GS ( k 4 0 49 65 n1 0).
func (b *Builder) SelectQRCodeModel(m QRModel) error {
	if m != QRModel1 && m != QRModel2 {
		return invalid("GS ( k", "QR model %d is not 49 or 50", m)
	}
	b.qrFunction(65, byte(m), 0)
	return nil
}

// SetQRCodeModuleSize sets the size of one QR code module in dots, 1–16
// (GS ( k 3 0 49 67 n).
func (b *Builder) SetQRCodeModuleSize(dots uint8) error {
	if dots < 1 || dots > 16 {
		return invalid("GS ( k", "QR module size %d out of range 1–16", dots)
	}
	b.qrFunction(67, dots)
	return nil
}

// SetQRCodeErrorCorrection sets the QR code error correction level
// (GS ( k 3 0 49 69 n).
func (b *Builder) SetQRCodeErrorCorrection(ec QRErrorCorrection) error {
	n, ok := map[QRErrorCorrection]byte{QRErrorL: 48, QRErrorM: 49, QRErrorQ: 50, QRErrorH: 51}[ec]
	if !ok {
		return invalid("GS ( k", "unknown QR error correction level %d", ec)
	}
	b.qrFunction(69, n)
	return nil
}

// StoreQRCodeData stores data in the printer's QR code symbol buffer
// (GS ( k pL pH 49 80 48 d1...dk). Print it with [Builder.PrintStoredQRCode].
func (b *Builder) StoreQRCodeData(data string) error {
	if len(data) < 1 || len(data) > MaxQRCodeData {
		return invalid("GS ( k", "QR data length %d out of range 1–%d", len(data), MaxQRCodeData)
	}
	l, h := le16(uint16(len(data) + 3))
	b.cmd(GS, '(', 'k', l, h, 49, 80, 48)
	b.Raw([]byte(data)...)
	return nil
}

// PrintStoredQRCode prints the QR code stored with [Builder.StoreQRCodeData]
// (GS ( k 3 0 49 81 48).
func (b *Builder) PrintStoredQRCode() { b.qrFunction(81, 48) }

// PrintQRCode prints a QR code using the standard GS ( k commands: model 2,
// the given error correction level and module size (1–16 dots), then the
// data. The symbol version is chosen automatically. Like other bar codes, it
// follows [Builder.SetAlign].
func (b *Builder) PrintQRCode(data string, ec QRErrorCorrection, moduleSize uint8) error {
	// Validate everything before writing anything.
	if moduleSize < 1 || moduleSize > 16 {
		return invalid("GS ( k", "QR module size %d out of range 1–16", moduleSize)
	}
	if ec != QRErrorL && ec != QRErrorM && ec != QRErrorQ && ec != QRErrorH {
		return invalid("GS ( k", "unknown QR error correction level %d", ec)
	}
	if len(data) < 1 || len(data) > MaxQRCodeData {
		return invalid("GS ( k", "QR data length %d out of range 1–%d", len(data), MaxQRCodeData)
	}
	_ = b.SelectQRCodeModel(QRModel2)
	_ = b.SetQRCodeModuleSize(moduleSize)
	_ = b.SetQRCodeErrorCorrection(ec)
	_ = b.StoreQRCodeData(data)
	b.PrintStoredQRCode()
	return nil
}
