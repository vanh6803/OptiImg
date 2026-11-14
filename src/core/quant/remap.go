package quant

import "image"

// remapPlain maps every pixel to its nearest palette entry, without dither.
func remapPlain(img *image.NRGBA, pal []col, transparent int) []uint8 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	idx := make([]uint8, w*h)
	cache := make(map[uint32]uint8, 4096)
	for p := 0; p < w*h; p++ {
		px := img.Pix[p*4 : p*4+4]
		if px[3] == 0 && transparent >= 0 {
			idx[p] = uint8(transparent)
			continue
		}
		k := uint32(px[0])<<24 | uint32(px[1])<<16 | uint32(px[2])<<8 | uint32(px[3])
		v, ok := cache[k]
		if !ok {
			i, _ := nearest(pal, premul(px[0], px[1], px[2], px[3]))
			v = uint8(i)
			cache[k] = v
		}
		idx[p] = v
	}
	return idx
}

// remapDither maps pixels with Floyd–Steinberg error diffusion scaled by
// strength. Fully transparent pixels never take part in diffusion.
func remapDither(img *image.NRGBA, pal []col, transparent int, strength float32) []uint8 {
	if strength <= 0 {
		return remapPlain(img, pal, transparent)
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	idx := make([]uint8, w*h)
	cache := make(map[uint32]uint8, 1<<14)
	// nearestCached looks up the palette entry for a 6-bit-per-channel bucket
	// (alpha exact): brute force only once per bucket instead of per pixel.
	nearestCached := func(c col) int {
		r, g, b, a := uint32(c[0])>>2, uint32(c[1])>>2, uint32(c[2])>>2, uint32(c[3]+0.5)
		k := r<<20 | g<<14 | b<<8 | a
		if v, ok := cache[k]; ok {
			return int(v)
		}
		ctr := col{float32(r*4 + 2), float32(g*4 + 2), float32(b*4 + 2), float32(a)}
		for i := 0; i < 3; i++ {
			if ctr[i] > ctr[3] {
				ctr[i] = ctr[3]
			}
		}
		i, _ := nearest(pal, ctr)
		cache[k] = uint8(i)
		return i
	}
	cur := make([]col, w+2)
	next := make([]col, w+2)
	for y := 0; y < h; y++ {
		for i := range next {
			next[i] = col{}
		}
		for x := 0; x < w; x++ {
			p := y*w + x
			px := img.Pix[p*4 : p*4+4]
			if px[3] == 0 && transparent >= 0 {
				idx[p] = uint8(transparent)
				continue
			}
			c := premul(px[0], px[1], px[2], px[3])
			e := cur[x+1]
			for k := 0; k < 4; k++ {
				c[k] = clamp255(c[k] + e[k])
			}
			// alpha must not exceed what premultiplied colour allows
			for k := 0; k < 3; k++ {
				if c[k] > c[3] {
					c[k] = c[3]
				}
			}
			i := nearestCached(c)
			idx[p] = uint8(i)
			var err col
			for k := 0; k < 4; k++ {
				err[k] = (c[k] - pal[i][k]) * strength
			}
			for k := 0; k < 4; k++ {
				cur[x+2][k] += err[k] * 7 / 16
				next[x][k] += err[k] * 3 / 16
				next[x+1][k] += err[k] * 5 / 16
				next[x+2][k] += err[k] * 1 / 16
			}
		}
		cur, next = next, cur
	}
	return idx
}
