// One-shot helper: emit 8 PNGs for the system tray icon (4 mac template +
// 4 default colored), one per bridgestate.State variant (idle, downloading,
// error, stopped). Mac variants are 44x44 monochrome templates derived from
// tray/icons/icon-source.png; the alpha channel becomes the mask, every
// opaque pixel renders solid black so macOS can tint per appearance. Default
// variants are 44x44 colored renders derived from tray/icons/icon.png.
//
// Downloading + error variants overlay a small filled circle in the
// lower-right corner (radius 6 centered at (35, 35)). For mac the circle is
// opaque black baked into the alpha mask. For default the circle uses the
// BL accent red (#FF0040). Stopped multiplies alpha by 0.4 so the glyph
// reads as dimmed.
//
// Run via:
//
//	go run ./internal/cmd/tray-icon-gen
//
// The committed outputs live under tray/icons/tray-*.png. The implementation
// is deterministic and idempotent: re-running produces byte-identical PNGs
// assuming the sources have not changed.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"golang.org/x/image/draw"
)

const (
	dim         = 44
	badgeRadius = 6.0
	badgeCX     = 35.0
	badgeCY     = 35.0
	dimAlpha    = 0.4
)

var badgeColorDefault = color.NRGBA{0xFF, 0x00, 0x40, 0xFF}

func main() {
	macSrc := loadAndScale("tray/icons/icon-source.png")
	macIdle := toTemplate(macSrc)
	macDownloading := overlayBadge(cloneNRGBA(macIdle), color.NRGBA{0, 0, 0, 0xFF})
	macError := overlayBadge(cloneNRGBA(macIdle), color.NRGBA{0, 0, 0, 0xFF})
	macStopped := dim40(cloneNRGBA(macIdle))

	writePNG("tray/icons/tray-mac-idle.png", macIdle)
	writePNG("tray/icons/tray-mac-downloading.png", macDownloading)
	writePNG("tray/icons/tray-mac-error.png", macError)
	writePNG("tray/icons/tray-mac-stopped.png", macStopped)

	defSrc := loadAndScale("tray/icons/icon.png")
	defIdle := defSrc
	defDownloading := overlayBadge(cloneNRGBA(defIdle), badgeColorDefault)
	defError := overlayBadge(cloneNRGBA(defIdle), badgeColorDefault)
	defStopped := dim40(cloneNRGBA(defIdle))

	writePNG("tray/icons/tray-default-idle.png", defIdle)
	writePNG("tray/icons/tray-default-downloading.png", defDownloading)
	writePNG("tray/icons/tray-default-error.png", defError)
	writePNG("tray/icons/tray-default-stopped.png", defStopped)
}

func loadAndScale(path string) *image.NRGBA {
	in, err := os.Open(path)
	must(err)
	defer in.Close()
	src, err := png.Decode(in)
	must(err)
	dst := image.NewNRGBA(image.Rect(0, 0, dim, dim))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

func toTemplate(src *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(src.Bounds())
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{0, 0, 0, uint8(a >> 8)})
		}
	}
	return out
}

func cloneNRGBA(src *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	return out
}

func overlayBadge(dst *image.NRGBA, c color.NRGBA) *image.NRGBA {
	minX := int(math.Floor(badgeCX - badgeRadius - 1))
	maxX := int(math.Ceil(badgeCX + badgeRadius + 1))
	minY := int(math.Floor(badgeCY - badgeRadius - 1))
	maxY := int(math.Ceil(badgeCY + badgeRadius + 1))
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > dim {
		maxX = dim
	}
	if maxY > dim {
		maxY = dim
	}
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			dx := float64(x) + 0.5 - badgeCX
			dy := float64(y) + 0.5 - badgeCY
			d := math.Sqrt(dx*dx + dy*dy)
			cover := badgeRadius + 0.5 - d
			if cover <= 0 {
				continue
			}
			if cover > 1 {
				cover = 1
			}
			srcA := float64(c.A) / 255.0 * cover
			i := dst.PixOffset(x, y)
			dr := float64(dst.Pix[i+0]) / 255.0
			dg := float64(dst.Pix[i+1]) / 255.0
			db := float64(dst.Pix[i+2]) / 255.0
			da := float64(dst.Pix[i+3]) / 255.0
			sr := float64(c.R) / 255.0
			sg := float64(c.G) / 255.0
			sb := float64(c.B) / 255.0
			outA := srcA + da*(1-srcA)
			var outR, outG, outB float64
			if outA > 0 {
				outR = (sr*srcA + dr*da*(1-srcA)) / outA
				outG = (sg*srcA + dg*da*(1-srcA)) / outA
				outB = (sb*srcA + db*da*(1-srcA)) / outA
			}
			dst.Pix[i+0] = uint8(math.Round(outR * 255))
			dst.Pix[i+1] = uint8(math.Round(outG * 255))
			dst.Pix[i+2] = uint8(math.Round(outB * 255))
			dst.Pix[i+3] = uint8(math.Round(outA * 255))
		}
	}
	return dst
}

func dim40(dst *image.NRGBA) *image.NRGBA {
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := dst.PixOffset(x, y)
			a := float64(dst.Pix[i+3]) * dimAlpha
			dst.Pix[i+3] = uint8(math.Round(a))
		}
	}
	return dst
}

func writePNG(path string, img image.Image) {
	out, err := os.Create(path)
	must(err)
	defer out.Close()
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	must(enc.Encode(out, img))
	fmt.Printf("wrote %s\n", path)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
