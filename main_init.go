//go:build !bindings

package main

import (
	"context"
	"log/slog"

	"github.com/better-lyrics/composer-bridge/internal/ytdlp"
)

// bootstrapYtdlp downloads yt-dlp on first run and schedules the daily
// refresh. Skipped under the `bindings` build tag (used by wails build's
// "Generating bindings" phase) because that phase runs main() just to
// introspect bound types and shouldn't hit GitHub's release API.
func bootstrapYtdlp(ctx context.Context, dataDir string) (string, error) {
	// TODO(phase-b): read channel + binary-path override from cfg.
	return ytdlp.Ensure(ctx, dataDir, "stable")
}

func scheduleYtdlpRefresh(ctx context.Context, dataDir string) {
	// TODO(phase-b): replace the constant callback with cfg.YtdlpChannel.
	go ytdlp.RefreshDaily(ctx, dataDir, func() string { return "stable" })
}

// bootstrapDeno downloads deno on first run and registers <dataDir>/bin with
// the ytdlp package so every yt-dlp invocation gets PATH augmented with that
// dir. yt-dlp needs an external JS engine to solve YouTube's n-sig
// challenges; macOS apps spawned by launchd inherit a minimal PATH that
// omits /opt/homebrew/bin, so without a bundled deno the youtube extractor
// silently returns zero formats. A failure here is logged but non-fatal:
// the rest of the app still works, just YouTube downloads will surface the
// underlying error to the user via the activity log.
func bootstrapDeno(dataDir string) {
	ytdlp.SetDenoBinDir(ytdlp.DenoBinDir(dataDir))
	if _, err := ytdlp.EnsureDeno(dataDir); err != nil {
		slog.Warn("ensure deno failed; YouTube extraction may fail until next launch",
			"err", err, "dataDir", dataDir)
	}
}
