// Package metric implements the image-quality measures used to drive
// compression: SSIM on luma and PSNR on RGB.
package metric

import "image"

const (
	win    = 8
	stride = 4
	c1     = (0.01 * 255) * (0.01 * 255)
	c2     = (0.03 * 255) * (0.03 * 255)
)

// rgb holds channel planes composited over a flat background.
type rgb struct {
	w, h    int
	r, g, b []float64
}

func checkSizes(a, b image.Image) {
	if a.Bounds().Size() != b.Bounds().Size() {
		panic("metric: image sizes differ")
	}
}

// hasAlpha reports whether any pixel is not fully opaque.
func hasAlpha(img image.Image) bool {
	switch m := img.(type) {
	case *image.NRGBA:
		for i := 3; i < len(m.Pix); i += 4 {
			if m.Pix[i] != 255 {
				return true
			}
		}
		return false
	case *image.RGBA:
		for i := 3; i < len(m.Pix); i += 4 {
			if m.Pix[i] != 255 {
				return true
			}
		}
		return false
	case *image.Gray, *image.YCbCr, *image.Gray16, *image.CMYK:
		return false
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return true
			}
		}
	}
	return false
}

// planes composites img over a flat grey background bg (0 or 255).
func planes(img image.Image, bg float64) *rgb {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	p := &rgb{w: w, h: h, r: make([]float64, w*h), g: make([]float64, w*h), b: make([]float64, w*h)}
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA() // premultiplied 16-bit
			inv := 1 - float64(a)/65535
			p.r[i] = float64(r)/257 + bg*inv
			p.g[i] = float64(g)/257 + bg*inv
			p.b[i] = float64(bl)/257 + bg*inv
			i++
		}
	}
	return p
}

func luma(p *rgb) []float64 {
	y := make([]float64, len(p.r))
	for i := range y {
		y[i] = 0.299*p.r[i] + 0.587*p.g[i] + 0.114*p.b[i]
	}
	return y
}

func ssimPlane(a, b []float64, w, h int) float64 {
	ww, wh, sx, sy := win, win, stride, stride
	if w < win || h < win {
		ww, wh, sx, sy = w, h, w, h // single window over the whole image
	}
	var total float64
	var n int
	for y := 0; y+wh <= h; y += sy {
		for x := 0; x+ww <= w; x += sx {
			var sa, sb, saa, sbb, sab float64
			for j := 0; j < wh; j++ {
				row := (y+j)*w + x
				for i := 0; i < ww; i++ {
					va, vb := a[row+i], b[row+i]
					sa += va
					sb += vb
					saa += va * va
					sbb += vb * vb
					sab += va * vb
				}
			}
			cnt := float64(ww * wh)
			ma, mb := sa/cnt, sb/cnt
			va, vb, cov := saa/cnt-ma*ma, sbb/cnt-mb*mb, sab/cnt-ma*mb
			total += ((2*ma*mb + c1) * (2*cov + c2)) / ((ma*ma + mb*mb + c1) * (va + vb + c2))
			n++
		}
	}
	return total / float64(n)
}

// SSIM returns the mean structural similarity of a and b on luma.
// Images with transparency are measured over white and over black; the lower
// value is returned. a and b must have the same size.
func SSIM(a, b image.Image) float64 {
	checkSizes(a, b)
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	if w == 0 || h == 0 {
		return 1
	}
	bgs := []float64{255}
	if hasAlpha(a) || hasAlpha(b) {
		bgs = []float64{255, 0}
	}
	best := 2.0
	for _, bg := range bgs {
		s := ssimPlane(luma(planes(a, bg)), luma(planes(b, bg)), w, h)
		if s < best {
			best = s
		}
	}
	return best
}
