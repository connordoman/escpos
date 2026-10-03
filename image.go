package escpos

import (
	"image"
)

// BitImageMode is a density mode for [Builder.PrintBitImage].
type BitImageMode byte

const (
	BitImage8DotSingle  BitImageMode = 0  // 8 dots tall, half horizontal density
	BitImage8DotDouble  BitImageMode = 1  // 8 dots tall, full horizontal density
	BitImage24DotSingle BitImageMode = 32 // 24 dots tall, half horizontal density
	BitImage24DotDouble BitImageMode = 33 // 24 dots tall, full horizontal density
)

// bytesPerColumn returns how many data bytes make up one column, or 0 if the
// mode is invalid.
func (m BitImageMode) bytesPerColumn() int {
	switch m {
	case BitImage8DotSingle, BitImage8DotDouble:
		return 1
	case BitImage24DotSingle, BitImage24DotDouble:
		return 3
	}
	return 0
}

// PrintBitImage prints one band of a column-format bit image (ESC * m nL nH
// d1...dk). data holds one byte per column for 8-dot modes or three bytes per
// column (top to bottom) for 24-dot modes, most significant bit at the top.
// The image is at most 1023 columns wide.
func (b *Builder) PrintBitImage(mode BitImageMode, data []byte) error {
	k := mode.bytesPerColumn()
	if k == 0 {
		return invalid("ESC *", "mode %d is not 0, 1, 32 or 33", mode)
	}
	if len(data) == 0 || len(data)%k != 0 {
		return invalid("ESC *", "data length %d is not a positive multiple of %d", len(data), k)
	}
	cols := len(data) / k
	if cols > 1023 {
		return invalid("ESC *", "%d columns exceeds 1023", cols)
	}
	l, h := le16(uint16(cols))
	b.cmd(ESC, '*', byte(mode), l, h)
	b.cmd(data...)
	return nil
}

// ImageScale selects the dot density used to print downloaded, NV and raster
// bit images.
type ImageScale byte

const (
	ScaleNormal       ImageScale = 0
	ScaleDoubleWidth  ImageScale = 1
	ScaleDoubleHeight ImageScale = 2
	ScaleQuadruple    ImageScale = 3
)

func (s ImageScale) valid() bool { return s <= 3 || (s >= 48 && s <= 51) }

// DefineDownloadedBitImage defines the downloaded bit image (GS * x y d). The
// image is x×8 dots wide and y×8 dots tall, with 1 ≤ x ≤ 255, 1 ≤ y ≤ 48 and
// x×y ≤ 1536. data is in column format: x×8 columns of y bytes each, top to
// bottom, as produced by [Bitmap.Columns]. Defining it clears user-defined
// characters.
func (b *Builder) DefineDownloadedBitImage(x, y uint8, data []byte) error {
	if x < 1 || y < 1 || y > 48 || int(x)*int(y) > 1536 {
		return invalid("GS *", "size x=%d y=%d out of range", x, y)
	}
	if want := int(x) * int(y) * 8; len(data) != want {
		return invalid("GS *", "got %d bytes, want %d", len(data), want)
	}
	b.cmd(GS, '*', x, y)
	b.cmd(data...)
	return nil
}

// PrintDownloadedBitImage prints the image defined by
// [Builder.DefineDownloadedBitImage] (GS / m).
func (b *Builder) PrintDownloadedBitImage(s ImageScale) error {
	if !s.valid() {
		return invalid("GS /", "scale %d out of range 0–3", s)
	}
	b.cmd(GS, '/', byte(s))
	return nil
}

// PrintRasterBitImage prints a raster bit image (GS v 0 m xL xH yL yH d).
// data is row-major with widthBytes bytes (8 dots each, most significant bit
// leftmost) per row, as produced by [Bitmap.Raster]. widthBytes must be
// within 1–128 and the image at most 4095 rows tall.
func (b *Builder) PrintRasterBitImage(s ImageScale, widthBytes int, data []byte) error {
	if !s.valid() {
		return invalid("GS v 0", "scale %d out of range 0–3", s)
	}
	if widthBytes < 1 || widthBytes > 128 {
		return invalid("GS v 0", "width %d bytes out of range 1–128", widthBytes)
	}
	if len(data) == 0 || len(data)%widthBytes != 0 {
		return invalid("GS v 0", "data length %d is not a positive multiple of %d", len(data), widthBytes)
	}
	rows := len(data) / widthBytes
	if rows > 4095 {
		return invalid("GS v 0", "%d rows exceeds 4095", rows)
	}
	xl, xh := le16(uint16(widthBytes))
	yl, yh := le16(uint16(rows))
	b.cmd(GS, 'v', '0', byte(s), xl, xh, yl, yh)
	b.cmd(data...)
	return nil
}

