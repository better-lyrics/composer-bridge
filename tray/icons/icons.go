// Package icons embeds the tray icon PNGs for each platform.
package icons

import _ "embed"

//go:embed icon-mac.png
var Mac []byte

//go:embed icon.png
var Default []byte
