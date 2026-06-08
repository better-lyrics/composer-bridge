// Package app is the Wails-bound surface exposed to the React frontend.
// Every public method on *App is auto-generated as a JS function by Wails
// at build time; signatures and error semantics here are the public contract.
package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/autostart"
	"github.com/better-lyrics/composer-bridge/internal/config"
	"github.com/better-lyrics/composer-bridge/internal/library"
	"github.com/better-lyrics/composer-bridge/internal/ytdlp"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// activeApp holds the most recently constructed App so package-level callers
// (Wails SingleInstanceLock handler, tray callbacks) can reach it without an
// explicit handle. Populated by New so callbacks that fire before OnStartup
// can still find the App. Reads happen from arbitrary goroutines so the
// registry is guarded by activeMu.
var (
	activeMu  sync.RWMutex
	activeApp *App
)

// App wires the bridge's storage and config into Wails-callable methods.
// Wails dispatches JS calls on separate goroutines, so any field a method both
// reads and writes needs mutex protection. mu guards cfg, downloadDir, and
// ytdlpPath; everything else is set once in New and never mutated.
type App struct {
	library     *library.Library
	activity    *activity.Log
	cfgPath     string
	dataDir     string
	thumbDir    string
	logPath     string
	version     string
	ctx         context.Context
	hideWindow  func(context.Context)
	mu          sync.RWMutex
	cfg         config.Config
	downloadDir string
	ytdlpPath   string
}

// New builds an App. Caller retains ownership of lib and act: App does not close them.
// The new App is installed into the package-level active registry before returning so
// callbacks that fire before Wails's OnStartup (e.g. SingleInstanceLock from a separate
// goroutine on app boot) can still find it.
func New(lib *library.Library, act *activity.Log, cfg config.Config, cfgPath, dataDir, ytdlpPath, version string) *App {
	a := &App{
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
		hideWindow:  wailsRuntime.WindowHide,
	}
	activeMu.Lock()
	activeApp = a
	activeMu.Unlock()
	return a
}

func resolveDownloadDir(configured string) string {
	if configured != "" {
		return configured
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Music", "Composer")
}

// Startup stashes the Wails runtime context so later methods can emit events to JS.
// The package-level active registry is populated by New so callbacks that fire
// before OnStartup can still find the App; Startup only needs to bind the ctx.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// Shutdown is a no-op: library and activity handles are owned by main.go.
func (a *App) Shutdown(_ context.Context) {}

// Ctx returns the Wails runtime context captured in Startup. May be nil before
// Startup runs.
func (a *App) Ctx() context.Context {
	return a.ctx
}

// OnBeforeClose is wired into options.App.OnBeforeClose. Returning true tells
// Wails to NOT quit the app: we hide the window and stay running in the tray.
func (a *App) OnBeforeClose(_ context.Context) bool {
	if a.ctx != nil && a.hideWindow != nil {
		a.hideWindow(a.ctx)
	}
	return true
}

// Active returns the most recently constructed App, or nil if none has been
// built yet. Safe for concurrent use; intended for Wails callbacks
// (SingleInstanceLock, tray handlers) that have no direct handle to the App
// instance.
func Active() *App {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return activeApp
}

// resetActiveForTesting clears the package-level registry. Test code calls
// this from t.Cleanup so per-test state does not leak.
func resetActiveForTesting() {
	activeMu.Lock()
	activeApp = nil
	activeMu.Unlock()
}

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
// Paths from the library are checked against downloadDir/thumbDir before removal so
// a corrupted DB row can't trick the bridge into deleting arbitrary files.
func (a *App) RemoveTrack(videoID string) error {
	track, err := a.library.GetTrack(videoID)
	if err == nil && track != nil {
		a.mu.RLock()
		downloadDir := a.downloadDir
		a.mu.RUnlock()
		if track.AudioPath != "" && pathIsUnder(track.AudioPath, downloadDir) {
			_ = os.Remove(track.AudioPath)
		}
		if track.ThumbPath != "" && pathIsUnder(track.ThumbPath, a.thumbDir) {
			_ = os.Remove(track.ThumbPath)
		}
	}
	return a.library.RemoveTrack(videoID)
}

// pathIsUnder reports whether path resolves to a location inside root. Both are
// cleaned via filepath.Abs before comparison; empty root rejects everything.
func pathIsUnder(path, root string) bool {
	if root == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg
}

// SaveConfig persists cfg to disk and updates the in-memory copy. Changes that affect
// the HTTP listener (ListenPort, AllowedOrigins) only take effect on the next bridge
// restart: the running server is not reconfigured in-place. When OpenAtLogin flips
// it also writes / removes the platform autostart entry.
func (a *App) SaveConfig(cfg config.Config) error {
	if err := config.Save(a.cfgPath, cfg); err != nil {
		return err
	}
	a.mu.Lock()
	prevOpenAtLogin := a.cfg.OpenAtLogin
	a.cfg = cfg
	a.downloadDir = resolveDownloadDir(cfg.DownloadDir)
	a.mu.Unlock()
	if cfg.OpenAtLogin != prevOpenAtLogin {
		if err := autostart.SetEnabled(cfg.OpenAtLogin, currentExecPath()); err != nil {
			return fmt.Errorf("apply open-at-login: %w", err)
		}
	}
	return nil
}

func currentExecPath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	return p
}

