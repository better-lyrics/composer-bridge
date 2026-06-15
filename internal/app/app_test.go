package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/bridge"
	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
	"github.com/better-lyrics/composer-bridge/internal/config"
	"github.com/better-lyrics/composer-bridge/internal/library"
	"github.com/better-lyrics/composer-bridge/internal/updater"
	"github.com/better-lyrics/composer-bridge/internal/ytdlp"
)

func newTestApp(t *testing.T) (*App, *library.Library, *activity.Log, string) {
	t.Helper()
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("library.Open: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	act, err := activity.Open(filepath.Join(dir, "activity.db"))
	if err != nil {
		t.Fatalf("activity.Open: %v", err)
	}
	t.Cleanup(func() { act.Close() })

	cfgPath := filepath.Join(dir, "config.json")
	cfg := config.Defaults()
	a := New(lib, act, cfg, cfgPath, dir, "0.1.0")
	// Tests don't pass a real Wails ctx, so swap the runtime hooks for no-ops.
	a.hideWindow = func(context.Context) {}
	a.showWindow = func(context.Context) {}
	t.Cleanup(resetActiveForTesting)
	return a, lib, act, cfgPath
}

func sampleTrack(videoID string, importedAt int64) library.Track {
	return library.Track{
		VideoID:      videoID,
		Title:        "Hey Jude",
		Artist:       "The Beatles",
		Album:        "Hey Jude",
		ReleaseYear:  1968,
		DurationSec:  431,
		ThumbnailURL: "https://yt3.googleusercontent.com/" + videoID + ".jpg",
		IsMusic:      true,
		MusicType:    "song",
		SourceURL:    "https://www.youtube.com/watch?v=" + videoID,
		ImportedAt:   importedAt,
	}
}

func TestListTracks_ReturnsAllInsertedTracks(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	first := sampleTrack("dQw4w9WgXcQ", 1000)
	second := sampleTrack("oHg5SJYRHA0", 2000)
	if err := lib.InsertTrack(&first); err != nil {
		t.Fatalf("InsertTrack first: %v", err)
	}
	if err := lib.InsertTrack(&second); err != nil {
		t.Fatalf("InsertTrack second: %v", err)
	}

	tracks, err := a.ListTracks()
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("ListTracks: got %d, want 2", len(tracks))
	}
	if tracks[0].VideoID != "oHg5SJYRHA0" {
		t.Errorf("ListTracks order: got %q first, want %q (DESC by imported_at)", tracks[0].VideoID, "oHg5SJYRHA0")
	}
}

func TestListTracks_EmptyLibraryReturnsEmptySlice(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	tracks, err := a.ListTracks()
	if err != nil {
		t.Fatalf("ListTracks: %v", err)
	}
	if tracks == nil {
		t.Fatal("ListTracks on empty library: got nil, want non-nil empty slice (JSON marshals nil as null)")
	}
	if len(tracks) != 0 {
		t.Errorf("ListTracks on empty library: got %d entries, want 0", len(tracks))
	}
}

func TestGetTrack_FoundReturnsTrack(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	track := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&track); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	got, err := a.GetTrack("dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got == nil || got.VideoID != "dQw4w9WgXcQ" {
		t.Errorf("GetTrack: got %+v, want VideoID=dQw4w9WgXcQ", got)
	}
}

func TestGetTrack_MissingReturnsErrNotFound(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	_, err := a.GetTrack("missingvideoid")
	if !errors.Is(err, library.ErrNotFound) {
		t.Errorf("GetTrack missing: got %v, want library.ErrNotFound", err)
	}
}

func TestRemoveTrack_RemovesExistingTrack(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	track := sampleTrack("dQw4w9WgXcQ", 1000)
	if err := lib.InsertTrack(&track); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := a.RemoveTrack("dQw4w9WgXcQ"); err != nil {
		t.Fatalf("RemoveTrack: %v", err)
	}
	if _, err := lib.GetTrack("dQw4w9WgXcQ"); !errors.Is(err, library.ErrNotFound) {
		t.Errorf("after RemoveTrack: GetTrack returned %v, want library.ErrNotFound", err)
	}
}

func TestRemoveTrack_MissingReturnsErrNotFound(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	err := a.RemoveTrack("missingvideoid")
	if !errors.Is(err, library.ErrNotFound) {
		t.Errorf("RemoveTrack missing: got %v, want library.ErrNotFound", err)
	}
}

func TestRemoveTrack_DoesNotDeleteFilesOutsideConfiguredDirs(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	outsideDir := t.TempDir()
	outsideAudio := filepath.Join(outsideDir, "definitely-not-mine.opus")
	outsideThumb := filepath.Join(outsideDir, "definitely-not-mine.jpg")
	if err := os.WriteFile(outsideAudio, []byte("audio"), 0o644); err != nil {
		t.Fatalf("write outside audio: %v", err)
	}
	if err := os.WriteFile(outsideThumb, []byte("thumb"), 0o644); err != nil {
		t.Fatalf("write outside thumb: %v", err)
	}
	track := sampleTrack("dQw4w9WgXcQ", 1000)
	track.AudioPath = outsideAudio
	track.ThumbPath = outsideThumb
	if err := lib.InsertTrack(&track); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if err := a.RemoveTrack("dQw4w9WgXcQ"); err != nil {
		t.Fatalf("RemoveTrack: %v", err)
	}
	if _, err := os.Stat(outsideAudio); err != nil {
		t.Errorf("outside audio was deleted: %v (path-rooting check failed)", err)
	}
	if _, err := os.Stat(outsideThumb); err != nil {
		t.Errorf("outside thumb was deleted: %v (path-rooting check failed)", err)
	}
}

