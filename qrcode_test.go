package escpos

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestQRCodeGS(t *testing.T) {
	var b Builder
	if err := b.PrintQRCode("hi", QRErrorQ, 6); err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x1D, 0x28, 0x6B, 4, 0, 49, 65, 50, 0, // model 2
		0x1D, 0x28, 0x6B, 3, 0, 49, 67, 6, // module size 6
		0x1D, 0x28, 0x6B, 3, 0, 49, 69, 50, // error correction Q
		0x1D, 0x28, 0x6B, 5, 0, 49, 80, 48, 'h', 'i', // store 2 bytes
		0x1D, 0x28, 0x6B, 3, 0, 49, 81, 48, // print
	}
	if got := b.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("got  % X\nwant % X", got, want)
	}

	b.Reset()
	if err := b.StoreQRCodeData(strings.Repeat("x", 300)); err != nil {
		t.Fatal(err)
	}
	if got := b.Bytes()[3:5]; !bytes.Equal(got, []byte{0x2F, 0x01}) { // 303 = 0x012F
		t.Errorf("length bytes % X, want 2F 01", got)
	}
}

func TestQRCodeGSValidation(t *testing.T) {
	for name, fn := range map[string]func(b *Builder) error{
		"module":  func(b *Builder) error { return b.PrintQRCode("x", QRErrorL, 17) },
		"ec":      func(b *Builder) error { return b.PrintQRCode("x", 'X', 4) },
		"empty":   func(b *Builder) error { return b.PrintQRCode("", QRErrorL, 4) },
		"long":    func(b *Builder) error { return b.StoreQRCodeData(strings.Repeat("x", MaxQRCodeData+1)) },
		"model":   func(b *Builder) error { return b.SelectQRCodeModel(51) },
		"ec only": func(b *Builder) error { return b.SetQRCodeErrorCorrection(0) },
	} {
		var b Builder
		if err := fn(&b); !errors.Is(err, ErrInvalidArgument) || b.Len() != 0 {
			t.Errorf("%s: got %v with %d bytes written", name, err, b.Len())
		}
	}
}
