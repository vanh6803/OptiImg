package core

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCompressReference(t *testing.T) {
	cases := []struct {
		file    string
		maxPct  int
		maxAbs  int
		minPSNR float64
	}{
		{"logo.png", 0, 5000, 45},
		{"photo.png", 34, 0, 30},
		{"photo.jpg", 36, 0, 33},
	}
	for _, c := range cases {
		in := readTestdata(t, c.file)
		out, r, err := Compress(in, DefaultOptions())
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if c.maxAbs > 0 && len(out) > c.maxAbs {
			t.Errorf("%s: size %d > %d", c.file, len(out), c.maxAbs)
		}
		if c.maxPct > 0 && len(out)*100 > len(in)*c.maxPct {
			t.Errorf("%s: size %d > %d%% of %d", c.file, len(out), c.maxPct, len(in))
		}
		if r.PSNR < c.minPSNR {
			t.Errorf("%s: PSNR %.2f < %.0f", c.file, r.PSNR, c.minPSNR)
		}
		if r.Skipped != "" {
			t.Errorf("%s: skipped %q", c.file, r.Skipped)
		}
		t.Logf("%s: %d -> %d (%.1f%%) colors=%d q=%d ssim=%.4f psnr=%.2f", c.file, len(in), len(out), r.SavedPct, r.Colors, r.Quality, r.SSIM, r.PSNR)
		if c.file == "logo.png" {
			if r.Colors > 64 {
				t.Errorf("logo colors %d", r.Colors)
			}
			if !hasPNGChunkAnywhere(out, "tRNS") {
				t.Error("logo output lacks tRNS")
			}
		}
	}
}

func hasPNGChunkAnywhere(b []byte, name string) bool { return bytes.Contains(b, []byte(name)) }

func TestCompressIdempotent(t *testing.T) {
	out, _, err := Compress(readTestdata(t, "photo.png"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out2, r, err := Compress(out, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped != SkippedAlreadyOptimal || !bytes.Equal(out, out2) {
		t.Errorf("second pass: skipped=%q equal=%v", r.Skipped, bytes.Equal(out, out2))
	}
}

func TestCompressAnimatedPNG(t *testing.T) {
	_, _, err := Compress(addACTL(pngBytes(t, 8, 8)), DefaultOptions())
	if !errors.Is(err, ErrAnimated) {
		t.Errorf("err = %v", err)
	}
}

func TestCompressWrongExtension(t *testing.T) {
	_, r, err := Compress(readTestdata(t, "photo.jpg"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.FormatIn != "jpeg" {
		t.Errorf("FormatIn = %q", r.FormatIn)
	}
}

func TestCompressWhiteBG(t *testing.T) {
	o := DefaultOptions()
	o.WhiteBG = true
	out, _, err := Compress(readTestdata(t, "logo.png"), o)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if HasAlpha(img) {
		t.Error("output still has transparency")
	}
}

func TestCompressPalettedInput(t *testing.T) {
	pal := make(color.Palette, 16)
	for i := range pal {
		pal[i] = color.RGBA{uint8(i * 16), uint8(255 - i*16), 90, 255}
	}
	src := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
	for i := range src.Pix {
		src.Pix[i] = uint8((i / 7) % 16)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	out, r, err := Compress(buf.Bytes(), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > buf.Len() || r.SizeOut > r.SizeIn {
		t.Errorf("grew: %d -> %d", buf.Len(), len(out))
	}
}

func TestCompressUnsupported(t *testing.T) {
	_, _, err := Compress([]byte("GIF89a\x01\x00\x01\x00"), DefaultOptions())
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("err = %v", err)
	}
}