// SupportsAutostart reports whether the current platform has a working
// autostart implementation. Now true on darwin (LaunchAgent), windows (HKCU
// Run key), and linux (XDG autostart .desktop). The frontend keeps its
// platform gate but the answer is always yes today.
func (a *App) SupportsAutostart() bool {
	return true
}

// OpenInComposer returns the Composer deep-link URL for videoID. Param names
// match Composer's useImportFromQuery handler (title / artist / album / duration
// / videoId) so the lyrics-import modal pre-fills. Metadata is pulled from the
// library when the track is known; otherwise only videoId is set.
func (a *App) OpenInComposer(videoID string) string {
	u, err := url.Parse("https://composer.boidu.dev/")
	if err != nil {
		return "https://composer.boidu.dev/?videoId=" + url.QueryEscape(videoID)
	}
	q := u.Query()
	q.Set("videoId", videoID)
	if track, err := a.library.GetTrack(videoID); err == nil && track != nil {
		if track.Title != "" {
			q.Set("title", track.Title)
		}
		if track.Artist != "" {
			q.Set("artist", track.Artist)
		}
		if track.Album != "" {
			q.Set("album", track.Album)
		}
		if track.DurationSec > 0 {
			q.Set("duration", strconv.Itoa(track.DurationSec))
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
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
	a.mu.RLock()
	path := a.ytdlpPath
	a.mu.RUnlock()
	if path == "" {
		return "unknown"
	}
	return ytdlp.Version(path)
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
	a.mu.RLock()
	prev := a.ytdlpPath
	a.mu.RUnlock()
	if prev != "" {
		_ = os.Remove(prev)
	}
	path, err := ytdlp.Ensure(a.dataDir)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.ytdlpPath = path
	a.mu.Unlock()
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
	a.mu.RLock()
	downloadDir := a.downloadDir
	ytdlpPath := a.ytdlpPath
	format := a.cfg.AudioFormat
	a.mu.RUnlock()
	if downloadDir == "" {
		return nil, fmt.Errorf("download directory is not configured")
	}
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return nil, fmt.Errorf("create download dir: %w", err)
	}
	if format == "" {
		format = "opus"
	}
	ext := ytdlp.FormatExtension(format)
	dest := filepath.Join(downloadDir, sanitizeFilename(track.Title)+"."+ext)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	actID := a.startActivity(activity.KindAudioDownload, videoID)
	size, err := ytdlp.DownloadToFile(ctx, ytdlpPath, videoID, format, dest)
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
	a.mu.RLock()
	cfg := a.cfg
	downloadDir := a.downloadDir
	a.mu.RUnlock()
	var b strings.Builder
	fmt.Fprintf(&b, "Composer Bridge diagnostics\n")
	fmt.Fprintf(&b, "===========================\n")
	fmt.Fprintf(&b, "bridge version: %s\n", a.version)
	fmt.Fprintf(&b, "yt-dlp version: %s\n", a.YtdlpVersion())
	fmt.Fprintf(&b, "platform:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "data dir:       %s\n", a.dataDir)
	fmt.Fprintf(&b, "thumb dir:      %s\n", a.thumbDir)
	fmt.Fprintf(&b, "download dir:   %s\n", downloadDir)
	fmt.Fprintf(&b, "log file:       %s\n", a.logPath)
	fmt.Fprintf(&b, "\nConfig:\n")
	fmt.Fprintf(&b, "  listen_port:      %d\n", cfg.ListenPort)
	fmt.Fprintf(&b, "  audio_format:     %s\n", cfg.AudioFormat)
	fmt.Fprintf(&b, "  audio_quality:    %s\n", cfg.AudioQuality)
	fmt.Fprintf(&b, "  max_concurrent:   %d\n", cfg.MaxConcurrent)
	fmt.Fprintf(&b, "  log_level:        %s\n", cfg.LogLevel)
	fmt.Fprintf(&b, "  ytdlp_channel:    %s\n", cfg.YtdlpChannel)
	fmt.Fprintf(&b, "  allowed_origins:  %s\n", strings.Join(cfg.AllowedOrigins, ", "))

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
