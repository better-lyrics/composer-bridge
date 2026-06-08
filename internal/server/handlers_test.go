package server

import (
	"bytes"
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
	"testing"

	"github.com/boidushya/composer-bridge/internal/activity"
	"github.com/boidushya/composer-bridge/internal/library"
	"github.com/boidushya/composer-bridge/internal/ytdlp"
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
		YtdlpPath: ytdlpPath,
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
	handler := WithCORS(env.handlers.Router(), corsAllowed)
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
	if got := resp.Header.Get("Content-Type"); got != "audio/mp4" {
		t.Errorf("content-type: got %q, want audio/mp4", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("cache-control: got %q, want no-store", got)
	}
	if got := resp.Header.Get("X-Bridge-Version"); got != ytdlp.BridgeVersion {
		t.Errorf("x-bridge-version: got %q, want %q", got, ytdlp.BridgeVersion)
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
		"-f", "bestaudio[ext=m4a]/bestaudio",
		"-o", "--quiet", "--no-warnings", "--no-playlist",
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

func TestThumb_ServesCachedFile(t *testing.T) {
	env := newTestEnv(t, "/nonexistent")
	jpegBytes := tinyJPEG(t)
	cached := filepath.Join(t.TempDir(), "cached.jpg")
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

// -- Sanity: combined Router under CORS still routes everything ----------------

func TestRouter_UnderCORSWrappersStillRoutes(t *testing.T) {
	env := newTestEnv(t, writeFakeYtdlp(t, `echo "2025.06.30"`))
	wrapped := WithCORS(env.handlers.Router(), corsAllowed)
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
