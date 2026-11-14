package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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

// modernSampleThreshold is the pixel count above which the quality search for
// WebP/AVIF/JPEG XL runs on a sample mosaic (their encoders are slower).
const modernSampleThreshold = 2_000_000

// useSample reports whether a quality search should run on a sample.
func useSample(w, h, threshold int) bool { return w*h > threshold && min(w, h) >= 512 }

// Compress re-encodes in (PNG, JPEG, WebP, AVIF or JPEG XL) at the smallest
// size that still meets o.MinSSIM. The output keeps the input format unless
// o.Convert is set. It never touches the filesystem. When the result would not
// be smaller, out is exactly in and r.Skipped is SkippedAlreadyOptimal.
func Compress(in []byte, o Options) (out []byte, r Report, err error) {
	r = Report{SizeIn: len(in), Warnings: []string{}}
	f, err := Detect(in)
	if err != nil {
		return nil, r, err
	}
	of := f
	if o.Convert != "" {
		of = o.Convert
	}
	r.FormatIn, r.FormatOut = string(f), string(of)
	if r.Width, r.Height, err = CheckLimits(in, f, o.Limits); err != nil {
		return nil, r, err
	}
	if IsAnimated(in, f) {
		return nil, r, ErrAnimated
	}
	if f == FormatWebP && bytes.Contains(in[:min(len(in), 64)], []byte("VP8L")) {
		// The WebP decoder in use outputs 4:2:0 YUV even for lossless files, so
		// re-encoding would silently degrade them. Leave them alone.
		return nil, r, fmt.Errorf("%w: lossless WebP", ErrUnsupportedFormat)
	}
	img, err := Decode(in, f)
	if err != nil {
		return nil, r, err
	}
	r.Width, r.Height = img.Bounds().Dx(), img.Bounds().Dy()

	var enc []byte
	switch of {
	case FormatPNG:
		enc, err = compressPNG(img, &r, o)
	case FormatJPEG:
		enc, err = compressJPEG(in, f, img, &r, o)
		if err == errAlreadyOptimal {
			return skip(in, r), r.withSkip(), nil
		}
	case FormatWebP, FormatAVIF, FormatJXL:
		enc, err = compressModern(img, of, &r, o)
	default:
		return nil, r, ErrUnsupportedFormat
	}
	if err != nil {
		return nil, r, err
	}
	return finish(in, enc, r, f, of, o)
}

var errAlreadyOptimal = errors.New("already optimal")

func compressPNG(img image.Image, r *Report, o Options) ([]byte, error) {
	nrgba := ToNRGBA(img)
	if o.WhiteBG {
		nrgba = FlattenWhite(nrgba)
	}
	var enc []byte
	var decoded image.Image
	var err error
	if o.Lossless {
		enc, err = pngenc.Encode(pngenc.Lossless(nrgba))
		r.SSIM = 1
	} else {
		p, st := quant.Quantize(nrgba, quant.Options{MaxColors: 256, MinSSIM: o.MinSSIM, MinPSNR: palettePSNRGate, Dither: o.Dither})
		r.Colors, r.SSIM = st.Colors, st.SSIM
		enc, err = pngenc.Encode(p)
		decoded = p
	}
	if err != nil {
		return nil, err
	}
	if o.Zopfli > 0 {
		if z, zerr := pngenc.Recompress(enc, o.Zopfli); zerr == nil {
			enc = z
		}
	}
	if decoded == nil {
		if decoded, err = png.Decode(bytes.NewReader(enc)); err != nil {
			return nil, err
		}
	}
	r.PSNR = metric.PSNR(nrgba, decoded)
	return enc, nil
}

