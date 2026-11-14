package metric

import (
	"image"
	"math"
)

// PSNR returns the peak signal-to-noise ratio in dB over RGB composited on
// white. Identical images yield +Inf. a and b must have the same size.
func PSNR(a, b image.Image) float64 {
	checkSizes(a, b)
	pa, pb := planes(a, 255), planes(b, 255)
	var sum float64
	for i := range pa.r {
		dr, dg, db := pa.r[i]-pb.r[i], pa.g[i]-pb.g[i], pa.b[i]-pb.b[i]
		sum += dr*dr + dg*dg + db*db
	}
	if sum == 0 {
		return math.Inf(1)
	}
	mse := sum / float64(3*len(pa.r))
	return 10 * math.Log10(255*255/mse)
}
