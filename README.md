# Composer Bridge

A small companion app for [Composer](https://composer.boidu.dev) that fetches YouTube audio through your own machine, because the cloud-based instances keep getting rate-limited or IP-blocked.

## Why this exists

Composer needs audio, and most of that audio lives on YouTube. YouTube blocks cloud IP ranges aggressively, so the public yt-dlp tunnels Composer used to depend on get throttled or hard-blocked within hours. Running yt-dlp on your home internet still works fine because you look like a normal viewer. So I packaged yt-dlp behind a tiny local HTTP server, wrapped it in a Wails app, and pointed Composer at it. Yes, this means you have a small Go binary running on your laptop forever. That is the deal.

## Install

The installer is unsigned. I am one person and Apple/Microsoft code signing costs more than this project earns (which is zero), so the first launch on every OS will show a scary warning. You have to bypass it once.

**macOS and Linux**

```
curl -fsSL https://github.com/better-lyrics/composer-bridge/releases/latest/download/install.sh | sh
```

The script verifies the sha256 of the asset against `manifest.json` before writing anything to disk. On macOS the app lands in `/Applications` with the Gatekeeper quarantine flag cleared, so the "downloaded from internet" prompt is skipped. On Linux the AppImage goes to `~/.local/bin/composer-bridge.AppImage`. Set `INSTALL_DIR` to override the path, or `VERSION=v0.2.0` to pin a specific release.

**Windows**

Grab the installer from the [latest release page](https://github.com/better-lyrics/composer-bridge/releases/latest) and double-click it. SmartScreen will block it the first time. Click "More info", then "Run anyway". This only happens once per version.

## How to use

1. Launch Composer Bridge. The app window has Library, Activity, and Settings views; leave it running in the background while you work in Composer.
2. In Composer, open Advanced settings and toggle on the experimental "Composer Bridge" option.
3. Paste a YouTube link as usual. Composer will route the fetch through the bridge.

The Library view shows every track you've fetched, with thumbnails, a search box, and inline download / open-in-Composer buttons. The Activity view is the live log of yt-dlp imports and downloads, which is handy when something fails and you want the raw stderr.

## Troubleshooting

**Bridge not detected by Composer.** Check the Composer Bridge URL field in Composer's Advanced settings. The default is `http://localhost:7777`. If something else was on 7777 already, the bridge falls back to a random free port and writes the chosen port to `~/.composer-bridge/port.txt`. Paste `http://localhost:<that port>` into Composer.

**yt-dlp out of date or failing to fetch.** YouTube changes things often, and yt-dlp usually catches up within a few days. The bridge auto-refreshes yt-dlp once a day in the background. To force an immediate update, open Settings inside the bridge and click "Update yt-dlp".

## License

AGPL-3.0. See [LICENSE](LICENSE).

## Acknowledgments

[yt-dlp](https://github.com/yt-dlp/yt-dlp) does all the heavy lifting. [Wails](https://wails.io) makes shipping a Go + React desktop app pleasant. The icon and brand colour come from [Better Lyrics](https://betterlyrics.org), whose visual identity the bridge inherits.
