package jpegenc

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"testing"

	"optiimg/core/metric"
)

func loadPhoto(t testing.TB) image.Image {
	t.Helper()
	b, err := os.ReadFile("../../testdata/photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// markers lists marker bytes before SOS.
func markers(b []byte) []byte {
	var m []byte
	for p := 2; p+4 <= len(b); {
		if b[p] != 0xFF {
			break
		}
		m = append(m, b[p+1])
		if b[p+1] == 0xDA {
			break
		}
		p += 2 + int(b[p+2])<<8 + int(b[p+3])
	}
	return m
}

func smallPhoto(t testing.TB) image.Image {
	t.Helper()
	img := loadPhoto(t)
	return img.(*image.YCbCr).SubImage(image.Rect(0, 0, 256, 256))
}

func TestEncodeProgressive(t *testing.T) {
	b, err := Encode(smallPhoto(t), 75)
	if err != nil {
		t.Fatal(err)
	}
	var sof2 bool
	for _, m := range markers(b) {
		if m == 0xC2 {
			sof2 = true
		}
		if m >= 0xE1 && m <= 0xEF {
			t.Errorf("unexpected APP marker %X", m)
		}
	}
	if !sof2 {
		t.Error("no SOF2 (not progressive)")
	}
}

func TestEncodeSubsampling420(t *testing.T) {
	b, _ := Encode(smallPhoto(t), 75)
	for p := 2; p+4 <= len(b); {
		m := b[p+1]
		if m == 0xC2 || m == 0xC0 {
			// SOF: len(2) P(1) Y(2) X(2) Nf(1) then 3 bytes per component
			c := b[p+10:]
			if c[1] != 0x22 || c[4] != 0x11 || c[7] != 0x11 {
				t.Fatalf("sampling Y=%x Cb=%x Cr=%x", c[1], c[4], c[7])
			}
			return
		}
		p += 2 + int(b[p+2])<<8 + int(b[p+3])
	}
	t.Fatal("no SOF")
}

func TestEstimateQuality(t *testing.T) {
	img := smallPhoto(t)
	for _, c := range []struct{ q, lo, hi int }{{95, 93, 97}, {70, 67, 73}} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: c.q}); err != nil {
			t.Fatal(err)
		}
		q, ok := EstimateQuality(buf.Bytes())
		if !ok || q < c.lo || q > c.hi {
			t.Errorf("q=%d estimated %d ok=%v", c.q, q, ok)
		}
	}
	if _, ok := EstimateQuality([]byte("garbage data here")); ok {
		t.Error("garbage accepted")
	}
}

func TestEncodeAutoMeetsSSIM(t *testing.T) {
	img := loadPhoto(t)
	out, q, ssim, err := EncodeAuto(img, 55, 85, 0.98)
	if err != nil {
		t.Fatal(err)
	}
	if q < 55 || q > 85 {
		t.Errorf("q = %d", q)
	}
	dec, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	s := metric.SSIM(img, dec)
	if s-ssim > 1e-6 || ssim-s > 1e-6 {
		t.Errorf("reported %v recomputed %v", ssim, s)
	}
	// Ruling (Task 6): the floor 0.98 is unreachable for this photo even at q=85
	// (measured 0.979), in which case the documented behaviour is to return hi.
	if s < 0.98 && q != 85 {
		t.Errorf("ssim %v below floor but q=%d is not the upper bound", s, q)
	}
	t.Logf("q=%d ssim=%.4f size=%d", q, ssim, len(out))
}

func TestSampleMosaic(t *testing.T) {
	img := loadPhoto(t) // 1200×652
	s := Sample(img, 256)
	if s.Bounds().Dx() != 512 || s.Bounds().Dy() != 512 {
		t.Fatalf("size %v", s.Bounds())
	}
	// top-left tile must equal the source block at the first quarter point
	x0, y0 := (1200/4-128)&^15, (652/4-128)&^15
	r1, g1, b1, _ := img.At(x0+5, y0+7).RGBA()
	r2, g2, b2, _ := s.At(5, 7).RGBA()
	if absDiff(r1, r2) > 400 || absDiff(g1, g2) > 400 || absDiff(b1, b2) > 400 {
		t.Error("tile content mismatch")
	}
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
