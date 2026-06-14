//go:build !bindings

package main

import (
	"context"
	"log/slog"

	"github.com/better-lyrics/composer-bridge/internal/ytdlp"
)

// bootstrapYtdlp downloads yt-dlp on first run when no user override is set.
// Skipped under the `bindings` build tag (used by wails build's
// "Generating bindings" phase) because that phase runs main() just to
// introspect bound types and shouldn't hit GitHub's release API. The
// effective path is read on demand via app.GetYtdlpPath so a mid-session
// override flip is honored without restart.
func bootstrapYtdlp(ctx context.Context, dataDir, channel, override string) error {
	// When the user supplies an explicit binary path we trust it verbatim:
	// no Ensure call, no download, no stat check. They own the lifecycle.
	if override != "" {
		return nil
	}
	_, err := ytdlp.Ensure(ctx, dataDir, channel)
	return err
}

// scheduleYtdlpRefresh starts the daily upgrade poll. channelFn and
// overrideFn are forwarded into RefreshDaily, which re-reads them on every
// tick so a mid-session override flip stops the next poll without needing
// a restart. onUpgrade is forwarded so every successful tick-driven
// upgrade also refreshes main.go's cached version string.
func scheduleYtdlpRefresh(ctx context.Context, dataDir string, channelFn, overrideFn func() string, onUpgrade func(string)) {
	go ytdlp.RefreshDaily(ctx, dataDir, channelFn, overrideFn, onUpgrade)
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
