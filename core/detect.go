package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/jpeg" // register decoders for DecodeConfig
	_ "image/png"
)

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// Detect identifies the format from magic bytes, never from a file name.
func Detect(b []byte) (Format, error) {
	switch {
	case bytes.HasPrefix(b, pngMagic):
		return FormatPNG, nil
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return FormatJPEG, nil
	}
	return "", ErrUnsupportedFormat
}

// CheckLimits validates size limits from the header, before full decoding.
func CheckLimits(b []byte, f Format, l Limits) (w, h int, err error) {
	if l.MaxBytes > 0 && int64(len(b)) > l.MaxBytes {
		return 0, 0, fmt.Errorf("%w: %d bytes > %d", ErrTooLarge, len(b), l.MaxBytes)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if l.MaxPixels > 0 && int64(cfg.Width)*int64(cfg.Height) > l.MaxPixels {
		return 0, 0, fmt.Errorf("%w: %dx%d pixels", ErrTooLarge, cfg.Width, cfg.Height)
	}
	return cfg.Width, cfg.Height, nil
}

// IsAnimatedPNG reports whether an acTL chunk precedes the first IDAT.
func IsAnimatedPNG(b []byte) bool {
	if !bytes.HasPrefix(b, pngMagic) {
		return false
	}
	for p := len(pngMagic); p+8 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[p:]))
		typ := string(b[p+4 : p+8])
		switch typ {
		case "acTL":
			return true
		case "IDAT", "IEND":
			return false
		}
		p += 12 + n
	}
	return false
}

// hasChunk reports whether a PNG has a chunk of the given type before IDAT.
func hasPNGChunk(b []byte, want string) bool {
	if !bytes.HasPrefix(b, pngMagic) {
		return false
	}
	for p := len(pngMagic); p+8 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[p:]))
		typ := string(b[p+4 : p+8])
		if typ == want {
			return true
		}
		if typ == "IDAT" || typ == "IEND" {
			return false
		}
		p += 12 + n
	}
	return false
}
