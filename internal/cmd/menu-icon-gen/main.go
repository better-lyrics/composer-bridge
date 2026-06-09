// One-shot helper: draw 6 simple monochrome 16x16 glyph PNGs into
// tray/icons/menu/. Each PNG has an opaque black foreground on a transparent
// background. macOS will template-tint them via SetTemplateIcon; other
// platforms render the black glyph directly via SetIcon. Run via:
//
//	go run ./internal/cmd/menu-icon-gen
//
// Deterministic and idempotent; safe to re-run any time.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	dim    = 16
	outDir = "tray/icons/menu"
)

var black = color.NRGBA{0, 0, 0, 255}

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	write("window.png", drawWindow())
	write("clock.png", drawClock())
	write("power.png", drawPower())
	write("gear.png", drawGear())
	write("x.png", drawX())
	write("dot.png", drawDot())
}

func write(name string, img *image.NRGBA) {
	path := filepath.Join(outDir, name)
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s 16x16\n", path)
}

func newCanvas() *image.NRGBA {
	return image.NewNRGBA(image.Rect(0, 0, dim, dim))
}

func setPx(img *image.NRGBA, x, y int) {
	if x < 0 || x >= dim || y < 0 || y >= dim {
		return
	}
	img.SetNRGBA(x, y, black)
}

func hLine(img *image.NRGBA, x0, x1, y int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		setPx(img, x, y)
	}
}

func vLine(img *image.NRGBA, x, y0, y1 int) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		setPx(img, x, y)
	}
}

func line(img *image.NRGBA, x0, y0, x1, y1 int) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		setPx(img, x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func circleOutline(img *image.NRGBA, cx, cy, r int) {
	for deg := 0; deg < 360; deg++ {
		rad := float64(deg) * math.Pi / 180.0
		x := cx + int(math.Round(float64(r)*math.Cos(rad)))
		y := cy + int(math.Round(float64(r)*math.Sin(rad)))
		setPx(img, x, y)
	}
}

func filledCircle(img *image.NRGBA, cx, cy, r int) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r*r {
				setPx(img, x, y)
			}
		}
	}
}

func filledRect(img *image.NRGBA, x0, y0, x1, y1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			setPx(img, x, y)
		}
	}
}

// drawWindow: rounded rectangle outline with a titlebar separator.
func drawWindow() *image.NRGBA {
	img := newCanvas()
	hLine(img, 3, 12, 2)
	hLine(img, 3, 12, 13)
	vLine(img, 2, 3, 12)
	vLine(img, 13, 3, 12)
	hLine(img, 3, 12, 5)
	return img
}

// drawClock: circle outline with a clock hand pointing up + right.
func drawClock() *image.NRGBA {
	img := newCanvas()
	circleOutline(img, 8, 8, 6)
	line(img, 8, 8, 8, 4)
	line(img, 8, 8, 11, 8)
	return img
}

// drawPower: arc with a vertical line through the top.
func drawPower() *image.NRGBA {
	img := newCanvas()
	for deg := 35; deg <= 325; deg++ {
		rad := float64(deg) * math.Pi / 180.0
		x := 8 + int(math.Round(5*math.Sin(rad)))
		y := 9 + int(math.Round(5*math.Cos(rad)))
		setPx(img, x, y)
	}
	vLine(img, 8, 2, 8)
	return img
}

// drawGear: a center circle with 8 small rectangles around it.
func drawGear() *image.NRGBA {
	img := newCanvas()
	circleOutline(img, 8, 8, 4)
	circleOutline(img, 8, 8, 3)
	filledRect(img, 7, 0, 8, 2)
	filledRect(img, 7, 13, 8, 15)
	filledRect(img, 0, 7, 2, 8)
	filledRect(img, 13, 7, 15, 8)
	filledRect(img, 2, 2, 3, 3)
	filledRect(img, 12, 2, 13, 3)
	filledRect(img, 2, 12, 3, 13)
	filledRect(img, 12, 12, 13, 13)
	return img
}

// drawX: two crossed lines.
func drawX() *image.NRGBA {
	img := newCanvas()
	for i := 0; i < 11; i++ {
		setPx(img, 3+i, 3+i)
		setPx(img, 13-i, 3+i)
	}
	return img
}

// drawDot: small filled circle centered in the 16x16 canvas.
func drawDot() *image.NRGBA {
	img := newCanvas()
	filledCircle(img, 8, 8, 3)
	return img
}
