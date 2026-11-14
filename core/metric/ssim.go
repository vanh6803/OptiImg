// Package metric implements the image-quality measures used to drive
// compression: SSIM on luma and PSNR on RGB.
package metric

import (
	"image"
	"image/color"
)

const (
	win    = 8
	stride = 4
	c1     = (0.01 * 255) * (0.01 * 255)
	c2     = (0.03 * 255) * (0.03 * 255)
)

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
	case *image.Paletted:
		for _, c := range m.Palette {
			if _, _, _, a := c.RGBA(); a != 0xffff {
				return true // conservative: any translucent entry
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

// readRow composites row y of img over a flat grey background bg (0 or 255)
// into r, g, b (each len == width), using fast paths for the common types.
func readRow(img image.Image, y int, bg float32, r, g, b []float32) {
	bd := img.Bounds()
	w := bd.Dx()
	yy := bd.Min.Y + y
	switch m := img.(type) {
	case *image.NRGBA:
		o := m.PixOffset(bd.Min.X, yy)
		for x := 0; x < w; x++ {
			p := m.Pix[o+4*x : o+4*x+4 : o+4*x+4]
			if p[3] == 255 {
				r[x], g[x], b[x] = float32(p[0]), float32(p[1]), float32(p[2])
				continue
			}
			a := float32(p[3]) / 255
			inv := bg * (1 - a)
			r[x], g[x], b[x] = float32(p[0])*a+inv, float32(p[1])*a+inv, float32(p[2])*a+inv
		}
	case *image.Paletted:
		var lut [256][3]float32
		for i, c := range m.Palette {
			cr, cg, cb, ca := c.RGBA()
			inv := bg * (1 - float32(ca)/65535)
			lut[i] = [3]float32{float32(cr)/257 + inv, float32(cg)/257 + inv, float32(cb)/257 + inv}
		}
		o := m.PixOffset(bd.Min.X, yy)
		for x := 0; x < w; x++ {
			c := lut[m.Pix[o+x]]
			r[x], g[x], b[x] = c[0], c[1], c[2]
		}
	case *image.YCbCr:
		for x := 0; x < w; x++ {
			c := m.YCbCrAt(bd.Min.X+x, yy)
			cr, cg, cb := color.YCbCrToRGB(c.Y, c.Cb, c.Cr)
			r[x], g[x], b[x] = float32(cr), float32(cg), float32(cb)
		}
	case *image.Gray:
		o := m.PixOffset(bd.Min.X, yy)
		for x := 0; x < w; x++ {
			v := float32(m.Pix[o+x])
			r[x], g[x], b[x] = v, v, v
		}
	default:
		for x := 0; x < w; x++ {
			cr, cg, cb, ca := img.At(bd.Min.X+x, yy).RGBA() // premultiplied 16-bit
			inv := bg * (1 - float32(ca)/65535)
			r[x], g[x], b[x] = float32(cr)/257+inv, float32(cg)/257+inv, float32(cb)/257+inv
		}
	}
}

// luma returns the Y plane of img composited over bg.
func luma(img image.Image, bg float32) []float32 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	y := make([]float32, w*h)
	r, g, b := make([]float32, w), make([]float32, w), make([]float32, w)
	for row := 0; row < h; row++ {
		readRow(img, row, bg, r, g, b)
		out := y[row*w : (row+1)*w]
		for x := range out {
			out[x] = 0.299*r[x] + 0.587*g[x] + 0.114*b[x]
		}
	}
	return y
}

func ssimPlane(a, b []float32, w, h int) float64 {
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
					va, vb := float64(a[row+i]), float64(b[row+i])
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
	bgs := []float32{255}
	if hasAlpha(a) || hasAlpha(b) {
		bgs = []float32{255, 0}
	}
	best := 2.0
	for _, bg := range bgs {
		s := ssimPlane(luma(a, bg), luma(b, bg), w, h)
		if s < best {
			best = s
		}
	}
	return best
}
