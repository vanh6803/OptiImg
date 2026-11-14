package core

import (
	"bytes"
	"image"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/jpegxl"
	"github.com/gen2brain/webp"
)

// Quality search ranges for the modern codecs, chosen so that the top of the
// range stays within the size targets (see compression-logic.md §4).
func modernRange(f Format) (lo, hi int) {
	switch f {
	case FormatAVIF:
		return 40, 70
	}
	return 50, 80 // WebP, JPEG XL
}

func decodeConfigModern(b []byte, f Format) (image.Config, error) {
	r := bytes.NewReader(b)
	switch f {
	case FormatWebP:
		return webp.DecodeConfig(r)
	case FormatAVIF:
		return avif.DecodeConfig(r)
	}
	return jpegxl.DecodeConfig(r)
}

func decodeModern(b []byte, f Format) (image.Image, error) {
	r := bytes.NewReader(b)
	switch f {
	case FormatWebP:
		return webp.Decode(r)
	case FormatAVIF:
		return avif.Decode(r)
	}
	return jpegxl.Decode(r)
}

// encodeModern writes img as WebP, AVIF or JPEG XL. Speed settings favour
// throughput: WebP method 4, AVIF speed 8, JPEG XL effort 5 (measured on a 4K
// photo: 2.3 s, 6 s and 8.8 s, within 1% of the slowest settings' size).
func encodeModern(img image.Image, f Format, q int, lossless bool) ([]byte, error) {
	var buf bytes.Buffer
	var err error
	switch f {
	case FormatWebP:
		err = webp.Encode(&buf, img, webp.Options{Quality: q, Lossless: lossless, Method: 4})
	case FormatAVIF:
		err = avif.Encode(&buf, img, avif.Options{Quality: q, QualityAlpha: q, Speed: 8, Lossless: lossless})
	default:
		err = jpegxl.Encode(&buf, img, jpegxl.Options{Quality: q, Effort: 5, Lossless: lossless})
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// IsAnimated reports whether b holds an animation Compress would destroy.
func IsAnimated(b []byte, f Format) bool {
	switch f {
	case FormatPNG:
		return IsAnimatedPNG(b)
	case FormatWebP:
		return bytes.Contains(b[:min(len(b), 1<<16)], []byte("ANIM"))
	case FormatAVIF:
		return bytes.Contains(b[:min(len(b), 64)], []byte("avis"))
	}
	return false
}

// hasModernICC reports whether a WebP/AVIF file carries a colour profile that
// re-encoding with these libraries drops.
func hasModernICC(b []byte, f Format) bool {
	head := b[:min(len(b), 1<<16)]
	switch f {
	case FormatWebP:
		return bytes.Contains(head, []byte("ICCP"))
	case FormatAVIF:
		return bytes.Contains(head, []byte("colrprof"))
	}
	return false
}

// searchQuality binary-searches the lowest quality in [lo,hi] whose decoded
// output reaches minSSIM on img; if none does, hi is returned.
func searchQuality(img image.Image, f Format, lo, hi int, minSSIM float64, ssim func(a, b image.Image) float64) (int, error) {
	for hi-lo > 2 {
		mid := (lo + hi) / 2
		b, err := encodeModern(img, f, mid, false)
		if err != nil {
			return 0, err
		}
		d, err := decodeModern(b, f)
		if err != nil {
			return 0, err
		}
		if ssim(img, d) >= minSSIM {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi, nil
}
