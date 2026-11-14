package quant

import (
	"image"
	"image/color"
	"math"
	"sort"

	"optiimg/core/metric"
)

// Options control Quantize.
type Options struct {
	MaxColors int
	MinSSIM   float64
	MinPSNR   float64 // optional second gate in dB; 0 = off
	Dither    float64
}

func (o Options) meets(ssim, psnr float64) bool {
	return ssim >= o.MinSSIM && (o.MinPSNR <= 0 || psnr >= o.MinPSNR)
}

// Stats describes the result.
type Stats struct {
	Colors   int
	SSIM     float64
	Lossless bool
}

// buildPalette returns n premultiplied colours (including the transparent
// entry when present), sorted: a<255 first, then by luma. transparent is the
// index of the (0,0,0,0) entry, or -1.
func buildPalette(bins []bin, hasTransparent bool, n int) (pal []col, transparent int) {
	target := n
	if hasTransparent {
		target--
	}
	if target < 1 {
		target = 1
	}
	pal = kmeans(bins, medianCut(bins, target))
	if hasTransparent {
		pal = append(pal, col{})
	}
	return sortPalette(pal)
}

func sortPalette(pal []col) ([]col, int) {
	sort.SliceStable(pal, func(i, j int) bool {
		ti, tj := pal[i][3] < 255, pal[j][3] < 255
		if ti != tj {
			return ti
		}
		return lumaOf(pal[i]) < lumaOf(pal[j])
	})
	transparent := -1
	for i, c := range pal {
		if c[3] == 0 {
			transparent = i
			break
		}
	}
	return pal, transparent
}

func toColor(c col) color.NRGBA {
	if c[3] <= 0 {
		return color.NRGBA{}
	}
	f := 255 / c[3]
	r := func(v float32) uint8 { return uint8(clamp255(float32(math.Round(float64(v * f))))) }
	return color.NRGBA{r(c[0]), r(c[1]), r(c[2]), uint8(clamp255(float32(math.Round(float64(c[3])))))}
}

func paletted(img *image.NRGBA, pal []col, idx []uint8) *image.Paletted {
	cp := make(color.Palette, len(pal))
	for i, c := range pal {
		cp[i] = toColor(c)
	}
	out := image.NewPaletted(img.Bounds(), cp)
	copy(out.Pix, idx)
	return out
}

// downscale box-averages img so the long side is at most maxSide.
func downscale(img *image.NRGBA, maxSide int) *image.NRGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	long := w
	if h > long {
		long = h
	}
	f := (long + maxSide - 1) / maxSide
	if f <= 1 {
		return img
	}
	nw, nh := (w+f-1)/f, (h+f-1)/f
	out := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			var s [4]float64
			var n float64
			for yy := y * f; yy < (y+1)*f && yy < h; yy++ {
				for xx := x * f; xx < (x+1)*f && xx < w; xx++ {
					i := (yy*w + xx) * 4
					p := premul(img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3])
					for c := 0; c < 4; c++ {
						s[c] += float64(p[c])
					}
					n++
				}
			}
			c := toColor(col{float32(s[0] / n), float32(s[1] / n), float32(s[2] / n), float32(s[3] / n)})
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

// Quantize reduces img to a palette of at most o.MaxColors entries, choosing
// the smallest size that still meets o.MinSSIM.
func Quantize(img *image.NRGBA, o Options) (*image.Paletted, Stats) {
	if o.MaxColors < 2 {
		o.MaxColors = 256
	}
	if o.MaxColors > 256 {
		o.MaxColors = 256
	}
	if n, exceeded := CountColors(img, o.MaxColors); !exceeded {
		return exact(img, n)
	}
	bins, hasT := histogram(img)
	thumb := downscale(img, 512)

	lo, hi := 2, o.MaxColors
	for lo < hi {
		mid := (lo + hi) / 2
		pal, tr := buildPalette(bins, hasT, mid)
		idx := remapPlain(thumb, pal, tr)
		tp := paletted(thumb, pal, idx)
		if o.meets(metric.SSIM(thumb, tp), psnrIf(o, thumb, tp)) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	n := lo
	var out *image.Paletted
	var s float64
	for attempt := 0; ; attempt++ {
		pal, tr := buildPalette(bins, hasT, n)
		// The plain remap is cheap and, on SSIM/PSNR, nearly always the better
		// of the two. Dithering (7× slower, noisier) is only worth trying when
		// the plain result already meets the gate, and is kept only if it does too.
		out = paletted(img, pal, remapPlain(img, pal, tr))
		s = metric.SSIM(img, out)
		ok := o.meets(s, psnrIf(o, img, out))
		if ok && o.Dither > 0 {
			d := paletted(img, pal, remapDither(img, pal, tr, float32(o.Dither)))
			if ds := metric.SSIM(img, d); o.meets(ds, psnrIf(o, img, d)) {
				out, s = d, ds
			}
		}
		if ok || attempt == 2 || n >= o.MaxColors {
			break
		}
		n = n * 3 / 2
		if n > o.MaxColors {
			n = o.MaxColors
		}
	}
	return out, Stats{Colors: len(out.Palette), SSIM: s}
}

// exact builds the lossless palette for images with few colours.
func exact(img *image.NRGBA, n int) (*image.Paletted, Stats) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	m := make(map[uint32]int, n)
	var pal []col
	keys := make([]uint32, 0, n)
	key := func(px []uint8) uint32 {
		if px[3] == 0 {
			return 0
		}
		return uint32(px[0])<<24 | uint32(px[1])<<16 | uint32(px[2])<<8 | uint32(px[3])
	}
	raw := make([]int, w*h)
	for p := 0; p < w*h; p++ {
		px := img.Pix[p*4 : p*4+4]
		k := key(px)
		i, ok := m[k]
		if !ok {
			i = len(pal)
			m[k] = i
			keys = append(keys, k)
			if px[3] == 0 {
				pal = append(pal, col{})
			} else {
				pal = append(pal, premul(px[0], px[1], px[2], px[3]))
			}
		}
		raw[p] = i
	}
	// sort and remap indices
	order := make([]int, len(pal))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		pa, pb := pal[order[a]], pal[order[b]]
		ta, tb := pa[3] < 255, pb[3] < 255
		if ta != tb {
			return ta
		}
		return lumaOf(pa) < lumaOf(pb)
	})
	rank := make([]uint8, len(pal))
	sorted := make([]col, len(pal))
	for newI, oldI := range order {
		rank[oldI] = uint8(newI)
		sorted[newI] = pal[oldI]
	}
	idx := make([]uint8, w*h)
	for p, r := range raw {
		idx[p] = rank[r]
	}
	// exact colours: use the original non-premultiplied values
	cp := make(color.Palette, len(pal))
	for newI, oldI := range order {
		k := keys[oldI]
		if k == 0 {
			cp[newI] = color.NRGBA{}
		} else {
			cp[newI] = color.NRGBA{uint8(k >> 24), uint8(k >> 16), uint8(k >> 8), uint8(k)}
		}
	}
	out := image.NewPaletted(img.Bounds(), cp)
	copy(out.Pix, idx)
	return out, Stats{Colors: len(cp), SSIM: 1, Lossless: true}
}

func psnrIf(o Options, a, b image.Image) float64 {
	if o.MinPSNR <= 0 {
		return 0
	}
	return metric.PSNR(a, b)
}
