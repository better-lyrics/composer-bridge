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

const targetSize = 36

func main() {
	icons := []icon{
		{src: "svg/app-window.svg", out: "tray/icons/menu/window.png"},
		{src: "svg/clock.svg", out: "tray/icons/menu/clock.png"},
		{src: "svg/power.svg", out: "tray/icons/menu/power.png"},
		{src: "svg/settings.svg", out: "tray/icons/menu/gear.png"},
		{src: "svg/x.svg", out: "tray/icons/menu/x.png"},
		{src: "svg/point-filled.svg", out: "tray/icons/menu/dot.png"},
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
	parsed.SetTarget(0, 0, float64(targetSize), float64(targetSize))

	rgba := image.NewNRGBA(image.Rect(0, 0, targetSize, targetSize))
	scanner := rasterx.NewScannerGV(targetSize, targetSize, rgba, rgba.Bounds())
	dasher := rasterx.NewDasher(targetSize, targetSize, scanner)
	parsed.Draw(dasher, 1.0)

	out := image.NewNRGBA(rgba.Bounds())
	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
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