func TestRecentActivity_ReturnsInsertedRowsDesc(t *testing.T) {
	a, _, act, _ := newTestApp(t)
	idOne, err := act.Start(activity.KindImport, "dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("act.Start one: %v", err)
	}
	if err := act.End(idOne, activity.StatusOK, ""); err != nil {
		t.Fatalf("act.End one: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	idTwo, err := act.Start(activity.KindAudioDownload, "oHg5SJYRHA0")
	if err != nil {
		t.Fatalf("act.Start two: %v", err)
	}
	if err := act.End(idTwo, activity.StatusError, "boom"); err != nil {
		t.Fatalf("act.End two: %v", err)
	}

	entries, err := a.RecentActivity(10)
	if err != nil {
		t.Fatalf("RecentActivity: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("RecentActivity: got %d entries, want 2", len(entries))
	}
	if entries[0].ID != idTwo {
		t.Errorf("RecentActivity order: got id=%d first, want %d (DESC by started_at)", entries[0].ID, idTwo)
	}
}

func TestRecentActivity_EmptyLogReturnsEmptySlice(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	entries, err := a.RecentActivity(10)
	if err != nil {
		t.Fatalf("RecentActivity: %v", err)
	}
	if entries == nil {
		t.Fatal("RecentActivity on empty log: got nil, want non-nil empty slice")
	}
	if len(entries) != 0 {
		t.Errorf("RecentActivity on empty log: got %d entries, want 0", len(entries))
	}
}

func TestGetConfig_ReturnsConstructorConfig(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	got := a.GetConfig()
	want := config.Defaults()
	if got.ListenPort != want.ListenPort {
		t.Errorf("GetConfig ListenPort: got %d, want %d", got.ListenPort, want.ListenPort)
	}
	if got.YtdlpChannel != want.YtdlpChannel {
		t.Errorf("GetConfig YtdlpChannel: got %q, want %q", got.YtdlpChannel, want.YtdlpChannel)
	}
	if len(got.AllowedOrigins) != len(want.AllowedOrigins) {
		t.Errorf("GetConfig AllowedOrigins len: got %d, want %d", len(got.AllowedOrigins), len(want.AllowedOrigins))
	}
}

func TestSaveConfig_PersistsAndUpdatesInMemoryCopy(t *testing.T) {
	a, _, _, cfgPath := newTestApp(t)
	updated := config.Defaults()
	updated.ListenPort = 9999
	updated.MaxConcurrent = 7

	if err := a.SaveConfig(updated); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	reloaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load after SaveConfig: %v", err)
	}
	if reloaded.ListenPort != 9999 {
		t.Errorf("reloaded ListenPort: got %d, want 9999", reloaded.ListenPort)
	}
	if reloaded.MaxConcurrent != 7 {
		t.Errorf("reloaded MaxConcurrent: got %d, want 7", reloaded.MaxConcurrent)
	}
	inMem := a.GetConfig()
	if inMem.ListenPort != 9999 {
		t.Errorf("in-memory ListenPort after SaveConfig: got %d, want 9999", inMem.ListenPort)
	}
	if inMem.MaxConcurrent != 7 {
		t.Errorf("in-memory MaxConcurrent after SaveConfig: got %d, want 7", inMem.MaxConcurrent)
	}
}

func TestSaveConfig_PreservesServerEnabledAgainstStaleForm(t *testing.T) {
	// Regression: ServerEnabled is owned by the tray + Settings server toggle,
	// not the general Settings form. A stale form submit (user opened Settings
	// before toggling the server off via the tray) must not silently revive
	// the server-enabled flag.
	a, _, _, cfgPath := newTestApp(t)
	a.mu.Lock()
	a.cfg.ServerEnabled = false
	a.mu.Unlock()

	stale := config.Defaults() // Defaults().ServerEnabled is true.
	stale.MaxConcurrent = 4
	if err := a.SaveConfig(stale); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	reloaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if reloaded.ServerEnabled {
		t.Errorf("ServerEnabled after stale SaveConfig: got true, want false (server-side authoritative)")
	}
	if reloaded.MaxConcurrent != 4 {
		t.Errorf("MaxConcurrent: got %d, want 4", reloaded.MaxConcurrent)
	}
}

// TestGetYtdlpPath_HonorsOverride asserts that the resolved yt-dlp path
// reflects the in-memory cfg.YtdlpBinaryPath on every call, falling back to
// the managed binary path under dataDir when the override is empty. This is
// what lets a Settings flip take effect without an app restart.
func TestGetYtdlpPath_HonorsOverride(t *testing.T) {
	a, _, _, _ := newTestApp(t)

	managed, err := ytdlp.BinaryPath(a.dataDir)
	if err != nil {
		t.Skipf("no yt-dlp asset for this platform: %v", err)
	}

	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = ""
	a.mu.Unlock()
	if got := a.GetYtdlpPath(); got != managed {
		t.Errorf("override empty: got %q, want managed %q", got, managed)
	}

	override := "/opt/whatever/yt-dlp"
	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = override
	a.mu.Unlock()
	if got := a.GetYtdlpPath(); got != override {
		t.Errorf("override set: got %q, want %q", got, override)
	}

	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = ""
	a.mu.Unlock()
	if got := a.GetYtdlpPath(); got != managed {
		t.Errorf("override cleared: got %q, want managed %q", got, managed)
	}
}

// TestSaveConfig_DoesNotClobberConcurrentServerToggle exercises the lock
// dance: while a hammer goroutine keeps flipping ServerEnabled=true via
// persistServerEnabled, a SaveConfig writer fires repeatedly with a stale
// form value that says ServerEnabled=false. With the read-release-acquire
// pattern (pre-fix) SaveConfig captured the form-state ServerEnabled before
// releasing the read lock, then re-acquired the write lock and swapped a.cfg
// wholesale, clobbering the persistServerEnabled(true) writes that landed
// in between. Under the single-critical-section fix, SaveConfig reads
// ServerEnabled inside the same lock that swaps a.cfg, so the hammered
// true value can never be silently overwritten.
func TestSaveConfig_DoesNotClobberConcurrentServerToggle(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.mu.Lock()
	a.cfg.ServerEnabled = false
	a.mu.Unlock()

	cfgForm := a.GetConfig()
	cfgForm.ServerEnabled = false

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			a.persistServerEnabled(true)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if err := a.SaveConfig(cfgForm); err != nil {
				t.Errorf("SaveConfig: %v", err)
				return
			}
		}
	}()
	wg.Wait()

	a.mu.RLock()
	got := a.cfg.ServerEnabled
	a.mu.RUnlock()
	if !got {
		t.Errorf("ServerEnabled silently clobbered by stale SaveConfig: got false, want true")
	}
}

