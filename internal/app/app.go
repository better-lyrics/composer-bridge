// Package app is the Wails-bound surface exposed to the React frontend.
// Every public method on *App is auto-generated as a JS function by Wails
// at build time; signatures and error semantics here are the public contract.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/autostart"
	"github.com/better-lyrics/composer-bridge/internal/bridge"
	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
	"github.com/better-lyrics/composer-bridge/internal/config"
	"github.com/better-lyrics/composer-bridge/internal/library"
	"github.com/better-lyrics/composer-bridge/internal/server"
	"github.com/better-lyrics/composer-bridge/internal/ytdlp"
	"github.com/better-lyrics/composer-bridge/tray"

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
	library       *library.Library
	activity      *activity.Log
	cfgPath       string
	dataDir       string
	thumbDir      string
	logPath       string
	version       string
	ctx           context.Context
	hideWindow    func(context.Context)
	showWindow    func(context.Context)
	ytdlpVersion  func() string
	state         *bridgestate.Holder
	bridge        *bridge.Bridge
	statusEmitter func(ctx context.Context, name string, data any)
	unsubStatus   func()
	quitting      atomic.Int32
	mu            sync.RWMutex
	cfg           config.Config
	downloadDir   string
	ytdlpPath     string
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
		showWindow:  wailsRuntime.WindowShow,
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
// On macOS Startup also flips the activation policy to Regular and shows the
// window: the app launches as Accessory (LSUIElement) so there is no Dock-icon
// flash during the brief window Wails's own AppDelegate forces Regular on us.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	tray.DockShow()
	if a.showWindow != nil {
		a.showWindow(ctx)
	}
	a.mu.RLock()
	state := a.state
	emit := a.statusEmitter
	a.mu.RUnlock()
	if state != nil && emit != nil {
		unsub := state.OnChange(func(s bridgestate.State) {
			emit(ctx, "bridge:status", s)
		})
		a.mu.Lock()
		a.unsubStatus = unsub
		a.mu.Unlock()
	}
}

// MarkQuitting flips an atomic flag the tray's Quit menu sets before calling
// runtime.Quit. OnBeforeClose reads it to decide whether to let the quit
// proceed (true: real quit) or intercept and hide instead (false: X button).
func (a *App) MarkQuitting() {
	a.quitting.Store(1)
}

// Shutdown releases any background subscriptions set up in Startup. Library
// and activity handles are owned by main.go and are not closed here.
func (a *App) Shutdown(_ context.Context) {
	a.mu.Lock()
	unsub := a.unsubStatus
	a.unsubStatus = nil
	a.mu.Unlock()
	if unsub != nil {
		unsub()
	}
}

// Ctx returns the Wails runtime context captured in Startup. May be nil before
// Startup runs.
func (a *App) Ctx() context.Context {
	return a.ctx
}

