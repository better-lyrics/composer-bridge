#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APPICON="$ROOT/build/appicon"
DARWIN="$ROOT/build/darwin"
WINDOWS="$ROOT/build/windows"
FRONTEND_PUBLIC="$ROOT/frontend/public"

RSVG="/opt/homebrew/bin/rsvg-convert"
ICONUTIL="/usr/bin/iconutil"

if [ ! -x "$RSVG" ]; then
  echo "error: rsvg-convert not found at $RSVG" >&2
  exit 1
fi

if [ ! -f "$APPICON/icon.svg" ] || [ ! -f "$APPICON/icon-mono.svg" ]; then
  echo "error: source SVGs missing in $APPICON" >&2
  exit 1
fi

mkdir -p "$APPICON" "$DARWIN" "$WINDOWS" "$FRONTEND_PUBLIC"

render_red() {
  local size="$1" out="$2"
  "$RSVG" -w "$size" -h "$size" "$APPICON/icon.svg" -o "$out"
}

render_mono() {
  local size="$1" out="$2"
  "$RSVG" -w "$size" -h "$size" "$APPICON/icon-mono.svg" -o "$out"
}

for size in 16 32 48 64 128 256 512 1024; do
  render_red "$size" "$APPICON/icon-${size}.png"
done

render_red 1024 "$APPICON/appicon.png"
cp "$APPICON/appicon.png" "$ROOT/build/appicon.png"

render_mono 16 "$APPICON/menubar-16.png"
render_mono 32 "$APPICON/menubar-32.png"
render_red  16 "$APPICON/tray-16.png"
render_red  32 "$APPICON/tray-32.png"

ICONSET="$DARWIN/AppIcon.iconset"
rm -rf "$ICONSET"
mkdir -p "$ICONSET"

render_red   16 "$ICONSET/icon_16x16.png"
render_red   32 "$ICONSET/icon_16x16@2x.png"
render_red   32 "$ICONSET/icon_32x32.png"
render_red   64 "$ICONSET/icon_32x32@2x.png"
render_red  128 "$ICONSET/icon_128x128.png"
render_red  256 "$ICONSET/icon_128x128@2x.png"
render_red  256 "$ICONSET/icon_256x256.png"
render_red  512 "$ICONSET/icon_256x256@2x.png"
render_red  512 "$ICONSET/icon_512x512.png"
render_red 1024 "$ICONSET/icon_512x512@2x.png"

rm -f "$DARWIN/AppIcon.icns"
"$ICONUTIL" -c icns "$ICONSET" -o "$DARWIN/AppIcon.icns"
rm -rf "$ICONSET"

for size in 16 24 32 48 64 256; do
  render_red "$size" "$WINDOWS/icon-${size}.png"
done

if command -v icotool >/dev/null 2>&1; then
  icotool -c -o "$WINDOWS/icon.ico" \
    "$WINDOWS/icon-16.png" \
    "$WINDOWS/icon-24.png" \
    "$WINDOWS/icon-32.png" \
    "$WINDOWS/icon-48.png" \
    "$WINDOWS/icon-64.png" \
    "$WINDOWS/icon-256.png"
  rm -f "$WINDOWS/MAKE_ICO.md"
else
  rm -f "$WINDOWS/icon.ico"
  cat > "$WINDOWS/MAKE_ICO.md" <<'EOF'
# Windows .ico assembly

`icotool` was not on PATH when icons were regenerated, so the PNG layers in this
directory have not been packed into a single `icon.ico`. Install `icoutils` and
run the one-liner below.

```
brew install icoutils
icotool -c -o icon.ico icon-16.png icon-24.png icon-32.png icon-48.png icon-64.png icon-256.png
```

Any equivalent tool works as well (for example ImageMagick:
`magick convert icon-16.png icon-24.png icon-32.png icon-48.png icon-64.png icon-256.png icon.ico`).

Re-running `scripts/regen-icons.sh` after installing `icotool` will produce the
`.ico` automatically and remove this note.
EOF
fi

render_red 512 "$FRONTEND_PUBLIC/favicon.png"
render_red  32 "$FRONTEND_PUBLIC/favicon-32.png"

echo "Icon regeneration complete."
