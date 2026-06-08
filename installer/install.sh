#!/bin/sh
# Composer Bridge installer (POSIX sh, no bash).
# Downloads the latest (or pinned) release from GitHub, verifies the sha256
# against the published manifest.json, and installs.
#
# Env:
#   VERSION      pin a specific release tag (e.g. v0.2.1). Default: latest.
#   INSTALL_DIR  Linux install dir. Default: $HOME/.local/bin.
#
# Exit codes:
#   0 success
#   1 unsupported platform
#   2 download failed
#   3 checksum mismatch
#   4 install step failed

set -eu

REPO="better-lyrics/composer-bridge"
PREFIX="[install]"
TMPDIR_ROOT=""

log() {
    printf '%s %s\n' "$PREFIX" "$*"
}

err() {
    printf '%s error: %s\n' "$PREFIX" "$*" >&2
}

usage() {
    cat <<EOF
Composer Bridge installer

Usage:
  install.sh           Install the latest release for the current OS/arch.
  install.sh --help    Show this help and exit.

Environment:
  VERSION              Pin a specific release tag (e.g. v0.2.1). Defaults to latest.
  INSTALL_DIR          Linux only. Defaults to \$HOME/.local/bin.

Exit codes:
  0 success
  1 unsupported platform
  2 download failed
  3 checksum mismatch
  4 install step failed
EOF
}

cleanup() {
    if [ -n "$TMPDIR_ROOT" ] && [ -d "$TMPDIR_ROOT" ]; then
        rm -rf "$TMPDIR_ROOT"
    fi
}
trap cleanup EXIT INT TERM

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        err "missing required command: $1"
        exit 4
    fi
}

case "${1:-}" in
    -h|--help)
        usage
        exit 0
        ;;
    "")
        ;;
    *)
        err "unknown argument: $1"
        usage >&2
        exit 4
        ;;
esac

require_cmd curl
require_cmd uname
require_cmd shasum

UNAME_S=$(uname -s)
UNAME_M=$(uname -m)

case "$UNAME_S" in
    Darwin)
        OS="darwin"
        ;;
    Linux)
        OS="linux"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        log "Windows detected. Please use the GUI installer at https://github.com/$REPO/releases/latest"
        exit 0
        ;;
    *)
        err "unsupported OS: $UNAME_S"
        exit 1
        ;;
esac

case "$UNAME_M" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        err "unsupported architecture: $UNAME_M"
        exit 1
        ;;
esac

log "detected $OS/$ARCH"

VERSION="${VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
    BASE_URL="https://github.com/$REPO/releases/latest/download"
else
    BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
fi

TMPDIR_ROOT=$(mktemp -d 2>/dev/null || mktemp -d -t composer-bridge-install)
MANIFEST="$TMPDIR_ROOT/manifest.json"

log "fetching manifest from $BASE_URL/manifest.json"
if ! curl -fsSL "$BASE_URL/manifest.json" -o "$MANIFEST"; then
    err "could not download manifest.json"
    exit 2
fi

# Parse manifest with sed/grep (no jq dependency). The manifest layout is
# {"version":"v0.2.1", "assets": {"darwin": {"arm64": {"url":"...", "sha256":"..."}}}}.
# We extract version first, then locate the OS->ARCH block and pull url + sha256.

extract_version() {
    # First "version": "..." in the file.
    grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$MANIFEST" \
        | head -n 1 \
        | sed 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/'
}

# extract_asset_field <field>: pulls the field from the asset block matching
# the current OS/ARCH. Strategy: collapse the JSON to one line, then use sed
# to grab the inner JSON object for "$ARCH" inside the "$OS" block.
extract_asset_field() {
    field="$1"
    # Collapse whitespace and newlines to make the regex tractable.
    tr -d '\n' < "$MANIFEST" \
        | tr -s ' \t' ' ' \
        | sed -n "s/.*\"$OS\"[[:space:]]*:[[:space:]]*{[^}]*\"$ARCH\"[[:space:]]*:[[:space:]]*{\\([^}]*\\)}.*/\\1/p" \
        | grep -o "\"$field\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" \
        | head -n 1 \
        | sed "s/.*\"$field\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/"
}

