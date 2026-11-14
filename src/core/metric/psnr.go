package metric

import (
	"image"
	"math"
)

// PSNR returns the peak signal-to-noise ratio in dB over RGB composited on
// white. Identical images yield +Inf. a and b must have the same size.
func PSNR(a, b image.Image) float64 {
	checkSizes(a, b)
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	ar, ag, ab := make([]float32, w), make([]float32, w), make([]float32, w)
	br, bg, bb := make([]float32, w), make([]float32, w), make([]float32, w)
	var sum float64
	for y := 0; y < h; y++ {
		readRow(a, y, 255, ar, ag, ab)
		readRow(b, y, 255, br, bg, bb)
		for x := 0; x < w; x++ {
			dr, dg, db := float64(ar[x]-br[x]), float64(ag[x]-bg[x]), float64(ab[x]-bb[x])
			sum += dr*dr + dg*dg + db*db
		}
	}
	if sum == 0 {
		return math.Inf(1)
	}
	mse := sum / float64(3*w*h)
	return 10 * math.Log10(255*255/mse)
}