// PrintImage converts img to black and white and prints it with raster bit
// image commands, splitting tall images into bands. Images wider than
// opts.MaxWidth (default: the paper width) are scaled down to fit.
func (b *Builder) PrintImage(img image.Image, opts ImageOptions) error {
	if opts.MaxWidth == 0 {
		opts.MaxWidth = b.PaperWidth()
	}
	if !opts.Scale.valid() {
		return invalid("GS v 0", "scale %d out of range 0–3", opts.Scale)
	}
	bm := NewBitmap(img, opts)
	if bm.Width == 0 || bm.Height == 0 {
		return invalid("GS v 0", "image is empty")
	}
	wb, data := bm.Raster()
	if wb > 128 {
		return invalid("GS v 0", "image is %d dots wide, maximum is 1024", bm.Width)
	}
	band := opts.BandHeight
	if band <= 0 || band > 4095 {
		band = 4095
	}
	for off := 0; off < len(data); off += band * wb {
		end := min(off+band*wb, len(data))
		if err := b.PrintRasterBitImage(opts.Scale, wb, data[off:end]); err != nil {
			return err
		}
	}
	return nil
}

// PrintNVBitImage prints NV bit image n, numbered from 1 in the order they
// were defined with [Builder.DefineNVBitImages] (FS p n m).
func (b *Builder) PrintNVBitImage(n uint8, s ImageScale) error {
	if n < 1 {
		return invalid("FS p", "image number must be at least 1")
	}
	if !s.valid() {
		return invalid("FS p", "scale %d out of range 0–3", s)
	}
	b.cmd(FS, 'p', n, byte(s))
	return nil
}

// NVBitImage is one image for [Builder.DefineNVBitImages].
type NVBitImage struct {
	X    int    // width in units of 8 dots, 1–1023
	Y    int    // height in units of 8 dots, 1–288
	Data []byte // X×8 columns of Y bytes each, as produced by [Bitmap.Columns]
}

// NVBitImageCapacity is the size of the printer's NV bit image area in bytes,
// including a 4-byte header per image.
const NVBitImageCapacity = 192 * 1024

// DefineNVBitImages stores images in non-volatile memory, replacing all
// previously defined NV images (FS q n [xL xH yL yH d1...dk]...). Images are
// numbered from 1 in the order given.
//
// NV memory wears out: the reference recommends at most 10 writes a day. The
// printer is busy while writing and must not be sent anything, including
// real-time commands, until it has finished.
func (b *Builder) DefineNVBitImages(images ...NVBitImage) error {
	if len(images) < 1 || len(images) > 255 {
		return invalid("FS q", "%d images given, want 1–255", len(images))
	}
	total := 0
	for i, img := range images {
		if img.X < 1 || img.X > 1023 || img.Y < 1 || img.Y > 288 {
			return invalid("FS q", "image %d size x=%d y=%d out of range", i+1, img.X, img.Y)
		}
		if want := img.X * img.Y * 8; len(img.Data) != want {
			return invalid("FS q", "image %d has %d bytes, want %d", i+1, len(img.Data), want)
		}
		total += len(img.Data) + 4
	}
	if total > NVBitImageCapacity {
		return invalid("FS q", "images need %d bytes, capacity is %d", total, NVBitImageCapacity)
	}
	b.cmd(FS, 'q', byte(len(images)))
	for _, img := range images {
		xl, xh := le16(uint16(img.X))
		yl, yh := le16(uint16(img.Y))
		b.cmd(xl, xh, yl, yh)
		b.cmd(img.Data...)
	}
	return nil
}

// ImageOptions controls how images are converted to black and white.
type ImageOptions struct {
	// MaxWidth scales images wider than this many dots down to fit,
	// preserving the aspect ratio. Zero means no limit, except in
	// [Builder.PrintImage] where it defaults to the paper width.
	MaxWidth int

	// Dither enables Floyd–Steinberg dithering, which suits photographs.
	// Without it each pixel is thresholded, which suits logos and text.
	Dither bool

	// Threshold is the luminance (0–255) below which a pixel prints black.
	// Zero means 128. It is ignored when dithering.
	Threshold uint8

	// Scale is the dot density used by [Builder.PrintImage].
	Scale ImageScale

	// BandHeight is the maximum number of rows sent per raster command by
	// [Builder.PrintImage]. Zero or values above 4095 mean 4095.
	BandHeight int
}

