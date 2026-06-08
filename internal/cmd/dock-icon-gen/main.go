// One-shot helper: composite tray/icons/icon-source.png (the BL logo on a
// transparent background, preserved as the immutable source of truth for all
// derived icons) over a white macOS-style squircle and overwrite
// build/appicon.png. Wails's packager regenerates Contents/Resources/iconfile.icns
// from build/appicon.png at every build via jackmordaunt/icns; the icns goes
// muddy when the source PNG has transparent pixels because the Dock renders
// its translucent background blur through the transparent regions.
// A fully opaque squircle source eliminates that.
//
// Run via:
//
//	go run ./internal/cmd/dock-icon-gen
//
// Re-run any time tray/icons/icon-source.png changes.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	xdraw "golang.org/x/image/draw"
)

const (
	canvas       = 1024
	cornerRadius = 188 // ~18.5%, matches macOS Big Sur+ icon corner radius.
	logoInset    = 128 // padding from squircle edges to give the logo breathing room.
)

func main() {
	in, err := os.Open("tray/icons/icon-source.png")
	must(err)
	defer in.Close()

	logo, err := png.Decode(in)
	must(err)

	bg := image.NewRGBA(image.Rect(0, 0, canvas, canvas))
	fillSquircle(bg, color.RGBA{255, 255, 255, 255})

	// Scale the logo down to leave logoInset px of padding on every side.
	innerSize := canvas - 2*logoInset
	inner := image.NewRGBA(image.Rect(0, 0, innerSize, innerSize))
	xdraw.CatmullRom.Scale(inner, inner.Bounds(), logo, logo.Bounds(), xdraw.Over, nil)
	target := image.Rect(logoInset, logoInset, logoInset+innerSize, logoInset+innerSize)
	draw.Draw(bg, target, inner, image.Point{}, draw.Over)

	dst, err := os.Create("build/appicon.png")
	must(err)
	defer dst.Close()
	must(png.Encode(dst, bg))
	fmt.Println("wrote build/appicon.png (white squircle composite for Wails to ingest)")
}

func fillSquircle(img *image.RGBA, fill color.RGBA) {
	r := float64(cornerRadius)
	w, h := canvas, canvas
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if insideRoundedRect(float64(x), float64(y), float64(w), float64(h), r) {
				img.SetRGBA(x, y, fill)
			}
		}
	}
}

func insideRoundedRect(x, y, w, h, r float64) bool {
	dx := math.Max(math.Max(r-x, 0), math.Max(x-(w-r), 0))
	dy := math.Max(math.Max(r-y, 0), math.Max(y-(h-r), 0))
	return dx*dx+dy*dy <= r*r
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
