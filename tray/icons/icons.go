// Package icons embeds the tray icon PNGs for each platform.
package icons

import _ "embed"

//go:embed icon-mac.png
var Mac []byte

//go:embed icon.png
var Default []byte

// Per-state tray-bar variants. Mac variants are 44x44 monochrome templates
// (alpha-only, opaque black) so macOS tints them per appearance. Default
// variants are 44x44 colored PNGs for non-mac platforms. Idle is the plain
// silhouette; Downloading and Error add a small filled badge dot in the
// lower-right corner; Stopped multiplies alpha by 0.4 so the glyph dims.
// Regenerate via `go run ./internal/cmd/tray-icon-gen`.

//go:embed tray-mac-idle.png
var MacIdle []byte

//go:embed tray-mac-downloading.png
var MacDownloading []byte

//go:embed tray-mac-error.png
var MacError []byte

//go:embed tray-mac-stopped.png
var MacStopped []byte

//go:embed tray-default-idle.png
var DefaultIdle []byte

//go:embed tray-default-downloading.png
var DefaultDownloading []byte

//go:embed tray-default-error.png
var DefaultError []byte

//go:embed tray-default-stopped.png
var DefaultStopped []byte

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