// Bitmap is a black and white image.
type Bitmap struct {
	Width, Height int
	pix           []bool // row-major, true prints a dot
}

// NewBitmap converts img to black and white. Transparent pixels are treated
// as white.
func NewBitmap(img image.Image, opts ImageOptions) *Bitmap {
	r := img.Bounds()
	w, h := r.Dx(), r.Dy()
	if w <= 0 || h <= 0 {
		return &Bitmap{}
	}
	dw, dh := w, h
	if opts.MaxWidth > 0 && w > opts.MaxWidth {
		dw = opts.MaxWidth
		dh = max(1, h*dw/w)
	}

	// Box-filter the source into a luminance grid of the target size.
	gray := make([]float32, dw*dh)
	for y := range dh {
		y0, y1 := r.Min.Y+y*h/dh, r.Min.Y+max((y+1)*h/dh, y*h/dh+1)
		for x := range dw {
			x0, x1 := r.Min.X+x*w/dw, r.Min.X+max((x+1)*w/dw, x*w/dw+1)
			var sum float32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					sum += luminance(img.At(sx, sy).RGBA())
				}
			}
			gray[y*dw+x] = sum / float32((y1-y0)*(x1-x0))
		}
	}

	bm := &Bitmap{Width: dw, Height: dh, pix: make([]bool, dw*dh)}
	if opts.Dither {
		for y := range dh {
			for x := range dw {
				i := y*dw + x
				old := gray[i]
				black := old < 128
				bm.pix[i] = black
				var e float32
				if black {
					e = old
				} else {
					e = old - 255
				}
				if x+1 < dw {
					gray[i+1] += e * 7 / 16
				}
				if y+1 < dh {
					if x > 0 {
						gray[i+dw-1] += e * 3 / 16
					}
					gray[i+dw] += e * 5 / 16
					if x+1 < dw {
						gray[i+dw+1] += e * 1 / 16
					}
				}
			}
		}
		return bm
	}
	t := float32(opts.Threshold)
	if t == 0 {
		t = 128
	}
	for i, g := range gray {
		bm.pix[i] = g < t
	}
	return bm
}

// luminance returns the brightness (0–255) of a colour composited on white.
func luminance(r, g, b, a uint32) float32 {
	// Colours are alpha-premultiplied, so adding the uncovered fraction of
	// white composites them onto a white background.
	white := float32(0xffff - a)
	y := 0.299*(float32(r)+white) + 0.587*(float32(g)+white) + 0.114*(float32(b)+white)
	return y / 0xffff * 255
}

// At reports whether the dot at (x, y) prints.
func (m *Bitmap) At(x, y int) bool {
	if x < 0 || y < 0 || x >= m.Width || y >= m.Height {
		return false
	}
	return m.pix[y*m.Width+x]
}

// Raster returns the bitmap in raster format for
// [Builder.PrintRasterBitImage]: rows of widthBytes bytes, the most
// significant bit leftmost, padded with white on the right.
func (m *Bitmap) Raster() (widthBytes int, data []byte) {
	widthBytes = (m.Width + 7) / 8
	data = make([]byte, widthBytes*m.Height)
	for y := range m.Height {
		for x := range m.Width {
			if m.pix[y*m.Width+x] {
				data[y*widthBytes+x/8] |= 0x80 >> (x % 8)
			}
		}
	}
	return widthBytes, data
}

// Columns returns the bitmap in column format for
// [Builder.DefineDownloadedBitImage] and [Builder.DefineNVBitImages]. The
// bitmap is padded with white to x×8 by y×8 dots; data holds x×8 columns of y
// bytes, top to bottom, the most significant bit at the top.
func (m *Bitmap) Columns() (x, y int, data []byte) {
	x, y = (m.Width+7)/8, (m.Height+7)/8
	data = make([]byte, x*8*y)
	for col := range m.Width {
		for row := range m.Height {
			if m.pix[row*m.Width+col] {
				data[col*y+row/8] |= 0x80 >> (row % 8)
			}
		}
	}
	return x, y, data
}

// NVBitImage returns the bitmap as an image for [Builder.DefineNVBitImages].
func (m *Bitmap) NVBitImage() NVBitImage {
	x, y, data := m.Columns()
	return NVBitImage{X: x, Y: y, Data: data}
}
