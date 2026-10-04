package escpos

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

// checker returns a w×h image with black pixels where (x+y) is even.
func checker(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if (x+y)%2 == 0 {
				img.SetGray(x, y, color.Gray{0})
			} else {
				img.SetGray(x, y, color.Gray{255})
			}
		}
	}
	return img
}

func TestBitmapRaster(t *testing.T) {
	bm := NewBitmap(checker(10, 2), ImageOptions{})
	wb, data := bm.Raster()
	if wb != 2 {
		t.Fatalf("width bytes = %d, want 2", wb)
	}
	want := []byte{0xAA, 0x80, 0x55, 0x40}
	if !bytes.Equal(data, want) {
		t.Errorf("got % X, want % X", data, want)
	}
}

func TestBitmapColumns(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 2, 9))
	for y := range 9 {
		img.SetGray(0, y, color.Gray{255})
		img.SetGray(1, y, color.Gray{255})
	}
	img.SetGray(0, 0, color.Gray{0}) // top-left
	img.SetGray(1, 8, color.Gray{0}) // ninth row of column 1
	x, y, data := NewBitmap(img, ImageOptions{}).Columns()
	if x != 1 || y != 2 {
		t.Fatalf("size = %d×%d units, want 1×2", x, y)
	}
	want := make([]byte, 16)
	want[0] = 0x80 // column 0, first byte, top bit
	want[3] = 0x80 // column 1, second byte, top bit
	if !bytes.Equal(data, want) {
		t.Errorf("got % X, want % X", data, want)
	}
	var b Builder
	if err := b.DefineDownloadedBitImage(uint8(x), uint8(y), data); err != nil {
		t.Errorf("columns not accepted by GS *: %v", err)
	}
	if err := b.DefineNVBitImages(NewBitmap(img, ImageOptions{}).NVBitImage()); err != nil {
		t.Errorf("columns not accepted by FS q: %v", err)
	}
}

func TestBitmapTransparentIsWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 1)) // fully transparent black
	_, data := NewBitmap(img, ImageOptions{}).Raster()
	if data[0] != 0 {
		t.Errorf("transparent pixels printed: % X", data)
	}
}

func TestBitmapScalesDown(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 1000, 500))
	bm := NewBitmap(img, ImageOptions{MaxWidth: 576})
	if bm.Width != 576 || bm.Height != 288 {
		t.Errorf("scaled to %d×%d, want 576×288", bm.Width, bm.Height)
	}
}

func TestBitmapDither(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	bm := NewBitmap(img, ImageOptions{Dither: true})
	black := 0
	for y := range bm.Height {
		for x := range bm.Width {
			if bm.At(x, y) {
				black++
			}
		}
	}
	// Mid grey should come out roughly half black.
	if black < 64*64*4/10 || black > 64*64*6/10 {
		t.Errorf("%d of %d dots black for mid grey", black, 64*64)
	}
}

func TestPrintImageBands(t *testing.T) {
	var b Builder
	if err := b.PrintImage(checker(16, 5), ImageOptions{BandHeight: 2}); err != nil {
		t.Fatal(err)
	}
	// Three GS v 0 commands of 2, 2 and 1 rows, 2 bytes wide.
	out := b.Bytes()
	if n := bytes.Count(out, []byte{GS, 'v', '0'}); n != 3 {
		t.Errorf("got %d raster commands, want 3", n)
	}
	if len(out) != 3*8+5*2 {
		t.Errorf("got %d bytes, want %d", len(out), 3*8+5*2)
	}
}

func TestPrintImageAlign(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 16, 1)) // black, 16 dots wide
	for _, tc := range []struct {
		align Align
		width int // raster bytes
		first int // first black byte
	}{{AlignLeft, 2, 0}, {AlignCenter, 72, 35}, {AlignRight, 72, 70}} {
		b := NewBuilder(PaperWidth80mm)
		if err := b.PrintImage(img, ImageOptions{Align: tc.align}); err != nil {
			t.Fatal(err)
		}
		data := b.Bytes()[8:]
		if len(data) != tc.width || data[tc.first] != 0xFF || data[tc.first+1] != 0xFF {
			t.Errorf("align %d: % X", tc.align, data)
		}
	}
}
