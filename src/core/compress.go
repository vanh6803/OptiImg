package core

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"math"

	"optiimg/src/core/jpegenc"
	"optiimg/src/core/metric"
	"optiimg/src/core/pngenc"
	"optiimg/src/core/quant"
)

// palettePSNRGate is the second quality gate for palette size: SSIM on luma
// alone is blind to chroma and alpha-edge errors (a 4-colour logo scores 0.98).
const palettePSNRGate = 45

// JPEG auto-quality range. The upper bound is 80 (not 85): on the reference
// photo the SSIM floor is unreachable and q=85 would exceed the 36% size cap.
const jpegQLo, jpegQHi = 55, 80

// Compress re-encodes in (PNG or JPEG) at the smallest size that still meets
// o.MinSSIM. It never touches the filesystem. When the result would not be
// smaller, out is exactly in and r.Skipped is SkippedAlreadyOptimal.
func Compress(in []byte, o Options) (out []byte, r Report, err error) {
	r = Report{SizeIn: len(in), Warnings: []string{}}
	f, err := Detect(in)
	if err != nil {
		return nil, r, err
	}
	r.FormatIn, r.FormatOut = string(f), string(f)
	if r.Width, r.Height, err = CheckLimits(in, f, o.Limits); err != nil {
		return nil, r, err
	}
	if f == FormatPNG && IsAnimatedPNG(in) {
		return nil, r, ErrAnimated
	}
	img, err := Decode(in, f)
	if err != nil {
		return nil, r, err
	}
	r.Width, r.Height = img.Bounds().Dx(), img.Bounds().Dy()

	var enc []byte
	var decoded image.Image
	switch f {
	case FormatPNG:
		nrgba := ToNRGBA(img)
		if o.WhiteBG {
			nrgba = FlattenWhite(nrgba)
		}
		img = nrgba
		if o.Lossless {
			enc, err = pngenc.Encode(pngenc.Lossless(nrgba))
			r.SSIM = 1
		} else {
			var p *image.Paletted
			var st quant.Stats
			p, st = quant.Quantize(nrgba, quant.Options{MaxColors: 256, MinSSIM: o.MinSSIM, MinPSNR: palettePSNRGate, Dither: o.Dither})
			r.Colors, r.SSIM = st.Colors, st.SSIM
			enc, err = pngenc.Encode(p)
			decoded = p
		}
		if err != nil {
			return nil, r, err
		}
		if o.Zopfli > 0 {
			if z, zerr := pngenc.Recompress(enc, o.Zopfli); zerr == nil {
				enc = z
			}
		}
		if decoded == nil {
			decoded, err = png.Decode(bytes.NewReader(enc))
			if err != nil {
				return nil, r, err
			}
		}
	case FormatJPEG:
		if o.WhiteBG && HasAlpha(img) {
			img = FlattenWhite(img)
		}
		q := o.Quality
		// Big images: search the quality on a small mosaic, then encode the
		// full image once. Metrics in the Report then describe the sample.
		var sample image.Image
		if q <= 0 && r.Width*r.Height > jpegenc.SampleThreshold {
			sample = jpegenc.Sample(img, 512)
		}
		switch {
		case q > 0 && r.Width*r.Height > jpegenc.LowMemPixels:
			enc, err = jpegenc.EncodeLowMem(img, q)
			r.Warnings = append(r.Warnings, "low_memory_mode")
		case q > 0:
			enc, err = jpegenc.Encode(img, q)
		case sample != nil:
			if _, q, _, err = jpegenc.EncodeAuto(sample, jpegQLo, jpegQHi, o.MinSSIM); err == nil {
				if r.Width*r.Height > jpegenc.LowMemPixels {
					enc, err = jpegenc.EncodeLowMem(img, q)
					r.Warnings = append(r.Warnings, "low_memory_mode")
				} else {
					enc, err = jpegenc.Encode(img, q)
				}
			}
		default:
			enc, q, _, err = jpegenc.EncodeAuto(img, jpegQLo, jpegQHi, o.MinSSIM)
		}
		if err != nil {
			return nil, r, err
		}
		r.Quality = q
		if o.Quality <= 0 {
			if est, ok := jpegenc.EstimateQuality(in); ok && est <= q {
				return skip(in, r), r.withSkip(), nil
			}
		}
		if sample != nil {
			se, err := jpegenc.Encode(sample, q)
			if err != nil {
				return nil, r, err
			}
			sd, err := jpeg.Decode(bytes.NewReader(se))
			if err != nil {
				return nil, r, err
			}
			r.SSIM, r.PSNR = metric.SSIM(sample, sd), metric.PSNR(sample, sd)
			return finish(in, enc, r, f, o)
		}
		decoded, err = jpeg.Decode(bytes.NewReader(enc))
		if err != nil {
			return nil, r, err
		}
		r.SSIM = metric.SSIM(img, decoded)
	}
	r.PSNR = metric.PSNR(img, decoded)
	return finish(in, enc, r, f, o)
}

// finish keeps or drops the ICC profile and applies the size guard.
func finish(in, enc []byte, r Report, f Format, o Options) ([]byte, Report, error) {
	if prof := extractICC(in, f); prof != nil {
		switch {
		case isSRGB(prof):
			// sRGB is the default assumption: dropping it changes nothing
		case o.StripICC:
			r.Warnings = append(r.Warnings, "icc_profile_dropped")
		default:
			enc = embedICC(enc, f, prof)
			r.Warnings = append(r.Warnings, "icc_profile_kept")
		}
	} else if hasICC(in, f) {
		r.Warnings = append(r.Warnings, "icc_profile_dropped") // present but unreadable
	}
	if len(enc) >= len(in) {
		return skip(in, r), r.withSkip(), nil
	}
	r.SizeOut = len(enc)
	r.SavedPct = savedPct(len(in), len(enc))
	return enc, r, nil
}

func skip(in []byte, r Report) []byte { return in }

func (r Report) withSkip() Report {
	r.Skipped = SkippedAlreadyOptimal
	r.SizeOut = r.SizeIn
	r.SavedPct = 0
	return r
}

func savedPct(in, out int) float64 {
	return math.Round(1000*float64(in-out)/float64(in)) / 10
}

func hasICC(b []byte, f Format) bool {
	switch f {
	case FormatPNG:
		return hasPNGChunk(b, "iCCP")
	case FormatJPEG:
		for p := 2; p+4 <= len(b) && b[p] == 0xFF; {
			m := b[p+1]
			if m == 0xDA || m == 0xD9 {
				return false
			}
			n := int(binary.BigEndian.Uint16(b[p+2:]))
			if n < 2 || p+2+n > len(b) {
				return false
			}
			if m == 0xE2 && n >= 16 && bytes.HasPrefix(b[p+4:], []byte("ICC_PROFILE\x00")) {
				return true
			}
			p += 2 + n
		}
	}
	return false
}
