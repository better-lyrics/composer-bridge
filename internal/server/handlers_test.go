package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/activity"
	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
	"github.com/better-lyrics/composer-bridge/internal/library"
)

// -- Test helpers ---------------------------------------------------------------

// writeFakeYtdlp drops a shell script into t.TempDir() that runs `body` when
// invoked. Skips on Windows because /bin/sh is not generally present.
// Duplicated from internal/ytdlp/runner_test.go because cross-package test
// helpers are an anti-pattern.
func writeFakeYtdlp(t *testing.T, body string) string {
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

// echoFixtureScript writes a project testdata fixture into a temp file and
// returns a fake yt-dlp script that cats it to stdout.
func echoFixtureScript(t *testing.T, fixture string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return writeFakeYtdlp(t, "cat "+jsonPath)
}

type testEnv struct {
	t        *testing.T
	handlers *Handlers
	server   *httptest.Server
	lib      *library.Library
	act      *activity.Log
	thumbDir string
}

func newTestEnv(t *testing.T, ytdlpPath string) *testEnv {
	t.Helper()
	dir := t.TempDir()
	libPath := filepath.Join(dir, "library.db")
	lib, err := library.Open(libPath)
	if err != nil {
		t.Fatalf("library.Open: %v", err)
	}
	t.Cleanup(func() { lib.Close() })

	actPath := filepath.Join(dir, "activity.db")
	act, err := activity.Open(actPath)
	if err != nil {
		t.Fatalf("activity.Open: %v", err)
	}
	t.Cleanup(func() { act.Close() })

	thumbDir := filepath.Join(dir, "thumbs")
	h := &Handlers{
		Library:   lib,
		Activity:  act,
		YtdlpPath: func() string { return ytdlpPath },
		ThumbDir:  thumbDir,
		Bridge:    "0.1.0",
	}
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	return &testEnv{t: t, handlers: h, server: srv, lib: lib, act: act, thumbDir: thumbDir}
}

func (e *testEnv) lastActivity() activity.Entry {
	e.t.Helper()
	entries, err := e.act.Recent(1)
	if err != nil {
		e.t.Fatalf("activity.Recent: %v", err)
	}
	if len(entries) == 0 {
		e.t.Fatal("activity.Recent: no entries")
	}
	return entries[0]
}

func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 100), G: uint8(y * 100), B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func decodeJSONError(t *testing.T, body io.Reader) string {
	t.Helper()
	var got map[string]string
	if err := json.NewDecoder(body).Decode(&got); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return got["error"]
}

// -- Health ---------------------------------------------------------------------

func TestHealth_ReturnsLockedJSONShape(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `echo "2025.06.30"`))

	resp, err := http.Get(env.server.URL + "/health")
	if err != nil {
		t.Fatalf("Get /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type: got %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["bridge"] != "0.1.0" {
		t.Errorf("bridge: got %q, want 0.1.0", got["bridge"])
	}
	if got["ytdlp"] != "2025.06.30" {
		t.Errorf("ytdlp: got %q, want 2025.06.30", got["ytdlp"])
	}
	if got["status"] != "ok" {
		t.Errorf("status: got %q, want ok", got["status"])
	}
	wantKeys := map[string]struct{}{"bridge": {}, "ytdlp": {}, "status": {}}
	for k := range got {
		if _, ok := wantKeys[k]; !ok {
			t.Errorf("unexpected key %q in health response (BridgeHealth interface locked)", k)
		}
	}
}

func TestHealth_OPTIONSReturns204WithCORS(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `echo "2025.06.30"`))
	handler := WithCORS(env.handlers.Router(), staticOrigins(corsAllowed))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/health", nil)
	req.Header.Set("Origin", "https://composer.boidu.dev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://composer.boidu.dev" {
		t.Errorf("allow-origin: got %q, want echoed", got)
	}
}

// -- Audio ----------------------------------------------------------------------

