// Package app is the Wails-bound surface exposed to the React frontend.
// Every public method on *App is auto-generated as a JS function by Wails
// at build time; signatures and error semantics here are the public contract.
package app

import (
	"context"
	"fmt"

	"github.com/boidushya/composer-bridge/internal/activity"
	"github.com/boidushya/composer-bridge/internal/config"
	"github.com/boidushya/composer-bridge/internal/library"
)

// App wires the bridge's storage and config into Wails-callable methods.
type App struct {
	library  *library.Library
	activity *activity.Log
	cfg      config.Config
	cfgPath  string
	version  string
	ctx      context.Context
}

// New builds an App. Caller retains ownership of lib and act: App does not close them.
func New(lib *library.Library, act *activity.Log, cfg config.Config, cfgPath, version string) *App {
	return &App{
		library:  lib,
		activity: act,
		cfg:      cfg,
		cfgPath:  cfgPath,
		version:  version,
	}
}

// Startup stashes the Wails runtime context so later methods can emit events to JS.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// Shutdown is a no-op: library and activity handles are owned by main.go.
func (a *App) Shutdown(_ context.Context) {}

// ListTracks returns every track, newest import first.
func (a *App) ListTracks() ([]library.Track, error) {
	tracks, err := a.library.ListTracks()
	if err != nil {
		return nil, err
	}
	if tracks == nil {
		tracks = []library.Track{}
	}
	return tracks, nil
}

// GetTrack returns the track matching videoID. Returns library.ErrNotFound when missing.
func (a *App) GetTrack(videoID string) (*library.Track, error) {
	return a.library.GetTrack(videoID)
}

// RemoveTrack deletes the track matching videoID. Returns library.ErrNotFound when missing.
func (a *App) RemoveTrack(videoID string) error {
	return a.library.RemoveTrack(videoID)
}

// RecentActivity returns the most recent activity rows, newest first.
func (a *App) RecentActivity(limit int) ([]activity.Entry, error) {
	entries, err := a.activity.Recent(limit)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []activity.Entry{}
	}
	return entries, nil
}

// GetConfig returns the in-memory copy of the bridge config.
func (a *App) GetConfig() config.Config {
	return a.cfg
}

// SaveConfig persists cfg to disk and updates the in-memory copy. Changes that affect
// the HTTP listener (ListenPort, AllowedOrigins) only take effect on the next bridge
// restart: the running server is not reconfigured in-place.
func (a *App) SaveConfig(cfg config.Config) error {
	if err := config.Save(a.cfgPath, cfg); err != nil {
		return err
	}
	a.cfg = cfg
	return nil
}

// OpenInComposer returns the Composer deep-link URL for videoID. The frontend is
// expected to open it via the Wails runtime, keeping this side pure for testability.
func (a *App) OpenInComposer(videoID string) string {
	return fmt.Sprintf("https://composer.boidu.dev/?yt=%s", videoID)
}

// OpenInYouTube returns the canonical YouTube watch URL for videoID.
func (a *App) OpenInYouTube(videoID string) string {
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
}

// BridgeVersion returns the bridge version reported to the UI.
func (a *App) BridgeVersion() string {
	return a.version
}
