//go:build ignore

// genlogo writes the deterministic reference logo used by tests.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

func main() {
	out := flag.String("o", "logo.png", "output path")
	flag.Parse()
	const size, cx, cy, r, edge, cell = 400, 200.0, 200.0, 190.0, 5.0, 25
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	red := color.NRGBA{220, 40, 40, 255}
	blue := color.NRGBA{40, 80, 200, 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			if d >= r {
				continue
			}
			c := red
			if (x/cell+y/cell)%2 == 1 {
				c = blue
			}
			if d > r-edge {
				c.A = uint8(math.Round(255 * (r - d) / edge))
			}
			img.SetNRGBA(x, y, c)
		}
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(f, img); err != nil {
		log.Fatal(err)
	}
}
