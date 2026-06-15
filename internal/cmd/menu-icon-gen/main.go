// Rasterize Tabler outline SVGs into 36x36 monochrome PNGs for tray menu items.
// Run via:
//
//	go run ./internal/cmd/menu-icon-gen
//
// Deterministic and idempotent; safe to re-run any time.
package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

//go:embed svg/*.svg
var svgFS embed.FS

type icon struct {
	src string
	out string
}

// canvasSize is the PNG dimension (retina-crisp); glyphSize is the visible
// glyph rendered into a centered region of that canvas so the icon looks
// smaller in the menu without losing rasterization resolution.
const canvasSize = 36
const glyphSize = 32

func main() {
	icons := []icon{
		{src: "svg/app-window.svg", out: "tray/icons/menu/window.png"},
		{src: "svg/clock.svg", out: "tray/icons/menu/clock.png"},
		{src: "svg/power.svg", out: "tray/icons/menu/power.png"},
		{src: "svg/settings.svg", out: "tray/icons/menu/gear.png"},
		{src: "svg/x.svg", out: "tray/icons/menu/x.png"},
		{src: "svg/point-filled.svg", out: "tray/icons/menu/dot.png"},
		{src: "svg/cloud-download.svg", out: "tray/icons/menu/cloud-download.png"},
	}
	for _, ic := range icons {
		must(render(ic.src, ic.out))
		fmt.Println("wrote", ic.out)
	}
}

func render(srcPath, outPath string) error {
	raw, err := svgFS.ReadFile(srcPath)
	if err != nil {
		return err
	}
	raw = bytes.ReplaceAll(raw, []byte("currentColor"), []byte("#000000"))
	parsed, err := oksvg.ReadIconStream(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	offset := float64(canvasSize-glyphSize) / 2
	parsed.SetTarget(offset, offset, float64(glyphSize), float64(glyphSize))

	rgba := image.NewNRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	scanner := rasterx.NewScannerGV(canvasSize, canvasSize, rgba, rgba.Bounds())
	dasher := rasterx.NewDasher(canvasSize, canvasSize, scanner)
	parsed.Draw(dasher, 1.0)

	out := image.NewNRGBA(rgba.Bounds())
	for y := 0; y < canvasSize; y++ {
		for x := 0; x < canvasSize; x++ {
			_, _, _, a := rgba.At(x, y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: uint8(a >> 8)})
		}
	}

	dst, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	return png.Encode(dst, out)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
