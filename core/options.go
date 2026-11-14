// Package core is the pure compression library: bytes in, bytes out.
// It never touches the filesystem.
package core

// Format is a supported image container.
type Format string

const (
	FormatPNG  Format = "png"
	FormatJPEG Format = "jpeg"
)

// Limits bounds the input accepted by Compress.
type Limits struct {
	MaxBytes  int64
	MaxPixels int64
}

// Options control Compress.
type Options struct {
	Lossless bool    // PNG: no colour reduction
	MinSSIM  float64 // quality floor driving palette size / JPEG quality
	Dither   float64 // Floyd–Steinberg strength for PNG
	Quality  int     // JPEG quality, 0 = auto search
	WhiteBG  bool    // flatten transparency on white
	Limits   Limits
}

// DefaultOptions returns the documented defaults.
func DefaultOptions() Options {
	return Options{MinSSIM: 0.98, Dither: 1.0, Limits: Limits{MaxBytes: 50 << 20, MaxPixels: 100_000_000}}
}
