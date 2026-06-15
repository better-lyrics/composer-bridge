// Package icons embeds the tray icon PNGs for each platform.
package icons

import _ "embed"

//go:embed icon-mac.png
var Mac []byte

//go:embed icon.png
var Default []byte

// Per-state tray-bar variants. Mac variants are 256x256 monochrome templates
// (opaque black on transparent, state glyph as a transparent cutout from the
// inner disc) so macOS tints them per appearance and template-mode shows
// the state through the cutout. Default variants are 256x256 colored PNGs
// for non-mac platforms. Each state uses a distinct silhouette so the icon
// reads at any menu-bar background. Source SVGs live in iconset/; regenerate
// by re-exporting from there with rsvg-convert at 256x256 (mono SVGs need
// fill="white" swapped to fill="black" before render so the template path
// embeds opaque black bytes).

//go:embed tray-mac-idle.png
var MacIdle []byte

//go:embed tray-mac-downloading.png
var MacDownloading []byte

//go:embed tray-mac-downloading-dim.png
var MacDownloadingDim []byte

//go:embed tray-mac-error.png
var MacError []byte

//go:embed tray-mac-stopped.png
var MacStopped []byte

//go:embed tray-mac-update.png
var MacUpdate []byte

//go:embed tray-default-idle.png
var DefaultIdle []byte

//go:embed tray-default-downloading.png
var DefaultDownloading []byte

//go:embed tray-default-downloading-dim.png
var DefaultDownloadingDim []byte

//go:embed tray-default-error.png
var DefaultError []byte

//go:embed tray-default-stopped.png
var DefaultStopped []byte

//go:embed tray-default-update.png
var DefaultUpdate []byte

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
