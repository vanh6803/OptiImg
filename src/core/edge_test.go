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

func TestCompressLargeJPEGUsesSample(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2600, 1600)) // 4.16 MP > sample threshold
	for y := 0; y < 1600; y++ {
		for x := 0; x < 2600; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x/11 + y/9), uint8((x*y)>>9) ^ uint8(x), uint8(y / 7), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 97}); err != nil {
		t.Fatal(err)
	}
	out, r, err := Compress(buf.Bytes(), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped == "" {
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
		if err != nil || cfg.Width != 2600 || cfg.Height != 1600 {
			t.Fatalf("output %v %v", cfg, err)
		}
		if r.Quality < 55 || r.Quality > 80 || r.SSIM <= 0 {
			t.Errorf("report %+v", r)
		}
	}
}

func TestDefaultLimitsAllowLargeFiles(t *testing.T) {
	if DefaultOptions().Limits.MaxBytes != 0 {
		t.Error("default MaxBytes must be unlimited")
	}
}

func TestCompressHugeJPEGUsesLowMemoryMode(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates a 17 MP image")
	}
	img := image.NewRGBA(image.Rect(0, 0, 4200, 4000)) // 16.8 MP > LowMemPixels
	for y := 0; y < 4000; y++ {
		for x := 0; x < 4200; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x/17 + y/13), uint8(x ^ y), uint8(y / 11), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 97}); err != nil {
		t.Fatal(err)
	}
	out, r, err := Compress(buf.Bytes(), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped != "" {
		t.Skip("input already optimal")
	}
	found := false
	for _, w := range r.Warnings {
		found = found || w == "low_memory_mode"
	}
	if !found {
		t.Errorf("warnings %v lack low_memory_mode", r.Warnings)
	}
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(out)); err != nil || cfg.Width != 4200 || cfg.Height != 4000 {
		t.Fatalf("output %v %v", cfg, err)
	}
}
