package core

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestCompressGrayJPEG(t *testing.T) {
	g := image.NewGray(image.Rect(0, 0, 200, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 200; x++ {
			g.SetGray(x, y, color.Gray{uint8((x*7 + y*3) % 256)})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, g, &jpeg.Options{Quality: 98}); err != nil {
		t.Fatal(err)
	}
	out, r, err := Compress(buf.Bytes(), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("output not decodable: %v (skipped=%q)", err, r.Skipped)
	}
}

func TestCompressRotatedJPEG(t *testing.T) {
	b := jpegWithOrientation(t, 160, 80, 6)
	out, r, err := Compress(b, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped == "" {
		cfg, _ := jpeg.DecodeConfig(bytes.NewReader(out))
		if cfg.Width != 80 || cfg.Height != 160 {
			t.Errorf("output %dx%d, want 80x160", cfg.Width, cfg.Height)
		}
	}
}

func TestCompress16BitPNG(t *testing.T) {
	img := image.NewNRGBA64(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA64(x, y, color.NRGBA64{uint16(x * 1000), uint16(y * 1000), uint16(x*y) * 8, 0xffff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, _, err := Compress(buf.Bytes(), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatal(err)
	}
}
