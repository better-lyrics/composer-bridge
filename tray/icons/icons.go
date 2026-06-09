// Package icons embeds the tray icon PNGs for each platform.
package icons

import _ "embed"

//go:embed icon-mac.png
var Mac []byte

//go:embed icon.png
var Default []byte

// Menu glyphs (16x16 monochrome) rendered next to each tray menu item.

//go:embed menu/window.png
var MenuWindow []byte

//go:embed menu/clock.png
var MenuClock []byte

//go:embed menu/power.png
var MenuPower []byte

//go:embed menu/gear.png
var MenuGear []byte

//go:embed menu/x.png
var MenuX []byte

//go:embed menu/dot.png
var MenuDot []byte