func TestAudio_StreamsBytesWithLockedHeaders(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'hello world'`))

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "audio/webm" {
		t.Errorf("content-type: got %q, want audio/webm", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("cache-control: got %q, want no-store", got)
	}
	if got := resp.Header.Get("X-Bridge-Version"); got != env.handlers.Bridge {
		t.Errorf("x-bridge-version: got %q, want %q", got, env.handlers.Bridge)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Errorf("body: got %q, want hello world", body)
	}

	entry := env.lastActivity()
	if entry.Kind != activity.KindAudioDownload {
		t.Errorf("activity kind: got %q, want %q", entry.Kind, activity.KindAudioDownload)
	}
	if entry.Status != activity.StatusOK {
		t.Errorf("activity status: got %q, want ok", entry.Status)
	}
	if entry.VideoID != "RgKAFK5djSk" {
		t.Errorf("activity videoID: got %q, want RgKAFK5djSk", entry.VideoID)
	}
}

func TestAudio_InvalidVideoIDReturns400WithoutForking(t *testing.T) {
	env := newTestEnv(t, "/nonexistent/binary")

	// "tooshort" is < 11 chars and fails VideoIDRe before forking.
	resp, err := http.Get(env.server.URL + "/audio/tooshort")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	if msg := decodeJSONError(t, resp.Body); msg != "invalid video id" {
		t.Errorf("error: got %q, want invalid video id", msg)
	}
	entries, _ := env.act.Recent(1)
	if len(entries) != 0 {
		t.Errorf("activity should not log invalid IDs: got %d entries", len(entries))
	}
}

func TestAudio_YtdlpFailureBeforeBytesReturns502(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `echo "boom" >&2 && exit 1`))

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502 (composer-bridge-api differentiates 502 vs 500)", resp.StatusCode)
	}
	msg := decodeJSONError(t, resp.Body)
	if !strings.Contains(msg, "RgKAFK5djSk") {
		t.Errorf("error msg should include videoID: got %q", msg)
	}

	entry := env.lastActivity()
	if entry.Status != activity.StatusError {
		t.Errorf("activity status: got %q, want error", entry.Status)
	}
	if !strings.Contains(entry.Message, "RgKAFK5djSk") {
		t.Errorf("activity message: got %q, want contains videoID", entry.Message)
	}
}

func TestAudio_ArgvRegressionFlags(t *testing.T) {
	// Print argv to stderr, then exit 1 so the handler observes failure and
	// records argv in the activity log message.
	env := newTestEnv(t, writeFakeYtdlp(t, `for a in "$@"; do echo "$a" >&2; done; exit 1`))

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()

	entry := env.lastActivity()
	wantSubstrs := []string{
		"-f", "bestaudio[acodec=opus][ext=webm]/bestaudio[ext=webm]/bestaudio[protocol!*=m3u8]/best[protocol!*=m3u8]/bestaudio/best",
		"-o", "--quiet", "--no-warnings", "--no-playlist",
		"--extractor-args", "youtube:player_client=android_vr,web_safari;player_skip=configs,initial_data",
		"https://www.youtube.com/watch?v=RgKAFK5djSk",
	}
	for _, s := range wantSubstrs {
		if !strings.Contains(entry.Message, s) {
			t.Errorf("argv missing %q in activity message: %q", s, entry.Message)
		}
	}
}

// -- Import ---------------------------------------------------------------------

func TestImport_HappyPathInsertsTrackAndLogs(t *testing.T) {
	env := newTestEnv(t, echoFixtureScript(t, "music_frank_sinatra.json"))

	body := strings.NewReader(`{"video_id":"ZEcqHA7dbwM"}`)
	resp, err := http.Post(env.server.URL+"/import", "application/json", body)
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	var got library.Track
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.VideoID != "ZEcqHA7dbwM" {
		t.Errorf("videoID: got %q", got.VideoID)
	}
	if !got.IsMusic {
		t.Error("IsMusic: got false, want true (music fixture)")
	}
	if !strings.Contains(got.ThumbnailURL, "=w1024-h1024") {
		t.Errorf("thumbnail URL not rewritten to size=1024: got %q", got.ThumbnailURL)
	}

	stored, err := env.lib.GetTrack("ZEcqHA7dbwM")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if stored.Title == "" {
		t.Error("stored.Title: got empty")
	}

	entry := env.lastActivity()
	if entry.Kind != activity.KindImport {
		t.Errorf("activity kind: got %q, want import", entry.Kind)
	}
	if entry.Status != activity.StatusOK {
		t.Errorf("activity status: got %q, want ok", entry.Status)
	}
}

func TestImport_InvalidJSONReturns400(t *testing.T) {
	env := newTestEnv(t, "/nonexistent/binary")

	resp, err := http.Post(env.server.URL+"/import", "application/json", strings.NewReader("not json"))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	entries, _ := env.act.Recent(1)
	if len(entries) != 0 {
		t.Errorf("activity should not log malformed bodies: got %d entries", len(entries))
	}
}

func TestImport_MissingVideoIDReturns400(t *testing.T) {
	env := newTestEnv(t, "/nonexistent/binary")

	resp, err := http.Post(env.server.URL+"/import", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	if msg := decodeJSONError(t, resp.Body); msg != "invalid video id" {
		t.Errorf("error: got %q, want invalid video id", msg)
	}
}

func TestImport_InvalidVideoIDReturns400(t *testing.T) {
	env := newTestEnv(t, "/nonexistent/binary")

	resp, err := http.Post(env.server.URL+"/import", "application/json", strings.NewReader(`{"video_id":"bad"}`))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
}

func TestImport_YtdlpFailureReturns502WithActivityError(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `echo "boom" >&2 && exit 1`))

	resp, err := http.Post(env.server.URL+"/import", "application/json", strings.NewReader(`{"video_id":"ZEcqHA7dbwM"}`))
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502", resp.StatusCode)
	}
	entry := env.lastActivity()
	if entry.Status != activity.StatusError {
		t.Errorf("activity status: got %q, want error", entry.Status)
	}
	if entry.Kind != activity.KindImport {
		t.Errorf("activity kind: got %q, want import", entry.Kind)
	}
}

// -- Thumb ----------------------------------------------------------------------

func seedTrack(t *testing.T, lib *library.Library, tr library.Track) {
	t.Helper()
	if err := lib.InsertTrack(&tr); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
}

func TestThumb_RejectsOutOfRootPathAndDoesNotServeFile(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")
	jpegBytes := tinyJPEG(t)
	outside := filepath.Join(t.TempDir(), "outside.jpg")
	if err := os.WriteFile(outside, jpegBytes, 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: "http://example.invalid/x.jpg", ThumbPath: outside,
		SourceURL: "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
	})
	resp, err := http.Get(env.server.URL + "/thumb/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if bytes.Equal(body, jpegBytes) {
			t.Error("path-rooting check failed: served file outside ThumbDir")
		}
	}
}

func TestThumb_ServesCachedFile(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")
	jpegBytes := tinyJPEG(t)
	if err := os.MkdirAll(env.thumbDir, 0o755); err != nil {
		t.Fatalf("mkdir thumbDir: %v", err)
	}
	cached := filepath.Join(env.thumbDir, "RgKAFK5djSk.jpg")
	if err := os.WriteFile(cached, jpegBytes, 0o644); err != nil {
		t.Fatalf("write cached: %v", err)
	}
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: "http://example.invalid/x.jpg", ThumbPath: cached,
		SourceURL: "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
	})

	resp, err := http.Get(env.server.URL + "/thumb/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("cache-control: got %q", got)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, jpegBytes) {
		t.Errorf("body bytes mismatch: got %d, want %d", len(body), len(jpegBytes))
	}
}

func TestThumb_FetchesAndCachesOnMiss(t *testing.T) {
	jpegBytes := tinyJPEG(t)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpegBytes)
	}))
	defer origin.Close()

	env := newTestEnv(t, "/nonexistent")
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: origin.URL + "/art.jpg", ThumbPath: "",
		SourceURL: "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
	})

	resp, err := http.Get(env.server.URL + "/thumb/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, jpegBytes) {
		t.Errorf("body bytes mismatch: got %d, want %d", len(body), len(jpegBytes))
	}

	stored, err := env.lib.GetTrack("RgKAFK5djSk")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if stored.ThumbPath == "" {
		t.Error("ThumbPath was not persisted after fetch")
	}
	if _, err := os.Stat(stored.ThumbPath); err != nil {
		t.Errorf("cached file missing on disk: %v", err)
	}
	if !strings.HasPrefix(stored.ThumbPath, env.thumbDir) {
		t.Errorf("ThumbPath %q not under thumbDir %q", stored.ThumbPath, env.thumbDir)
	}
}

func TestThumb_NotFoundReturns404(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")

	resp, err := http.Get(env.server.URL + "/thumb/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

func TestThumb_OriginFailureReturns502(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer origin.Close()

	env := newTestEnv(t, "/nonexistent")
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: origin.URL + "/art.jpg", ThumbPath: "",
		SourceURL: "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
	})

	resp, err := http.Get(env.server.URL + "/thumb/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status: got %d, want 502", resp.StatusCode)
	}
}

func TestThumb_AtomicWriteLeavesNoPartialOnUpstreamMidStreamDisconnect(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial-bytes"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, err := hj.Hijack()
			if err == nil {
				conn.Close()
			}
		}
	}))
	defer origin.Close()

	env := newTestEnv(t, "/nonexistent")
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: origin.URL + "/art.jpg", ThumbPath: "",
		SourceURL: "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
	})

	resp, err := http.Get(env.server.URL + "/thumb/RgKAFK5djSk")
	if err == nil {
		resp.Body.Close()
	}

	// The atomic write must leave NO <videoID>.jpg behind because io.Copy
	// errored before the rename was attempted.
	finalPath := filepath.Join(env.thumbDir, "RgKAFK5djSk.jpg")
	if _, statErr := os.Stat(finalPath); !os.IsNotExist(statErr) {
		t.Errorf("final thumb path %q exists after mid-stream disconnect; atomic write was not honored (statErr=%v)", finalPath, statErr)
	}

	// No .tmp leftovers should remain either (defer os.Remove handles it).
	entries, err := os.ReadDir(env.thumbDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir thumbDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file %q leaked in thumb dir after mid-stream disconnect", e.Name())
		}
	}

	stored, err := env.lib.GetTrack("RgKAFK5djSk")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if stored.ThumbPath != "" {
		t.Errorf("ThumbPath should remain empty when fetch failed; got %q", stored.ThumbPath)
	}
}

func TestThumb_CacheControlHeaderAlwaysSet(t *testing.T) {
	jpegBytes := tinyJPEG(t)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(jpegBytes)
	}))
	defer origin.Close()

	env := newTestEnv(t, "/nonexistent")
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: origin.URL + "/art.jpg",
		SourceURL:    "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
	})

	resp, err := http.Get(env.server.URL + "/thumb/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("cache-control: got %q", got)
	}
}

func TestAudio_XBridgeVersionUsesStructFieldNotConstant(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'hello world'`))
	env.handlers.Bridge = "9.9.9"

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Bridge-Version"); got != "9.9.9" {
		t.Errorf("x-bridge-version: got %q, want %q (must come from h.Bridge, not ytdlp.BridgeVersion)", got, "9.9.9")
	}
}

