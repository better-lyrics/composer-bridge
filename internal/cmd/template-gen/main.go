// One-shot helper: read tray/icons/icon-source.png (the immutable BL-logo
// transparent source), output a 44x44 monochrome template PNG suitable for
// macOS menubar SetTemplateIcon. Alpha channel becomes the mask; every
// opaque pixel is rendered as solid black. Run via:
//
//	go run ./internal/cmd/template-gen
//
// The committed output lives at tray/icons/icon-mac.png. Re-run any time
// tray/icons/icon-source.png changes; it's deterministic and idempotent.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

func main() {
	in, err := os.Open("tray/icons/icon-source.png")
	must(err)
	defer in.Close()

	src, err := png.Decode(in)
	must(err)

	const dim = 44
	scaled := image.NewNRGBA(image.Rect(0, 0, dim, dim))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Over, nil)

	out := image.NewNRGBA(scaled.Bounds())
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			_, _, _, a := scaled.At(x, y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{0, 0, 0, uint8(a >> 8)})
		}
	}

	dst, err := os.Create("tray/icons/icon-mac.png")
	must(err)
	defer dst.Close()
	must(png.Encode(dst, out))
	fmt.Println("wrote tray/icons/icon-mac.png 44x44 template")
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
