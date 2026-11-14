package jpegenc

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"

	"optiimg/core/metric"
)

// IJG standard luminance table (natural order) and the zigzag scan order.
var stdLuma = [64]int{
	16, 11, 10, 16, 24, 40, 51, 61,
	12, 12, 14, 19, 26, 58, 60, 55,
	14, 13, 16, 24, 40, 57, 69, 56,
	14, 17, 22, 29, 51, 87, 80, 62,
	18, 22, 37, 56, 68, 109, 103, 77,
	24, 35, 55, 64, 81, 104, 113, 92,
	49, 64, 78, 87, 103, 121, 120, 101,
	72, 92, 95, 98, 112, 100, 103, 99,
}

var zigzag = [64]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
}

// EstimateQuality estimates the IJG quality of b from its luma quantisation
// table (DQT id 0). ok is false when no usable table is found.
func EstimateQuality(b []byte) (q int, ok bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 0, false
	}
	for p := 2; p+4 <= len(b) && b[p] == 0xFF; {
		m := b[p+1]
		n := int(b[p+2])<<8 | int(b[p+3])
		if n < 2 || p+2+n > len(b) || m == 0xDA {
			return 0, false
		}
		if m == 0xDB {
			seg := b[p+4 : p+2+n]
			for len(seg) > 0 {
				prec, id := seg[0]>>4, seg[0]&0x0F
				size := 64 * (int(prec) + 1)
				if len(seg) < 1+size {
					return 0, false
				}
				if id == 0 {
					var sum float64
					for i := 0; i < 64; i++ {
						var v int
						if prec == 0 {
							v = int(seg[1+i])
						} else {
							v = int(seg[1+2*i])<<8 | int(seg[2+2*i])
						}
						sum += 100 * float64(v) / float64(stdLuma[zigzag[i]])
					}
					s := sum / 64
					var qf float64
					if s <= 100 {
						qf = (200 - s) / 2
					} else {
						qf = 5000 / s
					}
					return int(math.Round(qf)), true
				}
				seg = seg[1+size:]
			}
		}
		p += 2 + n
	}
	return 0, false
}

// EncodeAuto binary-searches q in [lo, hi] for the smallest quality whose
// decoded output reaches minSSIM. If even hi falls short, hi is used.
func EncodeAuto(img image.Image, lo, hi int, minSSIM float64) (out []byte, q int, ssim float64, err error) {
	measure := func(b []byte) (float64, error) {
		dec, err := jpeg.Decode(bytes.NewReader(b))
		if err != nil {
			return 0, err
		}
		return metric.SSIM(img, dec), nil
	}
	var best []byte
	bestSSIM := 0.0
	for hi-lo > 2 {
		mid := (lo + hi) / 2
		b, err := Encode(img, mid)
		if err != nil {
			return nil, 0, 0, err
		}
		s, err := measure(b)
		if err != nil {
			return nil, 0, 0, err
		}
		if s >= minSSIM {
			hi, best, bestSSIM = mid, b, s
		} else {
			lo = mid
		}
	}
	if best == nil {
		b, err := Encode(img, hi)
		if err != nil {
			return nil, 0, 0, err
		}
		s, err := measure(b)
		if err != nil {
			return nil, 0, 0, err
		}
		best, bestSSIM = b, s
	}
	return best, hi, bestSSIM, nil
}