func TestAudio_YtdlpFailsAfterFirstByteClosesWithoutJSONError(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'partial-audio-bytes'; exit 1`))

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200 (already committed when bytes flowed)", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "partial-audio-bytes" {
		t.Errorf("body: got %q, want %q (no JSON error suffix may be appended; would corrupt stream)", string(body), "partial-audio-bytes")
	}

	entry := env.lastActivity()
	if entry.Status != activity.StatusError {
		t.Errorf("activity status: got %q, want error", entry.Status)
	}
	if !strings.Contains(entry.Message, "RgKAFK5djSk") {
		t.Errorf("activity message: got %q, want contains videoID", entry.Message)
	}
}

// -- Audio cache-first ---------------------------------------------------------

// seedDownloadedTrack writes audioBytes to a file under env's per-test download
// directory (also wired into env.handlers.DownloadDir), inserts a matching
// library row whose AudioPath points at that file, and returns the absolute
// path on disk. ext drives both the filename suffix and the on-disk extension
// the cache-hit branch infers Content-Type from. Helper exists because every
// cache-hit test wants the same three-step setup; spelling each out inline
// makes the tests look like they're testing the setup, not the behavior.
func seedDownloadedTrack(t *testing.T, env *testEnv, videoID, ext string, audioBytes []byte) string {
	t.Helper()
	dlDir := filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(dlDir, 0o755); err != nil {
		t.Fatalf("mkdir downloads: %v", err)
	}
	env.handlers.DownloadDir = func() string { return dlDir }
	dest := filepath.Join(dlDir, videoID+"."+ext)
	if err := os.WriteFile(dest, audioBytes, 0o644); err != nil {
		t.Fatalf("write cached audio: %v", err)
	}
	seedTrack(t, env.lib, library.Track{
		VideoID: videoID, Title: "Title", Artist: "Artist", Album: "Album",
		DurationSec: 10, ThumbnailURL: "http://example.invalid/x.jpg",
		SourceURL: "https://www.youtube.com/watch?v=" + videoID, ImportedAt: 1,
		AudioPath: dest, AudioSize: int64(len(audioBytes)),
	})
	return dest
}

func TestAudio_ServesCachedFileWhenAudioPathExists(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'streamed-not-cached'`))
	audioBytes := []byte("cached audio payload")
	seedDownloadedTrack(t, env, "RgKAFK5djSk", "opus", audioBytes)

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "audio/webm" {
		t.Errorf("content-type: got %q, want audio/webm (opus on disk)", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("cache-control: got %q, want no-store", got)
	}
	if got := resp.Header.Get("X-Bridge-Version"); got != env.handlers.Bridge {
		t.Errorf("x-bridge-version: got %q, want %q", got, env.handlers.Bridge)
	}
	if got := resp.Header.Get("X-Track-Title"); got != "Title" {
		t.Errorf("x-track-title: got %q, want %q", got, "Title")
	}
	if got := resp.Header.Get("X-Track-Artist"); got != "Artist" {
		t.Errorf("x-track-artist: got %q, want %q", got, "Artist")
	}
	if got := resp.Header.Get("X-Track-Album"); got != "Album" {
		t.Errorf("x-track-album: got %q, want %q", got, "Album")
	}
	if got := resp.Header.Get("Access-Control-Expose-Headers"); !strings.Contains(got, "X-Track-Title") {
		t.Errorf("access-control-expose-headers: got %q, want list with X-Track-Title", got)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, audioBytes) {
		t.Errorf("body bytes mismatch: got %d bytes, want %d (cache hit must return on-disk payload, not fake yt-dlp output)", len(body), len(audioBytes))
	}

	// Cache hits must not log to the activity feed; the streaming path is the
	// only thing worth surfacing in the bridge UI.
	entries, err := env.act.Recent(1)
	if err != nil {
		t.Fatalf("activity.Recent: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("cache hit unexpectedly wrote activity entry: %+v", entries)
	}
}

func TestAudio_FallsThroughToStreamWhenFileMissing(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'stream payload'`))
	dest := seedDownloadedTrack(t, env, "RgKAFK5djSk", "opus", []byte("ignored"))
	if err := os.Remove(dest); err != nil {
		t.Fatalf("delete cached audio: %v", err)
	}

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "stream payload" {
		t.Errorf("body: got %q, want stream payload (must fall through to yt-dlp when file is gone)", body)
	}
	entry := env.lastActivity()
	if entry.Status != activity.StatusOK || entry.Kind != activity.KindAudioDownload {
		t.Errorf("activity: got %+v, want kind=audio_download status=ok (streaming path must log)", entry)
	}
}

// Regression: yt-dlp's old opus selector matched YouTube's HLS-Opus stream and
// saved MPEG-TS bytes into a .opus file. The browser can't play that, so cached
// rows in long-running installs end up broken. The cache-hit path must detect
// the MPEG-TS magic and fall through to streaming so the user gets working
// audio on the same play. The file itself is intentionally left in place: the
// background repair on startup is what overwrites it with valid bytes; the
// serve path must not delete user library files behind their back.
func TestAudio_BypassesMpegTSCacheAndStreamsLeavingFileIntact(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'fresh stream'`))
	// Synthesise a minimal MPEG-TS file: two sync packets (188 bytes each) so
	// the detector's "0x47 at 0 and 188" check fires positive.
	bad := make([]byte, 376)
	bad[0] = 0x47
	bad[188] = 0x47
	dest := seedDownloadedTrack(t, env, "RgKAFK5djSk", "opus", bad)

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "fresh stream" {
		t.Errorf("body: got %q, want fresh stream (must serve via yt-dlp when cache is unplayable)", body)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("bad cache file unexpectedly missing after bypass: %v", err)
	}
	if info.Size() != int64(len(bad)) {
		t.Errorf("bad file size changed: got %d, want %d (handler must not rewrite the file)", info.Size(), len(bad))
	}
	got, err := env.lib.GetTrack("RgKAFK5djSk")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.AudioPath != dest {
		t.Errorf("AudioPath: got %q, want %q (library row must not be mutated by the serve path)", got.AudioPath, dest)
	}
}

