// Package pngenc writes PNG files with the standard encoder at best
// compression. The standard encoder already picks the palette bit depth,
// writes tRNS only when needed, and emits no ancillary chunks.
package pngenc

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// Encode writes img as PNG.
func Encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Lossless returns an *image.Gray when every pixel is opaque grey; otherwise
// it returns img unchanged.
func Lossless(img *image.NRGBA) image.Image {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i+3] != 255 || img.Pix[i] != img.Pix[i+1] || img.Pix[i] != img.Pix[i+2] {
			return img
		}
	}
	b := img.Bounds()
	g := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g.SetGray(x, y, color.Gray{img.NRGBAAt(x, y).R})
		}
	}
	return g
}
