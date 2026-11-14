package core

import (
	"bytes"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHeavyDir compresses every PNG/JPEG in $OPTIIMG_HEAVY_DIR (4K/8K photos,
// log-profile frames, 16-bit PNGs, ...). Skipped when the variable is unset:
// the images are too large to commit.
func TestHeavyDir(t *testing.T) {
	dir := os.Getenv("OPTIIMG_HEAVY_DIR")
	if dir == "" {
		t.Skip("set OPTIIMG_HEAVY_DIR to run")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*"))
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			continue
		}
		in, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		out, r, err := Compress(in, DefaultOptions())
		if err != nil {
			t.Logf("%-22s error: %v", filepath.Base(f), err)
			continue
		}
		if ext == ".png" {
			_, err = png.Decode(bytes.NewReader(out))
		} else {
			_, err = jpeg.Decode(bytes.NewReader(out))
		}
		if err != nil {
			t.Errorf("%s: output does not decode: %v", f, err)
		}
		t.Logf("%-22s %dx%d %8d -> %8d (-%.1f%%) colors=%d q=%d ssim=%.4f psnr=%.1f %v",
			filepath.Base(f), r.Width, r.Height, len(in), len(out), r.SavedPct, r.Colors, r.Quality, r.SSIM, r.PSNR, time.Since(start).Round(time.Millisecond))
	}
}
