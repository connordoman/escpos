package main

import (
	"image"
	"math"
)

// The demo draws its images procedurally so it needs no asset files.

type grayImage = image.Gray

// paint returns a w×h image that is black wherever ink(x, y) is true. x and
// y are pixel centres.
func paint(w, h int, ink func(x, y float64) bool) *grayImage {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(255)
			if ink(float64(x)+0.5, float64(y)+0.5) {
				v = 0
			}
			img.Pix[y*img.Stride+x] = v
		}
	}
	return img
}

// logoImage draws a placeholder logo: a ring with a dot beside three bars,
// inside a frame.
func logoImage(w, h int) *grayImage {
	fw, fh := float64(w), float64(h)
	r := fh * 0.38
	cx, cy := fh/2, fh/2
	border := max(2, fh/40)
	return paint(w, h, func(x, y float64) bool {
		if x < border || y < border || x > fw-border || y > fh-border {
			return true
		}
		d := math.Hypot(x-cx, y-cy)
		if (d < r && d > r*0.75) || d < r*0.35 {
			return true
		}
		// Bars like lines of text, each shorter than the last.
		left := fh * 1.05
		for i, length := range []float64{0.9, 0.7, 0.5} {
			top := fh*0.24 + float64(i)*fh*0.2
			if y > top && y < top+fh*0.11 && x > left && x < left+(fw-left-fh*0.15)*length {
				return true
			}
		}
		return false
	})
}

// gradientImage draws a horizontal ramp from black to white.
func gradientImage(w, h int) *grayImage {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Pix[y*img.Stride+x] = uint8(255 * x / max(w-1, 1))
		}
	}
	return img
}

// waveImage draws a sine wave filling the height.
func waveImage(w, h int) *grayImage {
	mid, amp := float64(h)/2, float64(h)/2-3
	return paint(w, h, func(x, y float64) bool {
		return math.Abs(y-(mid+amp*math.Sin(x/16))) < 2
	})
}

// Glyphs are 12×24 dots, the Font A character cell.

func smileyGlyph() *grayImage {
	return paint(12, 24, func(x, y float64) bool {
		d := math.Hypot(x-6, y-12)
		face := d > 4.6 && d < 5.8
		eye := math.Hypot(x-4, y-10.5) < 1.1 || math.Hypot(x-8, y-10.5) < 1.1
		mouth := d > 2.4 && d < 3.4 && y > 12.5
		return face || eye || mouth
	})
}

func heartGlyph() *grayImage {
	return paint(12, 24, func(x, y float64) bool {
		// The classic implicit heart curve, scaled to the cell.
		u, v := (x-6)/5, -(y-12)/5
		a := u*u + v*v - 1
		return a*a*a-u*u*v*v*v < 0
	})
}

func tickGlyph() *grayImage {
	segment := func(x, y, x1, y1, x2, y2 float64) float64 {
		dx, dy := x2-x1, y2-y1
		t := math.Max(0, math.Min(1, ((x-x1)*dx+(y-y1)*dy)/(dx*dx+dy*dy)))
		return math.Hypot(x-(x1+t*dx), y-(y1+t*dy))
	}
	return paint(12, 24, func(x, y float64) bool {
		return segment(x, y, 1.5, 12, 4.5, 16) < 1.3 || segment(x, y, 4.5, 16, 10.5, 7) < 1.3
	})
}
