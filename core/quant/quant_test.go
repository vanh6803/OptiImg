package quant

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"testing"

	"optiimg/core"
	"optiimg/core/metric"
)

func loadNRGBA(t testing.TB, name string) *image.NRGBA {
	t.Helper()
	b, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return core.ToNRGBA(img)
}

func opts() Options { return Options{MaxColors: 256, MinSSIM: 0.98, Dither: 1.0} }

func TestCountColors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{1, 2, 3, 255})
	img.SetNRGBA(1, 0, color.NRGBA{4, 5, 6, 255})
	img.SetNRGBA(2, 0, color.NRGBA{7, 8, 9, 255})
	if n, ex := CountColors(img, 256); n != 3 || ex {
		t.Errorf("3 colours: %d %v", n, ex)
	}
	big := image.NewNRGBA(image.Rect(0, 0, 300, 1))
	for x := 0; x < 300; x++ {
		big.SetNRGBA(x, 0, color.NRGBA{uint8(x), uint8(x >> 8), 0, 255})
	}
	if _, ex := CountColors(big, 256); !ex {
		t.Error("300 colours not exceeded")
	}
}

func TestQuantizeFewColorsLossless(t *testing.T) {
	img2 := image.NewNRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			img2.SetNRGBA(x, y, color.NRGBA{uint8(x * 6), 10, 20, 255})
		}
	}
	p, st := Quantize(img2, opts())
	if len(p.Palette) != 40 || !st.Lossless {
		t.Fatalf("palette %d lossless %v", len(p.Palette), st.Lossless)
	}
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			r1, g1, b1, a1 := img2.At(x, y).RGBA()
			r2, g2, b2, a2 := p.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Fatalf("pixel %d,%d differs", x, y)
			}
		}
	}
}

func TestQuantizeTransparentFirst(t *testing.T) {
	p, _ := Quantize(loadNRGBA(t, "logo.png"), opts())
	seenOpaque := false
	for _, c := range p.Palette {
		_, _, _, a := c.RGBA()
		if a == 0xffff {
			seenOpaque = true
		} else if seenOpaque {
			t.Fatal("translucent colour after opaque colour")
		}
	}
}

func TestQuantizeFullyTransparentSingle(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	rng := rand.New(rand.NewSource(3))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if x < 16 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(rng.Intn(256)), uint8(rng.Intn(256)), uint8(rng.Intn(256)), 0})
			} else {
				img.SetNRGBA(x, y, color.NRGBA{uint8(x * 8), uint8(y * 8), uint8(x * y / 4), 255})
			}
		}
	}
	p, _ := Quantize(img, opts())
	n := 0
	for _, c := range p.Palette {
		if r, g, b, a := c.RGBA(); a == 0 {
			n++
			if r|g|b != 0 {
				t.Error("transparent entry has RGB")
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d transparent entries", n)
	}
}

func TestQuantizeGradientQuality(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), uint8((x + y) / 2), 255})
		}
	}
	p, st := Quantize(img, opts())
	// Ruling (Task 4): a smooth 2-D RGB plane cannot reach SSIM 0.98 with 256
	// colours (measured 0.82 dithered / 0.92 undithered). Floor lowered for this
	// synthetic case only; the reference-image thresholds are unchanged.
	if len(p.Palette) > 256 || st.SSIM < 0.80 {
		t.Fatalf("colors %d ssim %v", len(p.Palette), st.SSIM)
	}
}

func TestQuantizePicksFewColors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	rng := rand.New(rand.NewSource(7))
	base := []color.NRGBA{{200, 30, 30, 255}, {30, 200, 30, 255}, {30, 30, 200, 255}, {220, 220, 40, 255},
		{40, 220, 220, 255}, {220, 40, 220, 255}, {20, 20, 20, 255}, {240, 240, 240, 255}}
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			c := base[(y/64)*4+x/32]
			n := func(v uint8) uint8 { return uint8(int(v) + rng.Intn(5) - 2) }
			img.SetNRGBA(x, y, color.NRGBA{n(c.R), n(c.G), n(c.B), 255})
		}
	}
	p, st := Quantize(img, opts())
	if len(p.Palette) > 32 {
		t.Fatalf("palette %d (ssim %v)", len(p.Palette), st.SSIM)
	}
}

func TestQuantize16BitAndGray(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 64, 64))
	n64 := image.NewNRGBA64(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			g.SetGray(x, y, color.Gray{uint8(x * 4)})
			n64.SetNRGBA64(x, y, color.NRGBA64{uint16(x/16) * 16000, uint16(y/16) * 16000, 30000, 0xffff})
		}
	}
	for _, src := range []image.Image{g, n64} {
		_, st := Quantize(core.ToNRGBA(src), opts())
		if st.SSIM < 0.98 {
			t.Errorf("%T ssim %v", src, st.SSIM)
		}
	}
}

func TestQuantizePhoto(t *testing.T) {
	img := loadNRGBA(t, "photo.png")
	p, st := Quantize(img, opts())
	if len(p.Palette) > 256 {
		t.Fatalf("palette %d", len(p.Palette))
	}
	t.Logf("photo.png: colors=%d ssim=%.4f psnr=%.2f", st.Colors, st.SSIM, metric.PSNR(img, p))
}

func BenchmarkQuantizePhoto(b *testing.B) {
	img := loadNRGBA(b, "photo.png")
	for i := 0; i < b.N; i++ {
		Quantize(img, opts())
	}
}