func compressJPEG(in []byte, inFmt Format, img image.Image, r *Report, o Options) ([]byte, error) {
	if HasAlpha(img) { // JPEG has no alpha: composite on white
		img = FlattenWhite(img)
	}
	var enc []byte
	var err error
	q := o.Quality
	// Big images: search the quality on a small mosaic, then encode the full
	// image once. Metrics in the Report then describe the sample.
	var sample image.Image
	if q <= 0 && useSample(r.Width, r.Height, jpegenc.SampleThreshold) {
		sample = jpegenc.Sample(img, 512)
	}
	lowMem := r.Width*r.Height > jpegenc.LowMemPixels
	encodeFull := func(q int) ([]byte, error) {
		if lowMem {
			r.Warnings = append(r.Warnings, "low_memory_mode")
			return jpegenc.EncodeLowMem(img, q)
		}
		return jpegenc.Encode(img, q)
	}
	switch {
	case q > 0:
		enc, err = encodeFull(q)
	case sample != nil:
		if _, q, _, err = jpegenc.EncodeAuto(sample, jpegQLo, jpegQHi, o.MinSSIM); err == nil {
			enc, err = encodeFull(q)
		}
	default:
		enc, q, _, err = jpegenc.EncodeAuto(img, jpegQLo, jpegQHi, o.MinSSIM)
	}
	if err != nil {
		return nil, err
	}
	r.Quality = q
	if o.Quality <= 0 && inFmt == FormatJPEG {
		if est, ok := jpegenc.EstimateQuality(in); ok && est <= q {
			return nil, errAlreadyOptimal
		}
	}
	ref, dec := img, image.Image(nil)
	if sample != nil {
		se, err := jpegenc.Encode(sample, q)
		if err != nil {
			return nil, err
		}
		if dec, err = jpeg.Decode(bytes.NewReader(se)); err != nil {
			return nil, err
		}
		ref = sample
	} else if dec, err = jpeg.Decode(bytes.NewReader(enc)); err != nil {
		return nil, err
	}
	r.SSIM, r.PSNR = metric.SSIM(ref, dec), metric.PSNR(ref, dec)
	return enc, nil
}

func compressModern(img image.Image, of Format, r *Report, o Options) ([]byte, error) {
	if o.WhiteBG && HasAlpha(img) {
		img = FlattenWhite(img)
	}
	if o.Lossless {
		enc, err := encodeModern(img, of, 100, true)
		r.SSIM = 1
		return enc, err
	}
	q := o.Quality
	var sample image.Image
	if q <= 0 && useSample(r.Width, r.Height, modernSampleThreshold) {
		sample = jpegenc.Sample(img, 512)
	}
	if q <= 0 {
		lo, hi := modernRange(of)
		target := img
		if sample != nil {
			target = sample
		}
		var err error
		if q, err = searchQuality(target, of, lo, hi, o.MinSSIM, metric.SSIM); err != nil {
			return nil, err
		}
	}
	r.Quality = q
	enc, err := encodeModern(img, of, q, false)
	if err != nil {
		return nil, err
	}
	ref, probe := img, enc
	if sample != nil {
		ref = sample
		if probe, err = encodeModern(sample, of, q, false); err != nil {
			return nil, err
		}
	}
	dec, err := decodeModern(probe, of)
	if err != nil {
		return nil, err
	}
	r.SSIM, r.PSNR = metric.SSIM(ref, dec), metric.PSNR(ref, dec)
	return enc, nil
}

// finish keeps or drops the ICC profile and applies the size guard.
func finish(in, enc []byte, r Report, f, of Format, o Options) ([]byte, Report, error) {
	if prof := extractICC(in, f); prof != nil {
		switch {
		case isSRGB(prof):
			// sRGB is the default assumption: dropping it changes nothing
		case o.StripICC:
			r.Warnings = append(r.Warnings, "icc_profile_dropped")
		case of == FormatPNG || of == FormatJPEG:
			enc = embedICC(enc, of, prof)
			r.Warnings = append(r.Warnings, "icc_profile_kept")
		default: // WebP/AVIF/JXL writers here cannot carry a profile
			r.Warnings = append(r.Warnings, "icc_profile_dropped")
		}
	} else if hasICC(in, f) || hasModernICC(in, f) {
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
