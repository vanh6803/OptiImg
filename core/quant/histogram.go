// Package quant reduces an image to a palette: median cut, k-means refinement
// and Floyd–Steinberg dithering, with the palette size chosen by SSIM.
package quant

import "image"

// Colour distance weights (premultiplied R, G, B, A).
const (
	wR = 0.50
	wG = 1.00
	wB = 0.45
	wA = 0.625
)

// col is a premultiplied-alpha colour with channels in 0..255.
type col [4]float32

func dist(a, b col) float32 {
	dr, dg, db, da := a[0]-b[0], a[1]-b[1], a[2]-b[2], a[3]-b[3]
	return wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
}

func premul(r, g, b, a uint8) col {
	if a == 255 {
		return col{float32(r), float32(g), float32(b), 255}
	}
	f := float32(a) / 255
	return col{float32(r) * f, float32(g) * f, float32(b) * f, float32(a)}
}

// bin is one histogram cell: pixel count and mean premultiplied colour.
type bin struct {
	n int
	c col
}

// CountColors counts distinct colours (all a==0 pixels count as one),
// stopping once more than limit are seen.
func CountColors(img *image.NRGBA, limit int) (n int, exceeded bool) {
	seen := make(map[uint32]struct{}, 64)
	for i := 0; i+3 < len(img.Pix); i += 4 {
		var k uint32
		if img.Pix[i+3] != 0 {
			k = uint32(img.Pix[i])<<24 | uint32(img.Pix[i+1])<<16 | uint32(img.Pix[i+2])<<8 | uint32(img.Pix[i+3])
		}
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			if len(seen) > limit {
				return len(seen), true
			}
		}
	}
	return len(seen), false
}

// histogram builds 5-bit-per-channel bins over premultiplied colour. Fully
// transparent pixels are excluded and reported separately.
func histogram(img *image.NRGBA) (bins []bin, hasTransparent bool) {
	type acc struct {
		n          int
		r, g, b, a float64
	}
	m := make(map[uint32]*acc, 4096)
	for i := 0; i+3 < len(img.Pix); i += 4 {
		a := img.Pix[i+3]
		if a == 0 {
			hasTransparent = true
			continue
		}
		p := premul(img.Pix[i], img.Pix[i+1], img.Pix[i+2], a)
		k := uint32(p[0])>>3<<15 | uint32(p[1])>>3<<10 | uint32(p[2])>>3<<5 | uint32(p[3])>>3
		e := m[k]
		if e == nil {
			e = &acc{}
			m[k] = e
		}
		e.n++
		e.r += float64(p[0])
		e.g += float64(p[1])
		e.b += float64(p[2])
		e.a += float64(p[3])
	}
	bins = make([]bin, 0, len(m))
	for _, e := range m {
		f := float64(e.n)
		bins = append(bins, bin{n: e.n, c: col{float32(e.r / f), float32(e.g / f), float32(e.b / f), float32(e.a / f)}})
	}
	// Deterministic order regardless of map iteration.
	sortBins(bins, func(x, y bin) bool {
		for c := 0; c < 4; c++ {
			if x.c[c] != y.c[c] {
				return x.c[c] < y.c[c]
			}
		}
		return false
	})
	return bins, hasTransparent
}

func clamp255(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func lumaOf(c col) float64 {
	return 0.299*float64(c[0]) + 0.587*float64(c[1]) + 0.114*float64(c[2])
}
