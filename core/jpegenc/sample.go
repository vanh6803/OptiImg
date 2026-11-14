package jpegenc

import (
	"image"
	"image/draw"
)

// SampleThreshold is the pixel count above which quality is searched on a
// sample instead of the whole image.
const SampleThreshold = 4_000_000

// Sample builds a 2×2 mosaic of tile×tile blocks taken around the four
// quarter points of img. It is cheap to encode repeatedly and, unlike a
// single centre crop, sees several parts of the picture. Tiles are aligned
// to the 16-pixel MCU grid. img must be at least 2·tile on both sides.
func Sample(img image.Image, tile int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, 2*tile, 2*tile))
	pos := func(extent, frac int) int { // frac in {1,3}: quarter points
		p := extent*frac/4 - tile/2
		if p < 0 {
			p = 0
		}
		if p > extent-tile {
			p = extent - tile
		}
		return p &^ 15
	}
	for ty := 0; ty < 2; ty++ {
		for tx := 0; tx < 2; tx++ {
			sx, sy := b.Min.X+pos(w, 1+2*tx), b.Min.Y+pos(h, 1+2*ty)
			r := image.Rect(tx*tile, ty*tile, (tx+1)*tile, (ty+1)*tile)
			draw.Draw(dst, r, img, image.Pt(sx, sy), draw.Src)
		}
	}
	return dst
}
