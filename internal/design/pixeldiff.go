package design

import (
	"fmt"
	"image"
	"image/draw"
)

// DefaultThreshold is the per-pixel colour distance (0 to 1, in YIQ space)
// below which two pixels count as equal.
const DefaultThreshold = 0.1

// PixelDiff is the result of comparing two images of the same size.
type PixelDiff struct {
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	Different   int     `json:"different"`    // pixels that differ beyond the threshold and are not anti-aliasing
	AntiAliased int     `json:"anti_aliased"` // differing pixels the anti-aliasing mask removed
	Percent     float64 `json:"percent"`      // Different as a percentage of all pixels
}

// Diff compares two screenshots pixel by pixel. A pixel differs when its
// perceptual (YIQ) distance exceeds threshold; differing pixels that look like
// anti-aliasing in either image (an edge pixel between a darker and a brighter
// neighbour whose surroundings are flat in both images) are masked out and
// counted apart. The method follows Vysniauskas, "Anti-aliased Pixel and
// Intensity Slope Detector" (2009), as popularised by pixelmatch; this is an
// independent implementation.
func Diff(a, b image.Image, threshold float64) (PixelDiff, error) {
	ba, bb := a.Bounds(), b.Bounds()
	if ba.Dx() != bb.Dx() || ba.Dy() != bb.Dy() {
		return PixelDiff{}, fmt.Errorf("sizes differ: %dx%d and %dx%d", ba.Dx(), ba.Dy(), bb.Dx(), bb.Dy())
	}
	p, q := nrgba(a), nrgba(b)
	w, h := ba.Dx(), ba.Dy()
	res := PixelDiff{Width: w, Height: h}
	maxDelta := 35215 * threshold * threshold
	for y := range h {
		for x := range w {
			i := y*p.Stride + x*4
			j := y*q.Stride + x*4
			if p.Pix[i] == q.Pix[j] && p.Pix[i+1] == q.Pix[j+1] && p.Pix[i+2] == q.Pix[j+2] && p.Pix[i+3] == q.Pix[j+3] {
				continue
			}
			if colorDelta(p.Pix[i:i+4], q.Pix[j:j+4], false) <= maxDelta {
				continue
			}
			if antialiased(p, x, y, q) || antialiased(q, x, y, p) {
				res.AntiAliased++
			} else {
				res.Different++
			}
		}
	}
	if w*h > 0 {
		res.Percent = 100 * float64(res.Different) / float64(w*h)
	}
	return res, nil
}

func nrgba(m image.Image) *image.NRGBA {
	if n, ok := m.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := m.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Rect, m, b.Min, draw.Src)
	return n
}

// colorDelta is the squared YIQ distance of two NRGBA pixels after blending
// each with white by its alpha, or only the signed brightness difference.
func colorDelta(c1, c2 []uint8, yOnly bool) float64 {
	r1, g1, b1 := blend(c1)
	r2, g2, b2 := blend(c2)
	y := rgb2y(r1, g1, b1) - rgb2y(r2, g2, b2)
	if yOnly {
		return y
	}
	i := rgb2i(r1, g1, b1) - rgb2i(r2, g2, b2)
	q := rgb2q(r1, g1, b1) - rgb2q(r2, g2, b2)
	return 0.5053*y*y + 0.299*i*i + 0.1957*q*q
}

func blend(c []uint8) (r, g, b float64) {
	a := float64(c[3]) / 255
	f := func(v uint8) float64 { return 255 + (float64(v)-255)*a }
	return f(c[0]), f(c[1]), f(c[2])
}

func rgb2y(r, g, b float64) float64 { return r*0.29889531 + g*0.58662247 + b*0.11448223 }
func rgb2i(r, g, b float64) float64 { return r*0.59597799 - g*0.27417610 - b*0.32180189 }
func rgb2q(r, g, b float64) float64 { return r*0.21147017 - g*0.52261711 + b*0.31114694 }

func px(m *image.NRGBA, x, y int) []uint8 {
	i := y*m.Stride + x*4
	return m.Pix[i : i+4]
}

// antialiased reports whether the pixel at (x1, y1) of m looks like
// anti-aliasing: at most two neighbours share its brightness, it has both a
// darker and a brighter neighbour, and the darkest or brightest neighbour sits
// in a flat area in both images.
func antialiased(m *image.NRGBA, x1, y1 int, other *image.NRGBA) bool {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	x0, y0, x2, y2 := max(x1-1, 0), max(y1-1, 0), min(x1+1, w-1), min(y1+1, h-1)
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}
	var lo, hi float64
	var loX, loY, hiX, hiY int
	c := px(m, x1, y1)
	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			d := colorDelta(c, px(m, x, y), true)
			switch {
			case d == 0:
				zeroes++
				if zeroes > 2 {
					return false
				}
			case d < lo:
				lo, loX, loY = d, x, y
			case d > hi:
				hi, hiX, hiY = d, x, y
			}
		}
	}
	if lo == 0 || hi == 0 {
		return false
	}
	return (manySiblings(m, loX, loY) && manySiblings(other, loX, loY)) ||
		(manySiblings(m, hiX, hiY) && manySiblings(other, hiX, hiY))
}

// manySiblings reports whether more than two neighbours of (x1, y1) have
// exactly its colour.
func manySiblings(m *image.NRGBA, x1, y1 int) bool {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	x0, y0, x2, y2 := max(x1-1, 0), max(y1-1, 0), min(x1+1, w-1), min(y1+1, h-1)
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}
	c := px(m, x1, y1)
	for x := x0; x <= x2; x++ {
		for y := y0; y <= y2; y++ {
			if x == x1 && y == y1 {
				continue
			}
			n := px(m, x, y)
			if c[0] == n[0] && c[1] == n[1] && c[2] == n[2] && c[3] == n[3] {
				zeroes++
			}
			if zeroes > 2 {
				return true
			}
		}
	}
	return false
}
