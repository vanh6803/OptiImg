package metric

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"
)

func gradient(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), uint8((x + y) * 255 / (w + h)), 255})
		}
	}
	return img
}

func TestSSIMIdentical(t *testing.T) {
	a := gradient(64, 64)
	if s := SSIM(a, a); math.Abs(s-1) > 1e-9 {
		t.Fatalf("SSIM identical = %v", s)
	}
	if p := PSNR(a, a); !math.IsInf(p, 1) {
		t.Fatalf("PSNR identical = %v", p)
	}
}

func TestSSIMNoiseLower(t *testing.T) {
	a := gradient(64, 64)
	b := image.NewNRGBA(a.Bounds())
	rng := rand.New(rand.NewSource(1))
	clamp := func(v int) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v)
	}
	for i := 0; i < len(a.Pix); i += 4 {
		n := rng.Intn(41) - 20
		b.Pix[i] = clamp(int(a.Pix[i]) + n)
		b.Pix[i+1] = clamp(int(a.Pix[i+1]) + n)
		b.Pix[i+2] = clamp(int(a.Pix[i+2]) + n)
		b.Pix[i+3] = 255
	}
	s, p := SSIM(a, b), PSNR(a, b)
	if !(s > 0 && s < 0.95) {
		t.Fatalf("SSIM noisy = %v", s)
	}
	if p >= 30 {
		t.Fatalf("PSNR noisy = %v", p)
	}
}

func TestSSIMTinyImage(t *testing.T) {
	a := gradient(3, 3)
	if s := SSIM(a, a); math.Abs(s-1) > 1e-9 {
		t.Fatalf("SSIM tiny = %v", s)
	}
}

func TestSSIMAlphaMatters(t *testing.T) {
	a := gradient(32, 32)
	b := image.NewNRGBA(a.Bounds())
	copy(b.Pix, a.Pix)
	for i := 3; i < len(b.Pix); i += 4 {
		b.Pix[i] = 128
	}
	if s := SSIM(a, b); s >= 0.99 {
		t.Fatalf("SSIM alpha = %v, want < 0.99", s)
	}
}

func TestSizeMismatchPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	SSIM(gradient(8, 8), gradient(9, 8))
}
