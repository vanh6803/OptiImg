package core

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

// Decode decodes b. JPEG output is already rotated per EXIF Orientation.
func Decode(b []byte, f Format) (image.Image, error) {
	var img image.Image
	var err error
	switch f {
	case FormatPNG:
		img, err = png.Decode(bytes.NewReader(b))
	case FormatJPEG:
		img, err = jpeg.Decode(bytes.NewReader(b))
		if err == nil {
			if o := jpegOrientation(b); o != 1 {
				img = orient(img, o)
			}
		}
	default:
		return nil, ErrUnsupportedFormat
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return img, nil
}

// orient applies EXIF orientation o (2..8) and returns an NRGBA image.
func orient(src image.Image, o int) *image.NRGBA {
	s := ToNRGBA(src)
	w, h := s.Bounds().Dx(), s.Bounds().Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			si := s.PixOffset(x, y)
			di := dst.PixOffset(dx, dy)
			copy(dst.Pix[di:di+4], s.Pix[si:si+4])
		}
	}
	return dst
}

// ToNRGBA converts any image to a zero-origin 8-bit non-premultiplied NRGBA.
func ToNRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	if n, ok := img.(*image.NRGBA); ok && b.Min == (image.Point{}) {
		return n
	}
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			out.SetNRGBA(x, y, color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA))
		}
	}
	return out
}

// HasAlpha reports whether any pixel is not fully opaque.
func HasAlpha(img image.Image) bool {
	switch m := img.(type) {
	case *image.NRGBA:
		for i := 3; i < len(m.Pix); i += 4 {
			if m.Pix[i] != 255 {
				return true
			}
		}
		return false
	case *image.Gray, *image.Gray16, *image.YCbCr, *image.CMYK:
		return false
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return true
			}
		}
	}
	return false
}

// FlattenWhite composites img over white; every output pixel has A=255.
func FlattenWhite(img image.Image) *image.NRGBA {
	src := ToNRGBA(img)
	out := image.NewNRGBA(src.Bounds())
	for i := 0; i < len(src.Pix); i += 4 {
		a := int(src.Pix[i+3])
		for c := 0; c < 3; c++ {
			out.Pix[i+c] = uint8((int(src.Pix[i+c])*a + 255*(255-a) + 127) / 255)
		}
		out.Pix[i+3] = 255
	}
	return out
}