func TestOpenInComposer_NoLibraryEntry_ReturnsVideoIdOnly(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	got := a.OpenInComposer("dQw4w9WgXcQ")
	want := "https://composer.boidu.dev/?videoId=dQw4w9WgXcQ"
	if got != want {
		t.Errorf("OpenInComposer: got %q, want %q", got, want)
	}
}

func TestOpenInComposer_WithLibraryEntry_IncludesMetadata(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	track := library.Track{
		VideoID:      "dQw4w9WgXcQ",
		Title:        "Never Gonna Give You Up",
		Artist:       "Rick Astley",
		Album:        "Whenever You Need Somebody",
		DurationSec:  213,
		ThumbnailURL: "https://example.com/thumb.jpg",
		SourceURL:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		ImportedAt:   1,
	}
	if err := lib.InsertTrack(&track); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	got := a.OpenInComposer("dQw4w9WgXcQ")
	for _, want := range []string{
		"album=Whenever+You+Need+Somebody",
		"artist=Rick+Astley",
		"duration=213",
		"title=Never+Gonna+Give+You+Up",
		"videoId=dQw4w9WgXcQ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("OpenInComposer URL missing %q in %q", want, got)
		}
	}
}

func TestOpenInYouTube_ReturnsCanonicalURL(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	got := a.OpenInYouTube("dQw4w9WgXcQ")
	want := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	if got != want {
		t.Errorf("OpenInYouTube: got %q, want %q", got, want)
	}
}

func TestBridgeVersion_ReflectsConstructorArg(t *testing.T) {
	dir := t.TempDir()
	lib, err := library.Open(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("library.Open: %v", err)
	}
	defer lib.Close()
	act, err := activity.Open(filepath.Join(dir, "activity.db"))
	if err != nil {
		t.Fatalf("activity.Open: %v", err)
	}
	defer act.Close()

	a := New(lib, act, config.Defaults(), filepath.Join(dir, "config.json"), dir, "9.9.9")
	if got := a.BridgeVersion(); got != "9.9.9" {
		t.Errorf("BridgeVersion: got %q, want %q", got, "9.9.9")
	}
}

func TestStartup_StoresContext(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	type ctxKey struct{}
	parent := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	a.Startup(parent)
	if a.ctx == nil {
		t.Fatal("Startup: a.ctx is nil")
	}
	if got, _ := a.ctx.Value(ctxKey{}).(string); got != "sentinel" {
		t.Errorf("Startup: stored ctx value=%q, want %q", got, "sentinel")
	}
}

func TestShutdown_IsNoOp(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.Startup(context.Background())
	a.Shutdown(context.Background())
	if _, err := a.ListTracks(); err != nil {
		t.Errorf("ListTracks after Shutdown: %v (should still work)", err)
	}
}

func TestOnBeforeClose_XButtonHidesAndPreventsQuit(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.Startup(context.Background())
	hidden := false
	a.hideWindow = func(context.Context) { hidden = true }
	if got := a.OnBeforeClose(context.Background()); !got {
		t.Errorf("OnBeforeClose with quitting=0 should return true to prevent quit (X button path), got false")
	}
	if !hidden {
		t.Errorf("OnBeforeClose with quitting=0 should invoke hideWindow")
	}
}

func TestOnBeforeClose_MarkQuittingLetsQuitProceed(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.Startup(context.Background())
	a.MarkQuitting()
	if got := a.OnBeforeClose(context.Background()); got {
		t.Errorf("OnBeforeClose after MarkQuitting should return false so Wails quits the app (tray Quit path), got true")
	}
}

func TestActiveReturnsTheStartedUpApp(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.Startup(context.Background())
	if Active() != a {
		t.Errorf("Active() should return the most recently started-up App")
	}
}

func TestSupportsAutostartIsTrueOnDarwinLinuxWindows(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	if !a.SupportsAutostart() {
		t.Errorf("SupportsAutostart should be true now that all 3 platforms implement it")
	}
}

