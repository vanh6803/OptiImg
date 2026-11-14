package core

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// jpegWithOrientation encodes a w×h JPEG and, if o>0, inserts an Exif APP1
// segment carrying Orientation=o.
func jpegWithOrientation(t *testing.T, w, h, o int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if o == 0 {
		return b
	}
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, byte(o), 0, 0, 0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	seg = append(seg, payload...)
	return append(append(append([]byte{}, b[:2]...), seg...), b[2:]...)
}

func TestDecodeEXIFOrientation(t *testing.T) {
	img, err := Decode(jpegWithOrientation(t, 20, 10, 6), FormatJPEG)
	if err != nil {
		t.Fatal(err)
	}
	if s := img.Bounds().Size(); s != (image.Point{10, 20}) {
		t.Errorf("orientation 6: size %v", s)
	}
	img, err = Decode(jpegWithOrientation(t, 20, 10, 1), FormatJPEG)
	if err != nil {
		t.Fatal(err)
	}
	if s := img.Bounds().Size(); s != (image.Point{20, 10}) {
		t.Errorf("orientation 1: size %v", s)
	}
	img, _ = Decode(jpegWithOrientation(t, 20, 10, 0), FormatJPEG)
	if s := img.Bounds().Size(); s != (image.Point{20, 10}) {
		t.Errorf("no exif: size %v", s)
	}
}

func TestFlattenWhite(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{0, 0, 0, 0})
	src.SetNRGBA(1, 0, color.NRGBA{255, 0, 0, 128})
	out := FlattenWhite(src)
	if c := out.NRGBAAt(0, 0); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("transparent -> %v", c)
	}
	c := out.NRGBAAt(1, 0)
	if c.R != 255 || c.A != 255 || abs(int(c.G)-127) > 1 || abs(int(c.B)-127) > 1 {
		t.Errorf("half red -> %v", c)
	}
	if HasAlpha(out) || !HasAlpha(src) {
		t.Error("HasAlpha wrong")
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestDecodeCorrupt(t *testing.T) {
	b := pngBytes(t, 40, 40)
	_, err := Decode(b[:len(b)/2], FormatPNG)
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v", err)
	}
}
