package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/config"
	"github.com/better-lyrics/composer-bridge/internal/library"
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
	a := New(lib, act, cfg, cfgPath, dir, "", "0.1.0")
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

	a := New(lib, act, config.Defaults(), filepath.Join(dir, "config.json"), dir, "", "9.9.9")
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

func TestOnBeforeCloseHidesInsteadOfQuitting(t *testing.T) {
	a, _, _, _ := newTestApp(t)
	a.Startup(context.Background())
	hidden := false
	a.hideWindow = func(ctx context.Context) { hidden = true }
	if got := a.OnBeforeClose(context.Background()); !got {
		t.Errorf("OnBeforeClose should return true (prevent quit), got false")
	}
	if !hidden {
		t.Errorf("OnBeforeClose should have invoked hideWindow")
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
