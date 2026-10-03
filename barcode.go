package escpos

import (
	"strings"
)

// HRIPosition selects where human readable interpretation (HRI) characters
// are printed relative to a bar code.
type HRIPosition byte

const (
	HRINone  HRIPosition = 0
	HRIAbove HRIPosition = 1
	HRIBelow HRIPosition = 2
	HRIBoth  HRIPosition = 3
)

// SetHRIPosition selects where HRI characters are printed (GS H n).
func (b *Builder) SetHRIPosition(p HRIPosition) error {
	if p > 3 && (p < 48 || p > 51) {
		return invalid("GS H", "position %d out of range 0–3", p)
	}
	b.cmd(GS, 'H', byte(p))
	return nil
}

// SetHRIFont selects the font for HRI characters (GS f n): [FontA] or
// [FontB].
func (b *Builder) SetHRIFont(f Font) error {
	if nf, ok := f.normalize(); !ok || nf > FontB {
		return invalid("GS f", "font %d is not A or B", f)
	}
	b.cmd(GS, 'f', byte(f))
	return nil
}

// SetBarcodeHeight sets the bar code height in dots, at least 1 (GS h n).
// The default is 162.
func (b *Builder) SetBarcodeHeight(dots uint8) error {
	if dots < 1 {
		return invalid("GS h", "height must be at least 1")
	}
	b.cmd(GS, 'h', dots)
	return nil
}

// SetBarcodeWidth sets the bar code module width, from 2 (narrowest) to 6
// (GS w n). The default is 3. It also affects PDF417 symbols.
func (b *Builder) SetBarcodeWidth(n uint8) error {
	if n < 2 || n > 6 {
		return invalid("GS w", "width %d out of range 2–6", n)
	}
	b.cmd(GS, 'w', n)
	return nil
}

// SetBarcodeLeftSpace sets the bar code printing start position in dots from
// the left (GS x n).
func (b *Builder) SetBarcodeLeftSpace(n uint8) { b.cmd(GS, 'x', n) }

// BarcodeSystem is a bar code symbology for [Builder.PrintBarcode].
type BarcodeSystem byte

// Symbologies sent in length-prefixed form (GS k m n d1...dn). Prefer these.
const (
	BarcodeUPCA    BarcodeSystem = 65
	BarcodeUPCE    BarcodeSystem = 66
	BarcodeEAN13   BarcodeSystem = 67 // JAN13
	BarcodeEAN8    BarcodeSystem = 68 // JAN8
	BarcodeCode39  BarcodeSystem = 69
	BarcodeITF     BarcodeSystem = 70
	BarcodeCodabar BarcodeSystem = 71
	BarcodeCode93  BarcodeSystem = 72
	BarcodeCode128 BarcodeSystem = 73
)

// Symbologies sent in NUL-terminated form (GS k m d1...dk NUL).
const (
	BarcodeUPCANUL    BarcodeSystem = 0
	BarcodeUPCENUL    BarcodeSystem = 1
	BarcodeEAN13NUL   BarcodeSystem = 2
	BarcodeEAN8NUL    BarcodeSystem = 3
	BarcodeCode39NUL  BarcodeSystem = 4
	BarcodeITFNUL     BarcodeSystem = 5
	BarcodeCodabarNUL BarcodeSystem = 6
)

const (
	digits       = "0123456789"
	code39Chars  = digits + "ABCDEFGHIJKLMNOPQRSTUVWXYZ $%+-./"
	codabarChars = digits + "ABCD$+-./:"
)

func onlyChars(s, allowed string) bool {
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(allowed, s[i]) < 0 {
			return false
		}
	}
	return true
}

func onlyASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// validateBarcode checks data against the rules for sys, which must be one of
// the length-prefixed symbologies.
func validateBarcode(sys BarcodeSystem, data string) error {
	n := len(data)
	lenErr := func(lo, hi int) error {
		if n < lo || n > hi {
			return invalid("GS k", "%d characters, want %d–%d", n, lo, hi)
		}
		return nil
	}
	charErr := func(ok bool) error {
		if !ok {
			return invalid("GS k", "data %q contains characters the symbology cannot encode", data)
		}
		return nil
	}
	var err error
	switch sys {
	case BarcodeUPCA, BarcodeUPCE:
		err = lenErr(11, 12)
		if err == nil {
			err = charErr(onlyChars(data, digits))
		}
	case BarcodeEAN13:
		err = lenErr(12, 13)
		if err == nil {
			err = charErr(onlyChars(data, digits))
		}
	case BarcodeEAN8:
		err = lenErr(7, 8)
		if err == nil {
			err = charErr(onlyChars(data, digits))
		}
	case BarcodeCode39:
		err = lenErr(1, 255)
		if err == nil {
			err = charErr(onlyChars(data, code39Chars))
		}
	case BarcodeITF:
		err = lenErr(2, 254)
		if err == nil && n%2 != 0 {
			err = invalid("GS k", "ITF needs an even number of digits, got %d", n)
		}
		if err == nil {
			err = charErr(onlyChars(data, digits))
		}
	case BarcodeCodabar:
		err = lenErr(1, 255)
		if err == nil {
			err = charErr(onlyChars(data, codabarChars))
		}
	case BarcodeCode93:
		err = lenErr(1, 255)
		if err == nil {
			err = charErr(onlyASCII(data))
		}
	case BarcodeCode128:
		err = lenErr(2, 255)
		if err == nil {
			err = charErr(onlyASCII(data))
		}
		if err == nil {
			err = validateCode128(data)
		}
	default:
		err = invalid("GS k", "unknown bar code system %d", sys)
	}
	return err
}

func validateCode128(data string) error {
	if data[0] != '{' || strings.IndexByte("ABC", data[1]) < 0 {
		return invalid("GS k", "CODE128 data must start with {A, {B or {C")
	}
	for i := 0; i < len(data); i++ {
		if data[i] != '{' {
			continue
		}
		if i+1 >= len(data) || strings.IndexByte("SABC1234{", data[i+1]) < 0 {
			return invalid("GS k", "CODE128 data has an invalid { escape at byte %d", i)
		}
		i++
	}
	return nil
}

// PrintBarcode prints a bar code (GS k). The length-prefixed symbologies
// (BarcodeUPCA ... BarcodeCode128) are sent as GS k m n d1...dn and the
// NUL-terminated ones (BarcodeUPCANUL ... BarcodeCodabarNUL) as
// GS k m d1...dk NUL. Data is validated against the symbology's length and
// character rules. CODE128 data must begin with a code set selection such
// as "{B"; see [Code128].
//
// In standard mode the bar code is only printed if the print buffer is
// empty, so print it at the beginning of a line.
func (b *Builder) PrintBarcode(sys BarcodeSystem, data string) error {
	if sys <= BarcodeCodabarNUL {
		if err := validateBarcode(sys+BarcodeUPCA, data); err != nil {
			return err
		}
		b.cmd(GS, 'k', byte(sys))
		b.Raw([]byte(data)...)
		b.cmd(NUL)
		return nil
	}
	if err := validateBarcode(sys, data); err != nil {
		return err
	}
	b.cmd(GS, 'k', byte(sys), byte(len(data)))
	b.Raw([]byte(data)...)
	return nil
}

// Code128 returns data for a CODE128 bar code encoding s. Strings of an even
// number (at least 4) of digits use the compact code set C; everything else
// uses code set B with '{' escaped. s must be ASCII.
func Code128(s string) string {
	if len(s) >= 4 && len(s)%2 == 0 && onlyChars(s, digits) {
		out := make([]byte, 0, 2+len(s)/2)
		out = append(out, '{', 'C')
		for i := 0; i < len(s); i += 2 {
			out = append(out, (s[i]-'0')*10+(s[i+1]-'0'))
		}
		return string(out)
	}
	return "{B" + strings.ReplaceAll(s, "{", "{{")
}