func TestAudio_FallsThroughWhenAudioPathOutsideDownloadDir(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'stream payload'`))
	dlDir := filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(dlDir, 0o755); err != nil {
		t.Fatalf("mkdir downloads: %v", err)
	}
	env.handlers.DownloadDir = func() string { return dlDir }

	outsideDir := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("mkdir elsewhere: %v", err)
	}
	outside := filepath.Join(outsideDir, "RgKAFK5djSk.opus")
	if err := os.WriteFile(outside, []byte("should not be served"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: "http://example.invalid/x.jpg",
		SourceURL:    "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
		AudioPath: outside, AudioSize: 20,
	})

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "stream payload" {
		t.Errorf("body: got %q, want stream payload (DB row pointing outside DownloadDir must NOT be served)", body)
	}
}

func TestAudio_FallsThroughWhenDownloadDirCallbackNil(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'stream payload'`))
	// Seed a track with AudioPath set, but never wire DownloadDir.
	dest := filepath.Join(t.TempDir(), "RgKAFK5djSk.opus")
	if err := os.WriteFile(dest, []byte("cached"), 0o644); err != nil {
		t.Fatalf("write cached: %v", err)
	}
	seedTrack(t, env.lib, library.Track{
		VideoID: "RgKAFK5djSk", Title: "x", DurationSec: 10,
		ThumbnailURL: "http://example.invalid/x.jpg",
		SourceURL:    "https://www.youtube.com/watch?v=RgKAFK5djSk", ImportedAt: 1,
		AudioPath: dest, AudioSize: 6,
	})

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get /audio: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "stream payload" {
		t.Errorf("body: got %q, want stream payload (nil DownloadDir must disable cache-first)", body)
	}
}