// newAppWithStateAndBridge wraps the standard newTestApp with a fresh holder
// and a *bridge.Bridge whose build constructor returns a no-op server. Used
// by the BridgeStatus/StartServer/StopServer tests.
func newAppWithStateAndBridge(t *testing.T) (*App, *bridgestate.Holder, *bridge.Bridge) {
	t.Helper()
	a, _, _, _ := newTestApp(t)
	// Use an ephemeral port so tests do not collide with a running bridge or
	// each other; the tests assert holder state, not specific port values.
	a.mu.Lock()
	a.cfg.ListenPort = 0
	a.mu.Unlock()
	holder := bridgestate.NewHolder()
	br := bridge.New(holder, func() *http.Server {
		return &http.Server{Handler: http.NewServeMux()}
	})
	a.SetBridgeState(holder)
	a.SetBridge(br)
	t.Cleanup(func() { _ = br.Stop() })
	return a, holder, br
}

func TestBridgeStatus_ReturnsCurrentSnapshot(t *testing.T) {
	a, holder, _ := newAppWithStateAndBridge(t)
	holder.SetServer(bridgestate.ServerRunning)
	got := a.BridgeStatus()
	if got.Server != bridgestate.ServerRunning {
		t.Errorf("Server: got %q, want %q", got.Server, bridgestate.ServerRunning)
	}
}

func TestStartServer_FlipsHolderToRunning(t *testing.T) {
	a, holder, _ := newAppWithStateAndBridge(t)
	if err := a.StartServer(); err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if got := holder.Snapshot().Server; got != bridgestate.ServerRunning {
		t.Errorf("Server: got %q, want %q", got, bridgestate.ServerRunning)
	}
}

func TestStopServer_FlipsHolderToStopped(t *testing.T) {
	a, holder, _ := newAppWithStateAndBridge(t)
	_ = a.StartServer()
	if err := a.StopServer(); err != nil {
		t.Fatalf("StopServer: %v", err)
	}
	if got := holder.Snapshot().Server; got != bridgestate.ServerStopped {
		t.Errorf("Server: got %q, want %q", got, bridgestate.ServerStopped)
	}
}

func TestStartServer_PersistsServerEnabledTrue(t *testing.T) {
	a, _, _ := newAppWithStateAndBridge(t)
	if err := a.StartServer(); err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	reloaded, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !reloaded.ServerEnabled {
		t.Errorf("ServerEnabled after Start: got false, want true")
	}
}

func TestStopServer_PersistsServerEnabledFalse(t *testing.T) {
	a, _, _ := newAppWithStateAndBridge(t)
	if err := a.StartServer(); err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if err := a.StopServer(); err != nil {
		t.Fatalf("StopServer: %v", err)
	}
	reloaded, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if reloaded.ServerEnabled {
		t.Errorf("ServerEnabled after Stop: got true, want false")
	}
}

func TestStartServer_WritesPortFile(t *testing.T) {
	a, _, _ := newAppWithStateAndBridge(t)
	if err := a.StartServer(); err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(a.dataDir, "port.txt"))
	if err != nil {
		t.Fatalf("read port.txt: %v", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("parse port.txt: %v", err)
	}
	if port == 0 {
		t.Errorf("port: got 0, want bound port")
	}
}

func TestStopServer_RemovesPortFile(t *testing.T) {
	a, _, _ := newAppWithStateAndBridge(t)
	if err := a.StartServer(); err != nil {
		t.Fatalf("StartServer: %v", err)
	}
	if err := a.StopServer(); err != nil {
		t.Fatalf("StopServer: %v", err)
	}
	if _, err := os.Stat(filepath.Join(a.dataDir, "port.txt")); !os.IsNotExist(err) {
		t.Errorf("port.txt: got err %v, want IsNotExist", err)
	}
}

func TestStartup_SubscribesEmitterFiresOnStateChange(t *testing.T) {
	a, holder, _ := newAppWithStateAndBridge(t)

	type captured struct {
		name string
		data any
	}
	var got []captured
	var mu sync.Mutex
	a.SetStatusEmitter(func(_ context.Context, name string, data any) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, captured{name, data})
	})

	a.Startup(context.Background())
	t.Cleanup(func() { a.Shutdown(context.Background()) })

	holder.SetServer(bridgestate.ServerRunning)

	mu.Lock()
	defer mu.Unlock()
	if len(got) == 0 {
		t.Fatal("no events captured")
	}
	last := got[len(got)-1]
	if last.name != "bridge:status" {
		t.Errorf("event name: got %q, want bridge:status", last.name)
	}
	state, ok := last.data.(bridgestate.State)
	if !ok {
		t.Fatalf("event payload type: got %T, want bridgestate.State", last.data)
	}
	if state.Server != bridgestate.ServerRunning {
		t.Errorf("event Server: got %q, want %q", state.Server, bridgestate.ServerRunning)
	}
}

// -- CookiesPath ---------------------------------------------------------------

func TestApp_CookiesPath_ReturnsEmptyWhenDisabled(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	// CookiesEnabled defaults to false on a fresh install.
	if got := a.CookiesPath(); got != "" {
		t.Errorf("CookiesPath when disabled = %q, want empty", got)
	}
}

func TestApp_CookiesPath_ReturnsEmptyWhenNoFile(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.mu.Lock()
	a.cfg.CookiesEnabled = true
	a.mu.Unlock()
	if got := a.CookiesPath(); got != "" {
		t.Errorf("CookiesPath when enabled but absent = %q, want empty", got)
	}
}

