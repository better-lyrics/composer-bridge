//go:build !bindings

package main

import (
	"context"

	"github.com/better-lyrics/composer-bridge/internal/ytdlp"
)

// bootstrapYtdlp downloads yt-dlp on first run and schedules the daily
// refresh. Skipped under the `bindings` build tag (used by wails build's
// "Generating bindings" phase) because that phase runs main() just to
// introspect bound types and shouldn't hit GitHub's release API.
func bootstrapYtdlp(dataDir string) (string, error) {
	return ytdlp.Ensure(dataDir)
}

func scheduleYtdlpRefresh(ctx context.Context, dataDir string) {
	go ytdlp.RefreshDaily(ctx, dataDir)
}