func TestAudio_CacheHitContentTypeFromExtension(t *testing.T) {
	// AudioFormat on the handler is intentionally a different format than what's
	// on disk so a regression that reads h.AudioFormat instead of the extension
	// shows up immediately.
	cases := []struct {
		ext         string
		wantContent string
	}{
		{"opus", "audio/webm"},
		{"webm", "audio/webm"},
		{"m4a", "audio/mp4"},
		{"mp3", "audio/mpeg"},
	}
	for _, tc := range cases {
		t.Run(tc.ext, func(t *testing.T) {
			env := newTestEnv(t, "/nonexistent")
			env.handlers.AudioFormat = "mp3" // intentionally wrong vs. on-disk
			seedDownloadedTrack(t, env, "RgKAFK5djSk", tc.ext, []byte("cached"))

			resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			defer resp.Body.Close()
			if got := resp.Header.Get("Content-Type"); got != tc.wantContent {
				t.Errorf(".%s on disk: content-type got %q, want %q", tc.ext, got, tc.wantContent)
			}
		})
	}
}

func TestAudio_CacheHitSupportsRangeRequest(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")
	audioBytes := []byte("0123456789abcdef")
	seedDownloadedTrack(t, env, "RgKAFK5djSk", "opus", audioBytes)

	req, _ := http.NewRequest(http.MethodGet, env.server.URL+"/audio/RgKAFK5djSk", nil)
	req.Header.Set("Range", "bytes=2-5")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Errorf("status: got %d, want 206 (http.ServeFile must honor Range)", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "2345" {
		t.Errorf("partial body: got %q, want %q", body, "2345")
	}
}

func TestAudio_CacheHitDoesNotFlipDownloadState(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")
	holder := bridgestate.NewHolder()
	env.handlers.State = holder

	var sawActive bool
	var mu sync.Mutex
	t.Cleanup(holder.OnChange(func(s bridgestate.State) {
		mu.Lock()
		defer mu.Unlock()
		if s.Download == bridgestate.DownloadActive {
			sawActive = true
		}
	}))

	seedDownloadedTrack(t, env, "RgKAFK5djSk", "opus", []byte("cached"))

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	mu.Lock()
	defer mu.Unlock()
	if sawActive {
		t.Error("cache hit flipped Download to Active; tray would show false 'downloading' status")
	}
	if got := holder.Snapshot().Download; got != bridgestate.DownloadIdle {
		t.Errorf("final Download: got %q, want %q", got, bridgestate.DownloadIdle)
	}
}

// -- Emitter integration -------------------------------------------------------

type recordedEmission struct {
	name string
	args []any
}

type recordingEmitter struct {
	mu       sync.Mutex
	captured []recordedEmission
}

func (r *recordingEmitter) Emit(_ context.Context, name string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.captured = append(r.captured, recordedEmission{name: name, args: args})
}

func (r *recordingEmitter) snapshot() []recordedEmission {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedEmission, len(r.captured))
	copy(out, r.captured)
	return out
}

