package core

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func fakeProfile(tag string) []byte {
	p := make([]byte, 600)
	copy(p[36:], "acsp")
	copy(p[128:], tag)
	for i := 200; i < len(p); i++ {
		p[i] = byte(i * 7)
	}
	return p
}

func gradientRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 3), uint8(y * 5), uint8(x ^ y), 255})
		}
	}
	return img
}

func jpegWithICC(t *testing.T, prof []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gradientRGBA(256, 192), &jpeg.Options{Quality: 99}); err != nil {
		t.Fatal(err)
	}
	return embedICC(buf.Bytes(), FormatJPEG, prof)
}

func pngWithICC(t *testing.T, prof []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, gradientRGBA(200, 200)); err != nil {
		t.Fatal(err)
	}
	return embedICC(buf.Bytes(), FormatPNG, prof)
}

func TestICCRoundTripHelpers(t *testing.T) {
	prof := fakeProfile("Display P3 test")
	if got := extractICC(jpegWithICC(t, prof), FormatJPEG); !bytes.Equal(got, prof) {
		t.Error("JPEG round trip failed")
	}
	if got := extractICC(pngWithICC(t, prof), FormatPNG); !bytes.Equal(got, prof) {
		t.Error("PNG round trip failed")
	}
	big := bytes.Repeat([]byte{7}, 70000) // forces two APP2 chunks
	copy(big[36:], "acsp")
	if got := extractICC(jpegWithICC(t, big), FormatJPEG); !bytes.Equal(got, big) {
		t.Error("multi-chunk JPEG round trip failed")
	}
}

func TestCompressKeepsNonSRGBProfile(t *testing.T) {
	prof := fakeProfile("Display P3 test")
	for _, c := range []struct {
		name string
		in   []byte
		f    Format
	}{{"jpeg", jpegWithICC(t, prof), FormatJPEG}, {"png", pngWithICC(t, prof), FormatPNG}} {
		out, r, err := Compress(c.in, DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if r.Skipped != "" {
			continue
		}
		if got := extractICC(out, c.f); !bytes.Equal(got, prof) {
			t.Errorf("%s: profile not preserved", c.name)
		}
		if !contains(r.Warnings, "icc_profile_kept") {
			t.Errorf("%s: warnings %v", c.name, r.Warnings)
		}
		if _, err := Decode(out, c.f); err != nil {
			t.Errorf("%s: output does not decode: %v", c.name, err)
		}
	}
}

func TestCompressStripICCAndSRGB(t *testing.T) {
	p3 := fakeProfile("Display P3 test")
	o := DefaultOptions()
	o.StripICC = true
	out, r, err := Compress(jpegWithICC(t, p3), o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped == "" && extractICC(out, FormatJPEG) != nil {
		t.Error("StripICC kept the profile")
	}
	if !contains(r.Warnings, "icc_profile_dropped") {
		t.Errorf("warnings %v", r.Warnings)
	}
	srgb := fakeProfile("sRGB IEC61966-2.1")
	out, r, err = Compress(pngWithICC(t, srgb), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped == "" && extractICC(out, FormatPNG) != nil {
		t.Error("sRGB profile should be dropped")
	}
	if contains(r.Warnings, "icc_profile_kept") {
		t.Error("sRGB profile reported as kept")
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