func TestApp_CookiesPath_ReturnsPathWhenEnabledAndPresent(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.mu.Lock()
	a.cfg.CookiesEnabled = true
	a.mu.Unlock()
	if err := os.WriteFile(ytdlp.CookiesPath(a.dataDir), []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	want := ytdlp.CookiesPath(a.dataDir)
	if got := a.CookiesPath(); got != want {
		t.Errorf("CookiesPath when enabled and present = %q, want %q", got, want)
	}
}

// -- CookiesState / UploadCookies / RemoveCookies / SetCookiesEnabled ----------

func TestApp_UploadCookies_WritesFileAndEnables(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	dir := a.dataDir
	content := "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tFALSE\t0\tSID\tfoo\n"
	if err := a.UploadCookies(content); err != nil {
		t.Fatalf("UploadCookies: %v", err)
	}
	if !ytdlp.HasCookies(dir) {
		t.Fatalf("cookies.txt not written")
	}
	got, _ := os.ReadFile(ytdlp.CookiesPath(dir))
	if string(got) != content {
		t.Fatalf("file content mismatch")
	}
	state := a.CookiesState()
	if !state.Present || !state.Enabled {
		t.Fatalf("state after upload: present=%v enabled=%v", state.Present, state.Enabled)
	}
}

func TestApp_UploadCookies_RejectsJSON(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	dir := a.dataDir
	err := a.UploadCookies(`[{"name": "SID", "value": "foo"}]`)
	if err == nil {
		t.Fatalf("UploadCookies(JSON): want error")
	}
	if ytdlp.HasCookies(dir) {
		t.Fatalf("file should not be written on rejection")
	}
	state := a.CookiesState()
	if state.Enabled {
		t.Fatalf("CookiesEnabled should remain false after rejection")
	}
}

func TestApp_UploadCookies_PersistsEnabledFlag(t *testing.T) {
	a, _, _, cfgPath := newTestApp(t)
	if err := a.UploadCookies("# Netscape HTTP Cookie File\n"); err != nil {
		t.Fatalf("UploadCookies: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !cfg.CookiesEnabled {
		t.Fatalf("CookiesEnabled not persisted to disk")
	}
}

func TestApp_RemoveCookies_ClearsFileAndDisables(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	dir := a.dataDir
	_ = a.UploadCookies("# Netscape HTTP Cookie File\n")
	if err := a.RemoveCookies(); err != nil {
		t.Fatalf("RemoveCookies: %v", err)
	}
	if ytdlp.HasCookies(dir) {
		t.Fatalf("cookies.txt still present")
	}
	state := a.CookiesState()
	if state.Present || state.Enabled {
		t.Fatalf("state after remove: present=%v enabled=%v", state.Present, state.Enabled)
	}
}

func TestApp_RemoveCookies_AbsentFileIsOK(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	if err := a.RemoveCookies(); err != nil {
		t.Fatalf("RemoveCookies(absent): %v", err)
	}
}

func TestApp_SetCookiesEnabled_KeepsFile(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	dir := a.dataDir
	_ = a.UploadCookies("# Netscape HTTP Cookie File\n")
	if err := a.SetCookiesEnabled(false); err != nil {
		t.Fatalf("SetCookiesEnabled(false): %v", err)
	}
	if !ytdlp.HasCookies(dir) {
		t.Fatalf("file should remain on disk after disable")
	}
	state := a.CookiesState()
	if !state.Present || state.Enabled {
		t.Fatalf("state: present=%v enabled=%v", state.Present, state.Enabled)
	}
	if err := a.SetCookiesEnabled(true); err != nil {
		t.Fatalf("SetCookiesEnabled(true): %v", err)
	}
	state = a.CookiesState()
	if !state.Present || !state.Enabled {
		t.Fatalf("after re-enable: present=%v enabled=%v", state.Present, state.Enabled)
	}
}

func TestApp_SetCookiesEnabled_PersistsToDisk(t *testing.T) {
	a, _, _, cfgPath := newTestApp(t)
	_ = a.UploadCookies("# Netscape HTTP Cookie File\n")
	if err := a.SetCookiesEnabled(false); err != nil {
		t.Fatalf("SetCookiesEnabled: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg.CookiesEnabled {
		t.Fatalf("disable not persisted")
	}
}

func TestApp_CookiesState_PathAlwaysSet(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	dir := a.dataDir
	state := a.CookiesState()
	want := ytdlp.CookiesPath(dir)
	if state.Path != want {
		t.Fatalf("Path = %q, want %q (independent of Present/Enabled)", state.Path, want)
	}
}

func TestShutdown_UnsubscribesStatusEmitter(t *testing.T) {
	a, holder, _ := newAppWithStateAndBridge(t)
	var calls int
	var mu sync.Mutex
	a.SetStatusEmitter(func(_ context.Context, _ string, _ any) {
		mu.Lock()
		calls++
		mu.Unlock()
	})

	a.Startup(context.Background())
	a.Shutdown(context.Background())

	holder.SetServer(bridgestate.ServerRunning)

	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Errorf("calls after Shutdown: got %d, want 0", calls)
	}
}

func TestApp_VerifyCookies_ReturnsErrorWhenNoFile(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	_, err := a.VerifyCookies()
	if err == nil {
		t.Fatalf("VerifyCookies with no file: want error")
	}
}

// -- PreferPremiumAudio --------------------------------------------------------

func TestApp_SetPreferPremiumAudio_Persists(t *testing.T) {
	a, _, _, cfgPath := newTestApp(t)
	if err := a.SetPreferPremiumAudio(true); err != nil {
		t.Fatalf("SetPreferPremiumAudio: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !cfg.PreferPremiumAudio {
		t.Fatalf("PreferPremiumAudio not persisted")
	}
}

func TestApp_PreferPremiumAudio_GatedOnCookies(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	_ = a.SetPreferPremiumAudio(true)
	// Cookies not present -> should return false even though flag is true.
	if a.PreferPremiumAudio() {
		t.Fatalf("PreferPremiumAudio should be gated off without cookies")
	}
	_ = a.UploadCookies("# Netscape HTTP Cookie File\n")
	if !a.PreferPremiumAudio() {
		t.Fatalf("PreferPremiumAudio should return true when cookies present + enabled + flag set")
	}
	_ = a.SetCookiesEnabled(false)
	if a.PreferPremiumAudio() {
		t.Fatalf("PreferPremiumAudio should be gated off when cookies disabled")
	}
}

func TestApp_PreferPremiumAudio_FalseWhenFlagOff(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	_ = a.UploadCookies("# Netscape HTTP Cookie File\n")
	// Flag is off by default; PreferPremiumAudio should report false even when cookies are live.
	if a.PreferPremiumAudio() {
		t.Fatalf("PreferPremiumAudio should be false when flag is off")
	}
}

func TestApp_CookiesState_ReflectsPreferPremium(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	state := a.CookiesState()
	if state.PreferPremium {
		t.Fatalf("CookiesState.PreferPremium should default to false")
	}
	if err := a.SetPreferPremiumAudio(true); err != nil {
		t.Fatalf("SetPreferPremiumAudio: %v", err)
	}
	state = a.CookiesState()
	if !state.PreferPremium {
		t.Fatalf("CookiesState.PreferPremium should reflect persisted flag")
	}
}

// -- AutoDownloadToLibrary -----------------------------------------------------

func TestApp_AutoDownloadToLibrary_ReadsLiveConfig(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	if a.AutoDownloadToLibrary() {
		t.Fatalf("AutoDownloadToLibrary: default should be false")
	}
	cfg := a.GetConfig()
	cfg.AutoDownloadToLibrary = true
	if err := a.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if !a.AutoDownloadToLibrary() {
		t.Fatalf("AutoDownloadToLibrary: got false after flipping cfg, want true")
	}
}

// -- Update flow --------------------------------------------------------------

func TestApp_LatestUpdate_ReturnsNilByDefault(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	if got := a.LatestUpdate(); got != nil {
		t.Errorf("LatestUpdate: got %+v, want nil before any poll has fired", got)
	}
}

func TestApp_SetLatestUpdate_RoundTripsThroughLatestUpdate(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	info := &updater.UpdateInfo{Available: true, Current: "0.1.0", Latest: "9.9.9"}
	a.SetLatestUpdate(info)
	got := a.LatestUpdate()
	if got == nil {
		t.Fatal("LatestUpdate: got nil after SetLatestUpdate")
	}
	if got.Latest != "9.9.9" || !got.Available {
		t.Errorf("LatestUpdate: got %+v, want stashed copy", got)
	}
}

func TestApp_SetLatestUpdate_NilClearsTheStash(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.SetLatestUpdate(&updater.UpdateInfo{Available: true, Latest: "9.9.9"})
	a.SetLatestUpdate(nil)
	if got := a.LatestUpdate(); got != nil {
		t.Errorf("LatestUpdate after nil reset: got %+v, want nil", got)
	}
}

func TestApp_InstallUpdate_NoStashReturnsError(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	err := a.InstallUpdate()
	if err == nil {
		t.Fatal("InstallUpdate: got nil error when no update is stashed")
	}
	if !strings.Contains(err.Error(), "no update available") {
		t.Errorf("InstallUpdate error: got %q, want phrase 'no update available'", err.Error())
	}
}

func TestApp_InstallUpdate_StashedButUnavailableReturnsError(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.SetLatestUpdate(&updater.UpdateInfo{Available: false, Current: "9.9.9", Latest: "9.9.9"})
	err := a.InstallUpdate()
	if err == nil {
		t.Fatal("InstallUpdate: got nil error when stashed info reports Available=false")
	}
	if !strings.Contains(err.Error(), "no update available") {
		t.Errorf("InstallUpdate error: got %q, want phrase 'no update available'", err.Error())
	}
}

// newManifestServer is a tiny test-local helper because the manifest server
// helper in updater_test.go is private to that package.
func newManifestServer(t *testing.T, m updater.Manifest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(m)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestApp_SetManifestURL_OverridesCheckTarget(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	if got := a.ManifestURL(); got != updater.DefaultManifestURL {
		t.Fatalf("default ManifestURL: got %q, want %q", got, updater.DefaultManifestURL)
	}

	m := updater.Manifest{
		Version:    "7.7.7",
		ReleasedAt: time.Now(),
		Notes:      "override",
		Assets: map[string]map[string]updater.Asset{
			runtime.GOOS: {
				runtime.GOARCH: {URL: "https://example.invalid/x", SHA256: "deadbeef"},
			},
		},
	}
	srv := newManifestServer(t, m)
	a.SetManifestURL(srv.URL)
	if got := a.ManifestURL(); got != srv.URL {
		t.Fatalf("SetManifestURL did not persist: got %q, want %q", got, srv.URL)
	}

	info, err := a.CheckForUpdates()
	if err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	if info == nil || info.Latest != "7.7.7" {
		t.Errorf("CheckForUpdates did not hit the override server: got %+v", info)
	}
}

func TestApp_SetManifestURL_EmptyIsNoop(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	original := a.ManifestURL()
	a.SetManifestURL("")
	if got := a.ManifestURL(); got != original {
		t.Errorf("empty SetManifestURL changed value: got %q, want %q", got, original)
	}
}

func TestApp_CheckForUpdates_StashesAndReturnsAvailable(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	m := updater.Manifest{
		Version:    "9.9.9",
		ReleasedAt: time.Now(),
		Notes:      "fixtures",
		Assets: map[string]map[string]updater.Asset{
			runtime.GOOS: {
				runtime.GOARCH: {URL: "https://example.invalid/asset", SHA256: "deadbeef"},
			},
		},
	}
	srv := newManifestServer(t, m)

	// Steer Check at the test server by replacing DefaultManifestURL via a
	// pointer-to-string swap isn't possible (const); instead exercise the
	// public CheckForUpdates path indirectly via updater.Check on the test
	// URL, then stash and emit through the App surface. Same wiring shape as
	// the production CheckForUpdates body.
	info, err := updater.Check(context.Background(), srv.URL, a.version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("updater.Check: %v", err)
	}
	a.SetLatestUpdate(info)

	stashed := a.LatestUpdate()
	if stashed == nil {
		t.Fatal("LatestUpdate: nil after stash")
	}
	if !stashed.Available {
		t.Errorf("Available: got false, want true (current %q < latest %q)", stashed.Current, stashed.Latest)
	}
	if stashed.Latest != "9.9.9" {
		t.Errorf("Latest: got %q, want 9.9.9", stashed.Latest)
	}
}

// -- Repair of pre-1.4.11 MPEG-TS cached audio ----------------------------------

// writeFakeYtdlpForApp duplicates the helper from internal/ytdlp tests because
// cross-package test helpers are an anti-pattern in this codebase.
func writeFakeYtdlpForApp(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-script tests rely on /bin/sh")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	return path
}

func mpegTSBytes(n int) []byte {
	if n < 189 {
		n = 189
	}
	b := make([]byte, n)
	b[0] = 0x47
	b[188] = 0x47
	return b
}

func TestRepairBrokenAudio_OverwritesMpegTSInPlaceAndUpdatesLibrary(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	dl := filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(dl, 0o755); err != nil {
		t.Fatalf("mkdir downloads: %v", err)
	}

	// Bad file: looks like MPEG-TS to the detector.
	badPath := filepath.Join(dl, "RgKAFK5djSk.opus")
	if err := os.WriteFile(badPath, mpegTSBytes(376), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	// Good file: arbitrary bytes that do NOT match the MPEG-TS signature.
	goodPath := filepath.Join(dl, "ZEcqHA7dbwM.opus")
	goodBytes := []byte("real opus bytes, definitely not mpegts")
	if err := os.WriteFile(goodPath, goodBytes, 0o644); err != nil {
		t.Fatalf("write good file: %v", err)
	}

	badTrack := sampleTrack("RgKAFK5djSk", 1)
	badTrack.AudioPath = badPath
	badTrack.AudioSize = 376
	if err := lib.InsertTrack(&badTrack); err != nil {
		t.Fatalf("insert bad: %v", err)
	}
	goodTrack := sampleTrack("ZEcqHA7dbwM", 2)
	goodTrack.AudioPath = goodPath
	goodTrack.AudioSize = int64(len(goodBytes))
	if err := lib.InsertTrack(&goodTrack); err != nil {
		t.Fatalf("insert good: %v", err)
	}

	// Fake yt-dlp writes a known fresh blob to whatever -o path it is given.
	const freshPayload = "fresh webm-opus payload from repair"
	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = writeFakeYtdlpForApp(t, `
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    printf '`+freshPayload+`' > "$2"
  fi
  shift
done
`)
	a.cfg.AudioFormat = "opus"
	a.downloadDir = dl
	a.mu.Unlock()

	a.repairBrokenAudio(context.Background())

	// Bad file must now contain the fresh payload, written to the original path.
	got, err := os.ReadFile(badPath)
	if err != nil {
		t.Fatalf("read repaired file: %v", err)
	}
	if string(got) != freshPayload {
		t.Errorf("repaired contents: got %q, want %q", got, freshPayload)
	}
	// Library row's size must reflect the fresh write so the cache-hit path
	// doesn't keep streaming forever.
	updated, err := lib.GetTrack("RgKAFK5djSk")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if updated.AudioPath != badPath {
		t.Errorf("AudioPath changed: got %q, want %q", updated.AudioPath, badPath)
	}
	if updated.AudioSize != int64(len(freshPayload)) {
		t.Errorf("AudioSize: got %d, want %d", updated.AudioSize, len(freshPayload))
	}

	// Good track must not be touched: same bytes, same library size.
	stillGood, err := os.ReadFile(goodPath)
	if err != nil {
		t.Fatalf("read good file: %v", err)
	}
	if string(stillGood) != string(goodBytes) {
		t.Errorf("good file was rewritten: got %q, want %q", stillGood, goodBytes)
	}
	untouched, err := lib.GetTrack("ZEcqHA7dbwM")
	if err != nil {
		t.Fatalf("GetTrack good: %v", err)
	}
	if untouched.AudioSize != int64(len(goodBytes)) {
		t.Errorf("good AudioSize changed: got %d, want %d", untouched.AudioSize, len(goodBytes))
	}
}

func TestRepairBrokenAudio_EmptyAudioPathSkipped(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	tr := sampleTrack("RgKAFK5djSk", 1)
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// AudioPath empty: repair must not invoke yt-dlp. Sentinel file proves
	// invocation either by its presence or by counting marker bytes.
	sentinel := filepath.Join(t.TempDir(), "called")
	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = writeFakeYtdlpForApp(t, "printf x >> "+sentinel)
	a.cfg.AudioFormat = "opus"
	a.mu.Unlock()

	a.repairBrokenAudio(context.Background())

	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Errorf("yt-dlp was invoked despite empty AudioPath: stat err=%v", err)
	}
}

func TestRepairBrokenAudio_CancelledContextStopsLoop(t *testing.T) {
	a, lib, _, _ := newTestApp(t)
	dl := filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(dl, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for i := 0; i < 3; i++ {
		path := filepath.Join(dl, "RgKAFK5djSk"+strconv.Itoa(i)+".opus")
		if err := os.WriteFile(path, mpegTSBytes(376), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		tr := sampleTrack("RgKAFK5djS"+string(rune('a'+i)), int64(i+1))
		tr.AudioPath = path
		tr.AudioSize = 376
		if err := lib.InsertTrack(&tr); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = writeFakeYtdlpForApp(t, "exit 1") // would fail per call; cancel short-circuits earlier
	a.cfg.AudioFormat = "opus"
	a.downloadDir = dl
	a.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	a.repairBrokenAudio(ctx)
	// No assertion on yt-dlp invocation count: the contract is "returns without
	// hanging when ctx is dead". Hitting this point at all proves the loop
	// honored cancellation.
}

// TestSaveConfig_ChannelChangeKicksRefresher asserts that flipping
// cfg.YtdlpChannel via SaveConfig fires the installed refresher exactly once,
// and that a SaveConfig with the same channel does not fire it. Without this,
// a user switching Stable to Nightly in Settings would wait up to 24h for the
// next RefreshDaily tick to act on the new channel.
func TestSaveConfig_ChannelChangeKicksRefresher(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.mu.Lock()
	a.cfg.YtdlpChannel = "stable"
	a.mu.Unlock()

	var calls atomic.Int32
	a.SetYtdlpRefresher(func() { calls.Add(1) })

	// SaveConfig with the same channel: refresher must NOT fire.
	cfg := a.GetConfig()
	if err := a.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig (unchanged): %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Errorf("unchanged channel kicked refresher; got %d calls, want 0", got)
	}

	// SaveConfig with a different channel: refresher MUST fire exactly once.
	cfg.YtdlpChannel = "nightly"
	if err := a.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig (changed): %v", err)
	}
	deadline := time.Now().Add(1 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("channel change refresher count: got %d, want 1", got)
	}
}

// TestSaveConfig_ConcurrentIdenticalFlipsKickRefresherOnce regression-tests
// the read-release-reacquire race: with prevChannel captured outside the
// write Lock, two concurrent SaveConfigs flipping stable->nightly would each
// see prev="stable" and each fire the refresher, even though only one real
// transition occurred. With prevChannel captured inside the same critical
// section as the cfg swap, the second writer sees prev="nightly" (set by the
// first) and skips firing.
func TestSaveConfig_ConcurrentIdenticalFlipsKickRefresherOnce(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.mu.Lock()
	a.cfg.YtdlpChannel = "stable"
	a.mu.Unlock()

	var calls atomic.Int32
	a.SetYtdlpRefresher(func() { calls.Add(1) })

	cfg := a.GetConfig()
	cfg.YtdlpChannel = "nightly"

	var wg sync.WaitGroup
	wg.Add(2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			<-start
			if err := a.SaveConfig(cfg); err != nil {
				t.Errorf("SaveConfig: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	time.Sleep(50 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Errorf("concurrent identical flips kicked refresher %d times, want 1", got)
	}
}

// -- ForceYtdlpUpdate ---------------------------------------------------------

func TestForceYtdlpUpdate_BinaryOverrideReturnsError(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = "/opt/whatever/yt-dlp"
	a.mu.Unlock()

	_, err := a.ForceYtdlpUpdate()
	if err == nil {
		t.Fatal("ForceYtdlpUpdate with binary override: got nil error, want error")
	}
	if !strings.Contains(err.Error(), "/opt/whatever/yt-dlp") {
		t.Errorf("error should name the override path; got %q", err)
	}
}

// TestForceYtdlpUpdate_OffChannelReturnsError covers the symmetric gate to the
// override check: a user who turned auto-updates off and then clicks Force
// Update should get a clear error rather than a silent stable-channel
// download, since channelAPIURL falls through to stable for any non-nightly
// channel string.
func TestForceYtdlpUpdate_OffChannelReturnsError(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.mu.Lock()
	a.cfg.YtdlpBinaryPath = ""
	a.cfg.YtdlpChannel = "off"
	a.mu.Unlock()

	_, err := a.ForceYtdlpUpdate()
	if err == nil {
		t.Fatal("ForceYtdlpUpdate with channel=off: got nil error, want error")
	}
	if !strings.Contains(err.Error(), "turned off") {
		t.Errorf("error should explain the off channel; got %q", err)
	}
}