func TestAudio_EmitterPublishesStartAndEnd(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'hello world'`))
	rec := &recordingEmitter{}
	env.handlers.Emitter = rec
	env.handlers.EmitterCtx = context.Background()
	srv := httptest.NewServer(env.handlers.Router())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	got := rec.snapshot()
	if len(got) != 2 {
		t.Fatalf("emissions: got %d, want 2 (start + ok)", len(got))
	}
	for i, em := range got {
		if em.name != "activity:update" {
			t.Errorf("emission[%d].name: got %q, want activity:update", i, em.name)
		}
		if len(em.args) != 1 {
			t.Fatalf("emission[%d].args: got %d, want 1", i, len(em.args))
		}
		entry, ok := em.args[0].(activity.Entry)
		if !ok {
			t.Fatalf("emission[%d].args[0]: got %T, want activity.Entry", i, em.args[0])
		}
		if entry.VideoID != "RgKAFK5djSk" {
			t.Errorf("emission[%d].videoID: got %q, want RgKAFK5djSk", i, entry.VideoID)
		}
		if entry.Kind != activity.KindAudioDownload {
			t.Errorf("emission[%d].kind: got %q, want %q", i, entry.Kind, activity.KindAudioDownload)
		}
	}
	if start, _ := got[0].args[0].(activity.Entry); start.Status != activity.StatusRunning {
		t.Errorf("first emission status: got %q, want running", start.Status)
	}
	if end, _ := got[1].args[0].(activity.Entry); end.Status != activity.StatusOK {
		t.Errorf("second emission status: got %q, want ok", end.Status)
	}
}

// -- Cookies path callback -----------------------------------------------------

func TestAudio_PassesCookiesPathFromCallback(t *testing.T) {
	cookies := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	env := newTestEnv(t, writeFakeYtdlp(t, `for a in "$@"; do echo "$a" >&2; done; exit 1`))
	env.handlers.CookiesPath = func() string { return cookies }
	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()
	entry := env.lastActivity()
	if !strings.Contains(entry.Message, "--cookies") || !strings.Contains(entry.Message, cookies) {
		t.Errorf("activity message missing cookies args: %q", entry.Message)
	}
}

func TestAudio_NoCookiesPath_NoFlag(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `for a in "$@"; do echo "$a" >&2; done; exit 1`))
	// CookiesPath is nil, same as no callback.
	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()
	entry := env.lastActivity()
	if strings.Contains(entry.Message, "--cookies") {
		t.Errorf("activity message unexpectedly contains --cookies: %q", entry.Message)
	}
}

func TestAudio_NilCookiesCallbackReturnsEmpty(t *testing.T) {
	h := &Handlers{}
	if got := h.cookiesPath(); got != "" {
		t.Errorf("nil CookiesPath returned %q, want empty string", got)
	}
}

// -- Sanity: combined Router under CORS still routes everything ----------------

func TestRouter_UnderCORSWrappersStillRoutes(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `echo "2025.06.30"`))
	wrapped := WithCORS(env.handlers.Router(), staticOrigins(corsAllowed))
	srv := httptest.NewServer(wrapped)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allow-origin: got %q, want echoed", got)
	}
}

// -- Bridgestate wiring --------------------------------------------------------

func TestAudio_FlipsHolderActiveWhileStreamingAndIdleWhenDone(t *testing.T) {
	// Slow yt-dlp: write a byte, sleep, write another byte so we can observe
	// the holder mid-flight via OnChange transitions.
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'a'; sleep 0.05; printf 'b'`))
	holder := bridgestate.NewHolder()
	env.handlers.State = holder

	var seen []bridgestate.DownloadStatus
	var seenIDs []string
	var mu sync.Mutex
	t.Cleanup(holder.OnChange(func(s bridgestate.State) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, s.Download)
		seenIDs = append(seenIDs, s.DownloadVideoID)
	}))

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	mu.Lock()
	defer mu.Unlock()
	// We expect at least one active transition with the videoID, followed by
	// an idle transition. Other intermediate broadcasts may exist; we only
	// care that the sequence is observable.
	var sawActive, sawIdleAfterActive bool
	for i, s := range seen {
		if s == bridgestate.DownloadActive && seenIDs[i] == "RgKAFK5djSk" {
			sawActive = true
		}
		if sawActive && s == bridgestate.DownloadIdle {
			sawIdleAfterActive = true
		}
	}
	if !sawActive || !sawIdleAfterActive {
		t.Errorf("download transitions: got %v (ids %v), want Active-with-id then Idle", seen, seenIDs)
	}
	if got := holder.Snapshot().Download; got != bridgestate.DownloadIdle {
		t.Errorf("final Download: got %q, want %q", got, bridgestate.DownloadIdle)
	}
	if got := holder.Snapshot().DownloadVideoID; got != "" {
		t.Errorf("final DownloadVideoID: got %q, want empty", got)
	}
}

func TestAudio_FailedDownloadRecordsLastErrorOnHolder(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `echo "boom" >&2 && exit 1`))
	holder := bridgestate.NewHolder()
	env.handlers.State = holder

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	resp.Body.Close()

	snap := holder.Snapshot()
	if snap.Download != bridgestate.DownloadIdle {
		t.Errorf("Download: got %q, want %q", snap.Download, bridgestate.DownloadIdle)
	}
	if snap.LastError == "" {
		t.Error("LastError: empty, want non-empty after failed download")
	}
}

