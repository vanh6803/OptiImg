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
