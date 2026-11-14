// Package jpegenc encodes progressive 4:2:0 JPEGs with jpegli (WebAssembly,
// no cgo) and searches the lowest quality that still meets an SSIM floor.
package jpegenc

import (
	"bytes"
	"image"

	"github.com/gen2brain/jpegli"
)

// Encode writes img as progressive 4:2:0 JPEG with optimised Huffman tables.
func Encode(img image.Image, q int) ([]byte, error) {
	var buf bytes.Buffer
	err := jpegli.Encode(&buf, img, &jpegli.EncodingOptions{
		Quality:              q,
		ChromaSubsampling:    image.YCbCrSubsampleRatio420,
		ProgressiveLevel:     2,
		OptimizeCoding:       true,
		AdaptiveQuantization: true,
		FancyDownsampling:    true,
		DCTMethod:            jpegli.DefaultDCTMethod,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LowMemPixels is the size above which Compress uses EncodeStreaming.
const LowMemPixels = 16_000_000

// EncodeStreaming writes a sequential JPEG with standard Huffman tables.
// Progressive scans and optimised Huffman tables force jpegli to buffer the
// whole image (≈ 40 B/pixel); this mode needs ≈ 10 B/pixel (50 MP: 2.1 GB →
// 0.5 GB measured) at the cost of ≈ 14% larger files.
func EncodeStreaming(img image.Image, q int) ([]byte, error) {
	var buf bytes.Buffer
	err := jpegli.Encode(&buf, img, &jpegli.EncodingOptions{
		Quality:              q,
		ChromaSubsampling:    image.YCbCrSubsampleRatio420,
		ProgressiveLevel:     0,
		OptimizeCoding:       false,
		AdaptiveQuantization: true,
		FancyDownsampling:    true,
		DCTMethod:            jpegli.DefaultDCTMethod,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeLowMem is the memory-frugal path for very large images: a streaming
// sequential encode followed by a lossless Huffman-table optimisation pass,
// which recovers most of the size lost by skipping jpegli's own optimiser.
func EncodeLowMem(img image.Image, q int) ([]byte, error) {
	b, err := EncodeStreaming(img, q)
	if err != nil {
		return nil, err
	}
	if opt, err := OptimizeHuffman(b); err == nil {
		return opt, nil
	}
	return b, nil
}