func TestAudio_NilHolderIsNoop(t *testing.T) {
	// Regression: existing handlers tests construct Handlers without State.
	// The handler must not panic when State is nil.
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'ok'`))
	// State left nil intentionally.

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

// -- Cache-hit hang regression -------------------------------------------------

// Regression: production reports of GET /audio/{id} hanging indefinitely on
// cache-hit (track row + audio file both present). The hang manifests with no
// activity row written, so the wedge is somewhere in Audio -> resolveTrack ->
// serveCachedAudio before startActivity ever fires. The cache-hit path is
// pure local I/O (SQLite read with WAL/busy_timeout, file stat, ServeFile),
// so any handler-side response should land well under a second. Anything
// taking more than a few seconds is the bug.
func TestAudio_CacheHit_ConcurrentRequestsRespondUnderDeadline(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")
	audioBytes := []byte("cached audio payload")
	seedDownloadedTrack(t, env, "RgKAFK5djSk", "opus", audioBytes)

	const workers = 16
	const deadline = 5 * time.Second

	type result struct {
		status int
		bodyOK bool
		took   time.Duration
		err    error
	}
	results := make(chan result, workers)

	client := &http.Client{Timeout: deadline}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			t0 := time.Now()
			resp, err := client.Get(env.server.URL + "/audio/RgKAFK5djSk")
			if err != nil {
				results <- result{err: err, took: time.Since(t0)}
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			results <- result{
				status: resp.StatusCode,
				bodyOK: bytes.Equal(body, audioBytes),
				took:   time.Since(t0),
			}
		}()
	}

	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(deadline + time.Second):
		t.Fatalf("cache-hit handler wedged: %d concurrent requests did not finish within %s", workers, deadline+time.Second)
	}

	close(results)
	var seen int
	for r := range results {
		seen++
		if r.err != nil {
			t.Errorf("request %d failed after %s: %v", seen, r.took, r.err)
			continue
		}
		if r.status != http.StatusOK {
			t.Errorf("request %d status: got %d, want 200 (took %s)", seen, r.status, r.took)
		}
		if !r.bodyOK {
			t.Errorf("request %d body mismatch (took %s)", seen, r.took)
		}
		if r.took >= deadline {
			t.Errorf("request %d took %s, want < %s (cache-hit should be sub-second)", seen, r.took, deadline)
		}
	}
	if seen != workers {
		t.Fatalf("got %d results, want %d", seen, workers)
	}
}

// -- Audio auto-download to library --------------------------------------------

// seedTrackForAutoDL writes a library row for videoID with the given title and
// no AudioPath set, configures env.handlers.DownloadDir + AudioFormat + the
// AutoDownload callback, and returns the configured download directory. Helper
// exists because every auto-download test wants the same three-step setup.
func seedTrackForAutoDL(t *testing.T, env *testEnv, videoID, title, format string, enabled bool) string {
	t.Helper()
	dlDir := filepath.Join(t.TempDir(), "downloads")
	env.handlers.DownloadDir = func() string { return dlDir }
	env.handlers.AutoDownload = func() bool { return enabled }
	env.handlers.AudioFormat = format
	seedTrack(t, env.lib, library.Track{
		VideoID: videoID, Title: title, DurationSec: 10,
		ThumbnailURL: "http://example.invalid/x.jpg",
		SourceURL:    "https://www.youtube.com/watch?v=" + videoID,
		ImportedAt:   1,
	})
	return dlDir
}

func TestAudio_AutoDownload_TeeStreamWritesFileAndMarksLibrary(t *testing.T) {
	const payload = "tee-stream payload bytes"
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'tee-stream payload bytes'`))
	dlDir := seedTrackForAutoDL(t, env, "RgKAFK5djSk", "Hello World", "opus", true)

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if string(body) != payload {
		t.Errorf("client body: got %q, want %q", body, payload)
	}

	wantPath := filepath.Join(dlDir, library.AudioFilename("Hello World", "RgKAFK5djSk", "opus"))
	raw, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read saved file %q: %v", wantPath, err)
	}
	if string(raw) != payload {
		t.Errorf("on-disk bytes: got %q, want %q", raw, payload)
	}

	got, err := env.lib.GetTrack("RgKAFK5djSk")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.AudioPath != wantPath {
		t.Errorf("AudioPath: got %q, want %q", got.AudioPath, wantPath)
	}
	if got.AudioSize != int64(len(payload)) {
		t.Errorf("AudioSize: got %d, want %d", got.AudioSize, len(payload))
	}

	entries, _ := os.ReadDir(dlDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			t.Errorf("temp .part file leaked: %s", e.Name())
		}
	}
}