RESOLVED_VERSION=$(extract_version)
ASSET_URL=$(extract_asset_field url)
ASSET_SHA=$(extract_asset_field sha256)

if [ -z "$RESOLVED_VERSION" ]; then
    err "could not read version from manifest"
    exit 2
fi
if [ -z "$ASSET_URL" ] || [ -z "$ASSET_SHA" ]; then
    err "no asset published for $OS/$ARCH in manifest"
    exit 1
fi

log "manifest version: $RESOLVED_VERSION"
log "asset url:        $ASSET_URL"
log "expected sha256:  $ASSET_SHA"

ASSET_BASENAME=$(basename "$ASSET_URL")
ASSET_PATH="$TMPDIR_ROOT/$ASSET_BASENAME"

log "downloading $ASSET_BASENAME"
if ! curl -fsSL "$ASSET_URL" -o "$ASSET_PATH"; then
    err "asset download failed: $ASSET_URL"
    exit 2
fi

log "verifying checksum"
ACTUAL_SHA=$(shasum -a 256 "$ASSET_PATH" | awk '{print $1}')
if [ -z "$ACTUAL_SHA" ]; then
    err "could not compute sha256 of downloaded asset"
    exit 3
fi
if [ "$ACTUAL_SHA" != "$ASSET_SHA" ]; then
    err "checksum mismatch"
    err "  expected: $ASSET_SHA"
    err "  actual:   $ACTUAL_SHA"
    exit 3
fi
log "checksum ok"

install_macos() {
    require_cmd hdiutil
    require_cmd xattr

    APP_NAME="Composer Bridge.app"
    MOUNT_POINT="$TMPDIR_ROOT/mnt"
    mkdir -p "$MOUNT_POINT"

    log "mounting dmg"
    if ! hdiutil attach "$ASSET_PATH" -nobrowse -quiet -mountpoint "$MOUNT_POINT"; then
        err "failed to mount dmg"
        exit 4
    fi

    if [ ! -d "$MOUNT_POINT/$APP_NAME" ]; then
        err "$APP_NAME not found inside dmg"
        hdiutil detach "$MOUNT_POINT" -quiet || true
        exit 4
    fi

    log "copying to /Applications"
    if [ -d "/Applications/$APP_NAME" ]; then
        rm -rf "/Applications/$APP_NAME"
    fi
    if ! cp -R "$MOUNT_POINT/$APP_NAME" "/Applications/"; then
        err "copy to /Applications failed (try with sudo if you hit permissions)"
        hdiutil detach "$MOUNT_POINT" -quiet || true
        exit 4
    fi

    log "unmounting dmg"
    hdiutil detach "$MOUNT_POINT" -quiet || true

    log "clearing Gatekeeper quarantine flag"
    xattr -d com.apple.quarantine "/Applications/$APP_NAME" >/dev/null 2>&1 || true

    INSTALLED_PATH="/Applications/$APP_NAME"
}

install_linux() {
    INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
    if ! mkdir -p "$INSTALL_DIR"; then
        err "could not create $INSTALL_DIR"
        exit 4
    fi

    DEST="$INSTALL_DIR/composer-bridge.AppImage"
    if ! cp "$ASSET_PATH" "$DEST"; then
        err "could not write to $DEST"
        exit 4
    fi
    chmod +x "$DEST"

    case ":${PATH:-}:" in
        *":$INSTALL_DIR:"*)
            ;;
        *)
            log "note: $INSTALL_DIR is not in your PATH"
            log "add this to your shell rc: export PATH=\"\$HOME/.local/bin:\$PATH\""
            ;;
    esac

    INSTALLED_PATH="$DEST"
}

case "$OS" in
    darwin)
        install_macos
        ;;
    linux)
        install_linux
        ;;
esac

log "composer-bridge $RESOLVED_VERSION installed at $INSTALLED_PATH"
