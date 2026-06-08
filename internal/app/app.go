// Package app is the Wails-bound surface exposed to the React frontend.
// Every public method on *App is auto-generated as a JS function by Wails
// at build time; signatures and error semantics here are the public contract.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/boidushya/composer-bridge/internal/activity"
	"github.com/boidushya/composer-bridge/internal/config"
	"github.com/boidushya/composer-bridge/internal/library"
	"github.com/boidushya/composer-bridge/internal/ytdlp"
)

// App wires the bridge's storage and config into Wails-callable methods.
type App struct {
	library   *library.Library
	activity  *activity.Log
	cfg       config.Config
	cfgPath   string
	dataDir   string
	ytdlpPath string
	thumbDir  string
	downloadDir string
	logPath   string
	version   string
	ctx       context.Context
}

// New builds an App. Caller retains ownership of lib and act: App does not close them.
func New(lib *library.Library, act *activity.Log, cfg config.Config, cfgPath, dataDir, ytdlpPath, version string) *App {
	return &App{
		library:     lib,
		activity:    act,
		cfg:         cfg,
		cfgPath:     cfgPath,
		dataDir:     dataDir,
		ytdlpPath:   ytdlpPath,
		thumbDir:    filepath.Join(dataDir, "thumbs"),
		downloadDir: resolveDownloadDir(cfg.DownloadDir),
		logPath:     filepath.Join(dataDir, "bridge.log"),
		version:     version,
	}
}

func resolveDownloadDir(configured string) string {
	if configured != "" {
		return configured
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Music", "Composer Bridge")
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

// RemoveTrack deletes the track matching videoID and any cached audio/thumbnail on disk.
func (a *App) RemoveTrack(videoID string) error {
	track, err := a.library.GetTrack(videoID)
	if err == nil && track != nil {
		if track.AudioPath != "" {
			_ = os.Remove(track.AudioPath)
		}
		if track.ThumbPath != "" {
			_ = os.Remove(track.ThumbPath)
		}
	}
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
	a.downloadDir = resolveDownloadDir(cfg.DownloadDir)
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

// YtdlpVersion returns the version string of the bundled yt-dlp binary, or "unknown".
func (a *App) YtdlpVersion() string {
	if a.ytdlpPath == "" {
		return "unknown"
	}
	return ytdlp.Version(a.ytdlpPath)
}

// LibrarySize returns the total on-disk audio size in bytes across all imported tracks.
func (a *App) LibrarySize() (int64, error) {
	tracks, err := a.library.ListTracks()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, t := range tracks {
		total += t.AudioSize
	}
	return total, nil
}

// ThumbCacheSize returns the total bytes used by cached thumbnail files.
func (a *App) ThumbCacheSize() (int64, error) {
	var total int64
	err := filepath.Walk(a.thumbDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	return total, nil
}

// ForceYtdlpUpdate re-downloads the yt-dlp binary unconditionally. Returns the
// version string of the freshly-installed binary.
func (a *App) ForceYtdlpUpdate() (string, error) {
	if a.ytdlpPath != "" {
		_ = os.Remove(a.ytdlpPath)
	}
	path, err := ytdlp.Ensure(a.dataDir)
	if err != nil {
		return "", err
	}
	a.ytdlpPath = path
	return ytdlp.Version(path), nil
}

// DownloadAudio fetches audio for videoID using the configured format and stores
// it under DownloadDir. Updates the library row with the resulting path + size and
// returns the refreshed track.
func (a *App) DownloadAudio(videoID string) (*library.Track, error) {
	track, err := a.library.GetTrack(videoID)
	if err != nil {
		return nil, err
	}
	if a.downloadDir == "" {
		return nil, fmt.Errorf("download directory is not configured")
	}
	if err := os.MkdirAll(a.downloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("create download dir: %w", err)
	}
	format := a.cfg.AudioFormat
	if format == "" {
		format = "opus"
	}
	ext := ytdlp.FormatExtension(format)
	dest := filepath.Join(a.downloadDir, sanitizeFilename(track.Title)+"."+ext)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	actID := a.startActivity(activity.KindAudioDownload, videoID)
	size, err := ytdlp.DownloadToFile(ctx, a.ytdlpPath, videoID, format, dest)
	if err != nil {
		a.endActivity(actID, activity.StatusError, err.Error())
		return nil, err
	}
	if err := a.library.MarkAudioDownloaded(videoID, dest, size); err != nil {
		a.endActivity(actID, activity.StatusError, err.Error())
		return nil, err
	}
	a.endActivity(actID, activity.StatusOK, "")
	return a.library.GetTrack(videoID)
}

// OpenLogFile returns the absolute path to the bridge log file. The frontend
// opens it via the OS shell using the Wails runtime BrowserOpenURL helper.
func (a *App) OpenLogFile() string {
	return "file://" + a.logPath
}

// BuildDiagnosticReport produces a copy-pasteable diagnostics string covering
// bridge version, yt-dlp version, platform, config (with secrets stripped), and
// the last ~20 activity rows.
func (a *App) BuildDiagnosticReport() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Composer Bridge diagnostics\n")
	fmt.Fprintf(&b, "===========================\n")
	fmt.Fprintf(&b, "bridge version: %s\n", a.version)
	fmt.Fprintf(&b, "yt-dlp version: %s\n", a.YtdlpVersion())
	fmt.Fprintf(&b, "platform:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "data dir:       %s\n", a.dataDir)
	fmt.Fprintf(&b, "thumb dir:      %s\n", a.thumbDir)
	fmt.Fprintf(&b, "download dir:   %s\n", a.downloadDir)
	fmt.Fprintf(&b, "log file:       %s\n", a.logPath)
	fmt.Fprintf(&b, "\nConfig:\n")
	fmt.Fprintf(&b, "  listen_port:      %d\n", a.cfg.ListenPort)
	fmt.Fprintf(&b, "  audio_format:     %s\n", a.cfg.AudioFormat)
	fmt.Fprintf(&b, "  audio_quality:    %s\n", a.cfg.AudioQuality)
	fmt.Fprintf(&b, "  max_concurrent:   %d\n", a.cfg.MaxConcurrent)
	fmt.Fprintf(&b, "  log_level:        %s\n", a.cfg.LogLevel)
	fmt.Fprintf(&b, "  ytdlp_channel:    %s\n", a.cfg.YtdlpChannel)
	fmt.Fprintf(&b, "  allowed_origins:  %s\n", strings.Join(a.cfg.AllowedOrigins, ", "))

	entries, err := a.activity.Recent(20)
	if err == nil {
		fmt.Fprintf(&b, "\nRecent activity (newest first):\n")
		for _, e := range entries {
			fmt.Fprintf(&b, "  [%s] %s %s -> %s %s\n",
				time.UnixMilli(e.StartedAt).Format(time.RFC3339),
				e.Kind, e.VideoID, e.Status, e.Message)
		}
	}
	return b.String(), nil
}

// LibrarySize, ThumbCacheSize, and BuildDiagnosticReport are pure-read helpers;
// they don't touch shared state and are safe to call concurrently.

func (a *App) startActivity(kind activity.Kind, videoID string) int64 {
	id, err := a.activity.Start(kind, videoID)
	if err != nil {
		return 0
	}
	return id
}

func (a *App) endActivity(id int64, status activity.Status, message string) {
	if id == 0 {
		return
	}
	_ = a.activity.End(id, status, message)
}

func sanitizeFilename(name string) string {
	if name == "" {
		return "track"
	}
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	out := repl.Replace(name)
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}