func TestAudio_AutoDownload_MidStreamFailureCleansPartAndLeavesLibraryEmpty(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'partial'; exit 1`))
	dlDir := seedTrackForAutoDL(t, env, "RgKAFK5djSk", "Broken", "opus", true)

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	finalPath := filepath.Join(dlDir, library.AudioFilename("Broken", "RgKAFK5djSk", "opus"))
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Errorf("final audio file %q exists after failed yt-dlp run; auto-download must roll back on failure", finalPath)
	}
	entries, err := os.ReadDir(dlDir)
	if err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".part") {
				t.Errorf(".part file leaked after failure: %s", e.Name())
			}
		}
	}

	got, err := env.lib.GetTrack("RgKAFK5djSk")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.AudioPath != "" {
		t.Errorf("AudioPath: got %q, want empty (failed download must NOT mark library)", got.AudioPath)
	}
	if got.AudioSize != 0 {
		t.Errorf("AudioSize: got %d, want 0", got.AudioSize)
	}
}

func TestAudio_AutoDownload_DisabledLeavesNoFile(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'streamed payload'`))
	dlDir := seedTrackForAutoDL(t, env, "RgKAFK5djSk", "x", "opus", false)

	resp, err := http.Get(env.server.URL + "/audio/RgKAFK5djSk")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != "streamed payload" {
		t.Errorf("body: got %q, want streamed payload", body)
	}

	entries, err := os.ReadDir(dlDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("download dir not empty with auto-download off: %d entries", len(entries))
	}

	got, err := env.lib.GetTrack("RgKAFK5djSk")
	if err != nil {
		t.Fatalf("GetTrack: %v", err)
	}
	if got.AudioPath != "" {
		t.Errorf("AudioPath: got %q, want empty (auto-download off must not mutate library)", got.AudioPath)
	}
}

// TestSafeCacheWriter_WriteErrorSwallowedAndCleansUp locks in the contract
// that a write failure on the cache leg never propagates out of the writer
// (so io.MultiWriter cannot short-circuit and kill the live audio stream)
// and that the poisoned writer cleans up its own .part file.
func TestSafeCacheWriter_WriteErrorSwallowedAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "test.*.part")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	s := &safeCacheWriter{file: f}

	payload := []byte("some audio bytes")
	n, err := s.Write(payload)
	if err != nil {
		t.Fatalf("Write returned error: %v (must be swallowed)", err)
	}
	if n != len(payload) {
		t.Fatalf("Write n: got %d, want %d (must report full length)", n, len(payload))
	}
	if !s.poisoned {
		t.Fatalf("writer not marked poisoned after underlying failure")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf(".part file not removed after poison: stat err=%v", err)
	}

	// Subsequent writes also swallow and report success.
	n, err = s.Write(payload)
	if err != nil || n != len(payload) {
		t.Fatalf("post-poison Write: n=%d err=%v, want n=%d err=nil", n, err, len(payload))
	}
}

// TestAudio_AutoDownload_CollisionResolvedByVideoID locks in the fix for the
// cross-videoID filename collision: two tracks with identical titles must
// land on disk under distinct paths (videoID embedded) so the library row
// for one never references bytes belonging to the other.
func TestAudio_AutoDownload_CollisionResolvedByVideoID(t *testing.T) {
	const payload = "tee-stream payload bytes"
	env := newTestEnv(t, writeFakeYtdlp(t, `printf 'tee-stream payload bytes'`))
	dlDir := filepath.Join(t.TempDir(), "downloads")
	env.handlers.DownloadDir = func() string { return dlDir }
	env.handlers.AutoDownload = func() bool { return true }
	env.handlers.AudioFormat = "opus"

	const idA, idB = "RgKAFK5djSk", "dQw4w9WgXcQ"
	for _, id := range []string{idA, idB} {
		seedTrack(t, env.lib, library.Track{
			VideoID: id, Title: "Intro", DurationSec: 10,
			ThumbnailURL: "http://example.invalid/x.jpg",
			SourceURL:    "https://www.youtube.com/watch?v=" + id,
			ImportedAt:   1,
		})
		resp, err := http.Get(env.server.URL + "/audio/" + id)
		if err != nil {
			t.Fatalf("Get %s: %v", id, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	pathA := filepath.Join(dlDir, library.AudioFilename("Intro", idA, "opus"))
	pathB := filepath.Join(dlDir, library.AudioFilename("Intro", idB, "opus"))
	for _, p := range []string{pathA, pathB} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %q: %v", p, err)
		}
		if string(raw) != payload {
			t.Errorf("on-disk bytes for %q: got %q, want %q", p, raw, payload)
		}
	}

	trackA, _ := env.lib.GetTrack(idA)
	trackB, _ := env.lib.GetTrack(idB)
	if trackA.AudioPath != pathA {
		t.Errorf("track A audio_path: got %q, want %q", trackA.AudioPath, pathA)
	}
	if trackB.AudioPath != pathB {
		t.Errorf("track B audio_path: got %q, want %q", trackB.AudioPath, pathB)
	}
	if trackA.AudioPath == trackB.AudioPath {
		t.Errorf("same-title different-id tracks share audio_path %q: collision not resolved", trackA.AudioPath)
	}
}

// -- Debug endpoints -----------------------------------------------------------

func TestDebugGoroutines_ReturnsStackDump(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")

	resp, err := http.Get(env.server.URL + "/debug/goroutines")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("content-type: got %q, want text/plain", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Contains(body, []byte("goroutine ")) {
		t.Errorf("body missing goroutine header; got %d bytes starting with %q", len(body), firstN(body, 80))
	}
	if !bytes.Contains(body, []byte("runtime/pprof")) && !bytes.Contains(body, []byte("net/http.")) {
		t.Errorf("body has no stack frames; got %d bytes", len(body))
	}
}

func firstN(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
