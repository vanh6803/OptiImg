package core

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"testing"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
)

// crop returns a w×h piece of testdata/photo.jpg as PNG-encodable NRGBA.
func photoCrop(t *testing.T, w, h int) *image.NRGBA {
	t.Helper()
	b, err := os.ReadFile("../../testdata/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return ToNRGBA(img.(*image.YCbCr).SubImage(image.Rect(100, 100, 100+w, 100+h)))
}

func TestDetectModern(t *testing.T) {
	img := photoCrop(t, 64, 64)
	cases := map[Format][]byte{}
	for _, f := range []Format{FormatWebP, FormatAVIF, FormatJXL} {
		b, err := encodeModern(img, f, 70, false)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		cases[f] = b
	}
	for f, b := range cases {
		got, err := Detect(b)
		if err != nil || got != f {
			t.Errorf("Detect(%s) = %q, %v", f, got, err)
		}
		if _, _, err := CheckLimits(b, f, DefaultOptions().Limits); err != nil {
			t.Errorf("CheckLimits(%s): %v", f, err)
		}
	}
}

func TestCompressConvertToModern(t *testing.T) {
	jpgBytes, _ := os.ReadFile("../../testdata/photo.jpg")
	for _, of := range []Format{FormatWebP, FormatAVIF, FormatJXL} {
		o := DefaultOptions()
		o.Convert = of
		out, r, err := Compress(jpgBytes, o)
		if err != nil {
			t.Fatalf("%s: %v", of, err)
		}
		if r.Skipped != "" {
			t.Fatalf("%s: skipped", of)
		}
		got, _ := Detect(out)
		if got != of || r.FormatOut != string(of) || r.FormatIn != "jpeg" {
			t.Errorf("%s: detected %q report %+v", of, got, r)
		}
		if len(out) >= len(jpgBytes) {
			t.Errorf("%s: %d >= source %d", of, len(out), len(jpgBytes))
		}
		if _, err := Decode(out, of); err != nil {
			t.Errorf("%s: output does not decode: %v", of, err)
		}
		t.Logf("jpeg %d -> %s %d (q=%d ssim=%.4f psnr=%.1f)", len(jpgBytes), of, len(out), r.Quality, r.SSIM, r.PSNR)
	}
}

func TestCompressModernInPlace(t *testing.T) {
	src := photoCrop(t, 400, 300)
	for _, f := range []Format{FormatWebP, FormatAVIF, FormatJXL} {
		in, err := encodeModern(src, f, 95, false) // deliberately high quality
		if err != nil {
			t.Fatal(err)
		}
		out, r, err := Compress(in, DefaultOptions())
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if r.FormatIn != string(f) || r.FormatOut != string(f) {
			t.Errorf("%s: formats %+v", f, r)
		}
		if got, _ := Detect(out); got != f {
			t.Errorf("%s: output is %q", f, got)
		}
		if len(out) > len(in) {
			t.Errorf("%s: grew %d -> %d", f, len(in), len(out))
		}
	}
}

func TestCompressConvertAlphaToJPEGFlattens(t *testing.T) {
	src := photoCrop(t, 300, 300)
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = uint8(60 + (i/4)%190) // translucent everywhere
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	o := DefaultOptions()
	o.Convert = FormatJPEG
	out, r, err := Compress(buf.Bytes(), o)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Detect(out); got != FormatJPEG || r.Skipped != "" {
		t.Fatalf("output is %q (skipped %q)", got, r.Skipped)
	}
}

func TestCompressLosslessWebPRefused(t *testing.T) {
	in, err := encodeModern(photoCrop(t, 64, 64), FormatWebP, 100, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Compress(in, DefaultOptions()); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("err = %v", err)
	}
}

func TestCompressAnimatedWebPRejected(t *testing.T) {
	a := photoCrop(t, 64, 64)
	b := ToNRGBA(a.SubImage(image.Rect(10, 10, 54, 54)))
	b = scaleTo(b, 64)
	var buf bytes.Buffer
	if err := webp.EncodeAll(&buf, &webp.WEBP{Image: []image.Image{a, b}, Delay: []int{100, 100}}); err != nil {
		t.Skipf("cannot build animated webp: %v", err)
	}
	if _, _, err := Compress(buf.Bytes(), DefaultOptions()); !errors.Is(err, ErrAnimated) {
		t.Errorf("err = %v", err)
	}
}

func TestCompressAnimatedAVIFRejected(t *testing.T) {
	a := photoCrop(t, 64, 64)
	b := ToNRGBA(a.SubImage(image.Rect(10, 10, 54, 54)))
	b = scaleTo(b, 64)
	var buf bytes.Buffer
	if err := avif.EncodeAll(&buf, &avif.AVIF{Image: []image.Image{a, b}, Delay: []float64{0.1, 0.1}}); err != nil {
		t.Skipf("cannot build animated avif: %v", err)
	}
	if _, _, err := Compress(buf.Bytes(), DefaultOptions()); !errors.Is(err, ErrAnimated) {
		t.Errorf("err = %v", err)
	}
}

func scaleTo(src *image.NRGBA, n int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			out.SetNRGBA(x, y, src.NRGBAAt(x*src.Bounds().Dx()/n, y*src.Bounds().Dy()/n))
		}
	}
	return out
}
