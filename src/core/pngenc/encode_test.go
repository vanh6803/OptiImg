package pngenc

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func chunks(t *testing.T, b []byte) (types []string, ihdr []byte) {
	t.Helper()
	for p := 8; p+8 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[p:]))
		typ := string(b[p+4 : p+8])
		if typ == "IHDR" {
			ihdr = b[p+8 : p+8+n]
		}
		if len(types) == 0 || types[len(types)-1] != typ {
			types = append(types, typ)
		}
		p += 12 + n
	}
	return
}

func palImage(n int, alpha bool) *image.Paletted {
	pal := make(color.Palette, n)
	for i := range pal {
		a := uint8(255)
		if alpha && i < 3 {
			a = uint8(60 * i)
		}
		pal[i] = color.NRGBA{uint8(i * 5), uint8(255 - i*5), 40, a}
	}
	img := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	for i := range img.Pix {
		img.Pix[i] = uint8(i % n)
	}
	return img
}

func TestEncodePalettedChunks(t *testing.T) {
	b, err := Encode(palImage(40, true))
	if err != nil {
		t.Fatal(err)
	}
	types, ihdr := chunks(t, b)
	want := []string{"IHDR", "PLTE", "tRNS", "IDAT", "IEND"}
	if len(types) != len(want) {
		t.Fatalf("chunks %v", types)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("chunks %v", types)
		}
	}
	if ihdr[9] != 3 {
		t.Errorf("color type %d", ihdr[9])
	}
}

func TestEncodeBitDepth(t *testing.T) {
	b, _ := Encode(palImage(4, false))
	_, ihdr := chunks(t, b)
	if ihdr[8] != 2 {
		t.Errorf("bit depth %d", ihdr[8])
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	src := palImage(40, true)
	b, _ := Encode(src)
	dec, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			r1, g1, b1, a1 := src.At(x, y).RGBA()
			r2, g2, b2, a2 := dec.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				t.Fatalf("pixel %d,%d", x, y)
			}
		}
	}
}

func TestLosslessGray(t *testing.T) {
	g := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := 0; i < len(g.Pix); i += 4 {
		v := uint8(i * 3)
		g.Pix[i], g.Pix[i+1], g.Pix[i+2], g.Pix[i+3] = v, v, v, 255
	}
	if _, ok := Lossless(g).(*image.Gray); !ok {
		t.Error("grey not detected")
	}
	g.SetNRGBA(0, 0, color.NRGBA{1, 2, 3, 255})
	if _, ok := Lossless(g).(*image.NRGBA); !ok {
		t.Error("colour image changed type")
	}
}
