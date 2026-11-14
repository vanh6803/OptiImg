package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want Format
		err  error
	}{
		{"png", pngBytes(t, 4, 4), FormatPNG, nil},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}, FormatJPEG, nil},
		{"gif", []byte("GIF89a......"), "", ErrUnsupportedFormat},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "", ErrUnsupportedFormat},
		{"empty", nil, "", ErrUnsupportedFormat},
	}
	for _, c := range cases {
		got, err := Detect(c.in)
		if got != c.want || !errors.Is(err, c.err) {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", c.name, got, err, c.want, c.err)
		}
	}
}

func TestCheckLimits(t *testing.T) {
	b := pngBytes(t, 20, 20)
	if _, _, err := CheckLimits(b, FormatPNG, Limits{MaxBytes: 1 << 20, MaxPixels: 100}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("pixels: %v", err)
	}
	if _, _, err := CheckLimits(b, FormatPNG, Limits{MaxBytes: 10, MaxPixels: 1 << 20}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("bytes: %v", err)
	}
	w, h, err := CheckLimits(b, FormatPNG, DefaultOptions().Limits)
	if w != 20 || h != 20 || err != nil {
		t.Errorf("default: %d %d %v", w, h, err)
	}
}

func TestIsAnimatedPNG(t *testing.T) {
	b := pngBytes(t, 4, 4)
	// insert acTL (num_frames=2, num_plays=0) right after IHDR (8 sig + 25 IHDR)
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data, 2)
	chunk := make([]byte, 0, 20)
	chunk = binary.BigEndian.AppendUint32(chunk, 8)
	chunk = append(chunk, "acTL"...)
	chunk = append(chunk, data...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(append([]byte("acTL"), data...)))
	anim := append(append(append([]byte{}, b[:33]...), chunk...), b[33:]...)
	if !IsAnimatedPNG(anim) {
		t.Error("APNG not detected")
	}
	logo, err := os.ReadFile("../testdata/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if IsAnimatedPNG(logo) {
		t.Error("logo.png flagged animated")
	}
}