// Barcode2DType is a two-dimensional symbology for
// [Builder.Select2DBarcodeType].
type Barcode2DType byte

const (
	Barcode2DPDF417 Barcode2DType = 0
	Barcode2DQRCode Barcode2DType = 1
)

// Select2DBarcodeType selects the symbology printed by
// [Builder.Print2DBarcode] (GS Z n). The default is PDF417.
func (b *Builder) Select2DBarcodeType(t Barcode2DType) error {
	if t > 1 {
		return invalid("GS Z", "type %d is not 0 or 1", t)
	}
	b.cmd(GS, 'Z', byte(t))
	return nil
}

// Print2DBarcode prints a two-dimensional bar code of the type selected with
// [Builder.Select2DBarcodeType] (ESC Z m n k dL dH d1...dn). The meaning of m,
// n and k depends on the type; [Builder.PrintQRCode] and
// [Builder.PrintPDF417] validate them.
func (b *Builder) Print2DBarcode(m, n, k byte, data string) error {
	if len(data) < 1 || len(data) > 0xFFFF {
		return invalid("ESC Z", "data length %d out of range 1–65535", len(data))
	}
	dl, dh := le16(uint16(len(data)))
	b.cmd(ESC, 'Z', m, n, k, dl, dh)
	b.Raw([]byte(data)...)
	return nil
}

// QRErrorCorrection is a QR code error correction level.
//
// The reference names the levels but not their byte values; the ASCII
// letters used here are those used by other printers implementing ESC Z.
type QRErrorCorrection byte

const (
	QRErrorL QRErrorCorrection = 'L' // recovers 7% of data
	QRErrorM QRErrorCorrection = 'M' // recovers 15% of data
	QRErrorQ QRErrorCorrection = 'Q' // recovers 25% of data
	QRErrorH QRErrorCorrection = 'H' // recovers 30% of data
)

// PrintQRCode prints a QR code (GS Z 1, then ESC Z). version is the symbol
// version 1–40, or 0 to choose automatically (recommended); moduleSize is the
// size of one module in dots, 1–8.
func (b *Builder) PrintQRCode(data string, version uint8, ec QRErrorCorrection, moduleSize uint8) error {
	if version > 40 {
		return invalid("ESC Z", "QR version %d out of range 0–40", version)
	}
	if moduleSize < 1 || moduleSize > 8 {
		return invalid("ESC Z", "QR module size %d out of range 1–8", moduleSize)
	}
	if len(data) < 1 || len(data) > 0xFFFF {
		return invalid("ESC Z", "data length %d out of range 1–65535", len(data))
	}
	b.cmd(GS, 'Z', byte(Barcode2DQRCode))
	return b.Print2DBarcode(version, byte(ec), moduleSize, data)
}

// PrintPDF417 prints a PDF417 symbol (GS Z 0, then ESC Z). columns is 1–30,
// security is the error correction level 0–8 and ratio is the
// vertical:horizontal module ratio 2–5. Module width follows
// [Builder.SetBarcodeWidth].
func (b *Builder) PrintPDF417(data string, columns, security, ratio uint8) error {
	if columns < 1 || columns > 30 {
		return invalid("ESC Z", "PDF417 columns %d out of range 1–30", columns)
	}
	if security > 8 {
		return invalid("ESC Z", "PDF417 security level %d out of range 0–8", security)
	}
	if ratio < 2 || ratio > 5 {
		return invalid("ESC Z", "PDF417 ratio %d out of range 2–5", ratio)
	}
	if len(data) < 1 || len(data) > 0xFFFF {
		return invalid("ESC Z", "data length %d out of range 1–65535", len(data))
	}
	b.cmd(GS, 'Z', byte(Barcode2DPDF417))
	return b.Print2DBarcode(columns, security, ratio, data)
}