// OnBeforeClose is wired into options.App.OnBeforeClose. Two paths land here:
// the window's red close button (we want to hide the window and drop the Dock
// icon) and Wails Quit (Cmd+Q or runtime.Quit via the tray's Quit menu, where
// we want the app to actually terminate). The tray's Quit handler flips the
// quitting flag before calling runtime.Quit; this read tells the two apart.
func (a *App) OnBeforeClose(_ context.Context) bool {
	if a.quitting.Load() == 1 {
		return false
	}
	if a.ctx != nil && a.hideWindow != nil {
		a.hideWindow(a.ctx)
		tray.DockHide()
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
	// The Settings textarea sends origins as a single comma-separated string;
	// normalize before persisting so the on-disk shape stays a clean array.
	cfg.AllowedOrigins = config.SplitAndCleanOrigins(cfg.AllowedOrigins)
	// ServerEnabled is owned by StartServer/StopServer (tray + Settings
	// toggle) and is not exposed in the SaveConfig form. Preserve the
	// server-side authoritative value so a stale form submit cannot undo a
	// toggle that happened while the form was open.
	a.mu.RLock()
	cfg.ServerEnabled = a.cfg.ServerEnabled
	a.mu.RUnlock()
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

// SetYtdlpVersionFn installs a cached-version callback. main.go primes the
// cache once at startup (off the goroutine) so reads here never re-exec the
// binary. Falls back to a one-shot Version() call if the callback is nil.
func (a *App) SetYtdlpVersionFn(fn func() string) {
	a.mu.Lock()
	a.ytdlpVersion = fn
	a.mu.Unlock()
}

// SetBridgeState wires the holder that Wails-bound status methods read from
// and that Startup subscribes to for event emission. Called by main.go before
// wails.Run so the rest of the app sees a non-nil holder.
func (a *App) SetBridgeState(state *bridgestate.Holder) {
	a.mu.Lock()
	a.state = state
	a.mu.Unlock()
}

// SetBridge installs the controllable HTTP bridge used by StartServer and
// StopServer. Called by main.go before wails.Run.
func (a *App) SetBridge(br *bridge.Bridge) {
	a.mu.Lock()
	a.bridge = br
	a.mu.Unlock()
}

// SetStatusEmitter installs the function used to forward bridgestate changes
// to the frontend. main.go injects wailsRuntime.EventsEmit; tests inject a
// recording fake.
func (a *App) SetStatusEmitter(emit func(ctx context.Context, name string, data any)) {
	a.mu.Lock()
	a.statusEmitter = emit
	a.mu.Unlock()
}

// DownloadDir returns the absolute path of the user-configured audio download
// root. Read by the cache-first branch of the /audio/{id} handler (via the
// callback in main.go) to decide whether a track's library-recorded AudioPath
// is a valid cache hit. Lock-protected because SaveConfig can mutate this at
// runtime.
func (a *App) DownloadDir() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.downloadDir
}

// CookiesPath returns the absolute path of the active cookies file, or ""
// when cookies are disabled or absent. Used by the HTTP handlers (via the
// callback in main.go).
func (a *App) CookiesPath() string {
	a.mu.RLock()
	enabled := a.cfg.CookiesEnabled
	a.mu.RUnlock()
	if !enabled {
		return ""
	}
	if !ytdlp.HasCookies(a.dataDir) {
		return ""
	}
	return ytdlp.CookiesPath(a.dataDir)
}

// CookiesStatus is the read-only view of the cookies feature for the
// Settings UI. Present is whether a cookies.txt exists on disk; Enabled is
// whether the bridge currently passes --cookies to yt-dlp. The two can
// diverge: a user can upload then toggle off, or toggle on without ever
// uploading (in which case yt-dlp falls back to anonymous). Path is the
// canonical disk location regardless of state, so the UI can display it.
// PreferPremium mirrors the cfg.PreferPremiumAudio flag so the UI can show
// the toggle state; the flag has no effect at the yt-dlp call sites unless
// cookies are also present and enabled (see App.PreferPremiumAudio).
type CookiesStatus struct {
	Present       bool   `json:"present"`
	Enabled       bool   `json:"enabled"`
	Path          string `json:"path"`
	PreferPremium bool   `json:"prefer_premium"`
}

// CookiesState returns the current cookies feature state.
func (a *App) CookiesState() CookiesStatus {
	a.mu.RLock()
	enabled := a.cfg.CookiesEnabled
	preferPremium := a.cfg.PreferPremiumAudio
	a.mu.RUnlock()
	return CookiesStatus{
		Present:       ytdlp.HasCookies(a.dataDir),
		Enabled:       enabled,
		Path:          ytdlp.CookiesPath(a.dataDir),
		PreferPremium: preferPremium,
	}
}

// UploadCookies writes the given Netscape cookies.txt content to the
// canonical location atomically AND enables the feature. Rejects empty
// input and non-Netscape formats (e.g. JSON exports).
func (a *App) UploadCookies(content string) error {
	if err := ytdlp.SaveCookies(a.dataDir, content); err != nil {
		return err
	}
	a.mu.Lock()
	a.cfg.CookiesEnabled = true
	cfgCopy := a.cfg
	a.mu.Unlock()
	if err := config.Save(a.cfgPath, cfgCopy); err != nil {
		return fmt.Errorf("persist cookies-enabled flag: %w", err)
	}
	return nil
}

// RemoveCookies deletes the cookies file from disk AND disables the feature.
// Idempotent: an absent file is fine.
func (a *App) RemoveCookies() error {
	if err := ytdlp.RemoveCookies(a.dataDir); err != nil {
		return err
	}
	a.mu.Lock()
	a.cfg.CookiesEnabled = false
	cfgCopy := a.cfg
	a.mu.Unlock()
	return config.Save(a.cfgPath, cfgCopy)
}

// SetCookiesEnabled flips the bridge-side gate without touching the file
// on disk. Lets the user keep their uploaded cookies but pause use, or
// re-enable after a pause.
func (a *App) SetCookiesEnabled(enabled bool) error {
	a.mu.Lock()
	a.cfg.CookiesEnabled = enabled
	cfgCopy := a.cfg
	a.mu.Unlock()
	return config.Save(a.cfgPath, cfgCopy)
}

// SetPreferPremiumAudio persists the opt-in flag that prepends web_music to
// yt-dlp's player_client chain. The flag is harmless on its own; the gate at
// PreferPremiumAudio() ensures it has no runtime effect until cookies are
// also present and enabled.
func (a *App) SetPreferPremiumAudio(enabled bool) error {
	a.mu.Lock()
	a.cfg.PreferPremiumAudio = enabled
	cfgCopy := a.cfg
	a.mu.Unlock()
	return config.Save(a.cfgPath, cfgCopy)
}

// PreferPremiumAudio reports the effective state of the premium audio probe.
// Returns true only when the user has flipped the flag AND cookies are
// uploaded AND the cookies feature is enabled. This makes the toggle a
// no-op for users without cookies, keeping them on the fast extractor chain.
func (a *App) PreferPremiumAudio() bool {
	a.mu.RLock()
	flag := a.cfg.PreferPremiumAudio
	cookiesEnabled := a.cfg.CookiesEnabled
	a.mu.RUnlock()
	if !flag || !cookiesEnabled {
		return false
	}
	return ytdlp.HasCookies(a.dataDir)
}

// VerifyCookies runs a yt-dlp probe against a stable YouTube URL using the
// uploaded cookies file and reports whether the cookies loaded and whether
// YouTube recognised an authenticated session. Returns an error when no
// cookies file is present on disk. Does NOT take ctx as a parameter: Wails's
// JS binding generator would pass JS undefined into the slot and the
// resulting nil context would panic context.WithTimeout. Use the runtime
// context if available, else context.Background.
func (a *App) VerifyCookies() (ytdlp.VerifyResult, error) {
	if !ytdlp.HasCookies(a.dataDir) {
		return ytdlp.VerifyResult{}, errors.New("no cookies file uploaded")
	}
	a.mu.RLock()
	ytdlpPath := a.ytdlpPath
	parent := a.ctx
	a.mu.RUnlock()
	if parent == nil {
		parent = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(parent, 120*time.Second)
	defer cancel()
	return ytdlp.VerifyCookies(probeCtx, ytdlpPath, ytdlp.CookiesPath(a.dataDir))
}

// BridgeStatus returns a snapshot of the bridge's runtime state. Used by the
// frontend's initial fetch before the bridge:status event stream takes over.
// Returns the default-shaped State (server stopped, download idle) when no
// holder has been wired, so the frontend never sees empty enum strings.
func (a *App) BridgeStatus() bridgestate.State {
	a.mu.RLock()
	state := a.state
	a.mu.RUnlock()
	if state == nil {
		return bridgestate.State{Server: bridgestate.ServerStopped, Download: bridgestate.DownloadIdle}
	}
	return state.Snapshot()
}

// StartServer starts the HTTP bridge on the configured listen port and
// persists cfg.ServerEnabled=true so the choice survives a restart. Errors
// when no bridge has been wired or the underlying Start fails. Writes the
// bound port to dataDir/port.txt as a best-effort recovery hint for the
// README-documented "preferred port busy, fell back to ephemeral" path and
// for Composer's discovery hook.
func (a *App) StartServer() error {
	a.mu.RLock()
	br := a.bridge
	port := a.cfg.ListenPort
	a.mu.RUnlock()
	if br == nil {
		return errors.New("bridge not configured")
	}
	if err := br.Start(port); err != nil {
		return err
	}
	if err := server.WritePortFile(a.dataDir, br.Port()); err != nil {
		slog.Warn("write port.txt failed", "err", err)
	}
	a.persistServerEnabled(true)
	return nil
}

// StopServer stops the HTTP bridge and persists cfg.ServerEnabled=false.
// No-op when no bridge has been wired. Removes dataDir/port.txt so the
// recovery hint does not point at a port nothing is listening on.
func (a *App) StopServer() error {
	a.mu.RLock()
	br := a.bridge
	a.mu.RUnlock()
	if br == nil {
		return nil
	}
	if err := br.Stop(); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(a.dataDir, "port.txt"))
	a.persistServerEnabled(false)
	return nil
}

func (a *App) persistServerEnabled(enabled bool) {
	a.mu.Lock()
	a.cfg.ServerEnabled = enabled
	cfg := a.cfg
	path := a.cfgPath
	a.mu.Unlock()
	if err := config.Save(path, cfg); err != nil {
		slog.Warn("persist server_enabled failed", "err", err)
	}
}

// YtdlpVersion returns the cached yt-dlp version string, or "unknown".
func (a *App) YtdlpVersion() string {
	a.mu.RLock()
	fn := a.ytdlpVersion
	path := a.ytdlpPath
	a.mu.RUnlock()
	if fn != nil {
		return fn()
	}
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
	size, err := ytdlp.DownloadToFile(ctx, ytdlpPath, videoID, format, dest, a.CookiesPath(), a.PreferPremiumAudio())
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
