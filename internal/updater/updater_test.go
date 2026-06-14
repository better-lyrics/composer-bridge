package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// shortenRetryBackoff swaps retryBackoffBase down to 1ms for the duration of
// the test so retry-path tests run in milliseconds, not seconds. Restored via
// t.Cleanup.
func shortenRetryBackoff(t *testing.T) {
	t.Helper()
	prev := retryBackoffBase
	retryBackoffBase = time.Millisecond
	t.Cleanup(func() { retryBackoffBase = prev })
}

func sampleManifest(version string) Manifest {
	return Manifest{
		Version:    version,
		ReleasedAt: time.Date(2026, 6, 8, 18, 0, 0, 0, time.UTC),
		Notes:      "Bug fixes and yt-dlp bump.",
		Assets: map[string]map[string]Asset{
			"darwin": {
				"amd64": {URL: "https://example.invalid/d-amd64", SHA256: "deadbeef"},
				"arm64": {URL: "https://example.invalid/d-arm64", SHA256: "deadbeef"},
			},
			"linux": {
				"amd64": {URL: "https://example.invalid/l-amd64", SHA256: "deadbeef"},
				"arm64": {URL: "https://example.invalid/l-arm64", SHA256: "deadbeef"},
			},
			"windows": {
				"amd64": {URL: "https://example.invalid/w-amd64", SHA256: "deadbeef"},
			},
		},
	}
}

// -- FetchManifest ------------------------------------------------------------

func TestFetchManifest_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(sampleManifest("0.1.2"))
	}))
	t.Cleanup(srv.Close)

	m, err := FetchManifest(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	if m.Version != "0.1.2" {
		t.Errorf("Version: got %q, want 0.1.2", m.Version)
	}
	if m.Notes != "Bug fixes and yt-dlp bump." {
		t.Errorf("Notes: got %q", m.Notes)
	}
	if _, ok := m.Assets["darwin"]["arm64"]; !ok {
		t.Error("missing darwin/arm64 asset")
	}
}

func TestFetchManifest_RetriesOn500ThenSucceeds(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(sampleManifest("0.1.2"))
	}))
	t.Cleanup(srv.Close)

	m, err := FetchManifest(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchManifest: %v", err)
	}
	if m.Version != "0.1.2" {
		t.Errorf("Version: got %q", m.Version)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("request count: got %d, want 3", got)
	}
}

func TestFetchManifest_GivesUpAfterRetries(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	if _, err := FetchManifest(context.Background(), srv.URL); err == nil {
		t.Fatal("FetchManifest: got nil error after persistent 5xx")
	}
	if got := calls.Load(); got != int32(retryMaxAttempts) {
		t.Errorf("request count: got %d, want %d", got, retryMaxAttempts)
	}
}

func TestFetchManifest_DoesNotRetryOn404(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	if _, err := FetchManifest(context.Background(), srv.URL); err == nil {
		t.Fatal("FetchManifest: got nil error on 404")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("request count: got %d, want 1 (no retry on 4xx)", got)
	}
}

// TestIsRetryable_URLErrorTimeoutRetries pins the contract that a per-request
// timeout (DeadlineExceeded wrapped in *url.Error by Go's http client) is
// retryable: a single slow GitHub response should not hard-fail self-update.
func TestIsRetryable_URLErrorTimeoutRetries(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://example.invalid", Err: context.DeadlineExceeded}
	if !isRetryable(err) {
		t.Fatal("url.Error wrapping context.DeadlineExceeded should be retryable")
	}
}

// TestIsRetryable_CallerCancelDoesNotRetry guards that caller-side
// cancellation (e.g. app shutdown) propagates immediately without burning
// retries on a context that will never recover.
func TestIsRetryable_CallerCancelDoesNotRetry(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://example.invalid", Err: context.Canceled}
	if isRetryable(err) {
		t.Fatal("url.Error wrapping context.Canceled must NOT be retryable")
	}
}

func TestFetchManifest_InvalidJSON(t *testing.T) {
	shortenRetryBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{not json"))
	}))
	t.Cleanup(srv.Close)

	_, err := FetchManifest(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("FetchManifest: got nil error on bad JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error should mention decode: got %v", err)
	}
}

func TestFetchManifest_EmptyVersionRejected(t *testing.T) {
	shortenRetryBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(sampleManifest(""))
	}))
	t.Cleanup(srv.Close)

	_, err := FetchManifest(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("FetchManifest: got nil error on empty version")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("error should mention version: got %v", err)
	}
}

// -- Check --------------------------------------------------------------------

func newManifestServer(t *testing.T, m Manifest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(m)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheck_UpdateAvailable(t *testing.T) {
	srv := newManifestServer(t, sampleManifest("0.1.2"))
	info, err := Check(context.Background(), srv.URL, "0.1.1", "darwin", "arm64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Available {
		t.Error("Available: got false, want true")
	}
	if info.Latest != "0.1.2" || info.Current != "0.1.1" {
		t.Errorf("versions: got current=%q latest=%q", info.Current, info.Latest)
	}
	if info.Asset.URL == "" {
		t.Error("Asset.URL is empty")
	}
	if info.Notes == "" {
		t.Error("Notes is empty")
	}
	if info.ReleasedAt.IsZero() {
		t.Error("ReleasedAt is zero")
	}
}

func TestCheck_VersionEqual(t *testing.T) {
	srv := newManifestServer(t, sampleManifest("0.1.2"))
	info, err := Check(context.Background(), srv.URL, "0.1.2", "darwin", "arm64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Error("Available: got true, want false")
	}
	if info.Asset.URL == "" {
		t.Error("Asset.URL should still be populated for UI display")
	}
}

func TestCheck_VersionOlder(t *testing.T) {
	srv := newManifestServer(t, sampleManifest("0.1.0"))
	info, err := Check(context.Background(), srv.URL, "0.1.5", "darwin", "arm64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Error("Available: got true, want false on downgrade")
	}
}

func TestCheck_NoAssetForPlatform(t *testing.T) {
	m := sampleManifest("0.1.2")
	delete(m.Assets, "linux")
	srv := newManifestServer(t, m)
	_, err := Check(context.Background(), srv.URL, "0.1.1", "linux", "amd64")
	if err == nil {
		t.Fatal("Check: got nil error, want missing-asset error")
	}
	if !strings.Contains(err.Error(), "linux/amd64") {
		t.Errorf("error should mention pair: got %v", err)
	}
}

func TestCheck_NoArchForPlatform(t *testing.T) {
	m := sampleManifest("0.1.2")
	delete(m.Assets["windows"], "amd64")
	srv := newManifestServer(t, m)
	_, err := Check(context.Background(), srv.URL, "0.1.1", "windows", "amd64")
	if err == nil {
		t.Fatal("Check: got nil error, want missing-arch error")
	}
	if !strings.Contains(err.Error(), "windows/amd64") {
		t.Errorf("error should mention pair: got %v", err)
	}
}

func TestCheck_PrereleaseSemver(t *testing.T) {
	srv := newManifestServer(t, sampleManifest("0.1.2-rc.1"))
	info, err := Check(context.Background(), srv.URL, "0.1.1", "darwin", "arm64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !info.Available {
		t.Error("Available: got false, want true for prerelease > stable")
	}
}

func TestCheck_PrereleaseLowerThanStable(t *testing.T) {
	// semver rule: 0.1.2-rc.1 < 0.1.2
	srv := newManifestServer(t, sampleManifest("0.1.2-rc.1"))
	info, err := Check(context.Background(), srv.URL, "0.1.2", "darwin", "arm64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Error("Available: got true, want false (prerelease ranks below stable)")
	}
}

func TestCheck_InvalidCurrentVersionDegrades(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	srv := newManifestServer(t, sampleManifest("0.1.2"))
	info, err := Check(context.Background(), srv.URL, "garbage", "darwin", "arm64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Error("Available: got true, want false on unparseable current")
	}
	if !strings.Contains(buf.String(), "unparseable semver") {
		t.Errorf("expected slog warn about unparseable semver, log was: %s", buf.String())
	}
}

func TestCheck_InvalidLatestVersionDegrades(t *testing.T) {
	srv := newManifestServer(t, sampleManifest("not-a-version"))
	info, err := Check(context.Background(), srv.URL, "0.1.1", "darwin", "arm64")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.Available {
		t.Error("Available: got true, want false on unparseable latest")
	}
}

// -- Apply --------------------------------------------------------------------

func computeSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestApply_HappyPath(t *testing.T) {
	shortenRetryBackoff(t)
	payload := []byte("FAKE_BRIDGE_BINARY_CONTENTS_v2")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	targetPath := filepath.Join(t.TempDir(), "fake-binary")
	// selfupdate.Apply expects the target to exist when TargetPath is set so
	// it can swap atomically; create an initial file.
	if err := os.WriteFile(targetPath, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	asset := Asset{URL: srv.URL, SHA256: computeSHA256Hex(payload)}
	if err := applyTo(context.Background(), asset, targetPath); err != nil {
		t.Fatalf("applyTo: %v", err)
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("target contents: got %q, want %q", got, payload)
	}
}

func TestApply_WrongChecksum(t *testing.T) {
	shortenRetryBackoff(t)
	payload := []byte("FAKE_BRIDGE_BINARY_CONTENTS")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	targetPath := filepath.Join(t.TempDir(), "fake-binary")
	original := []byte("OLD_BINARY")
	if err := os.WriteFile(targetPath, original, 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	asset := Asset{URL: srv.URL, SHA256: computeSHA256Hex([]byte("something-else"))}
	if err := applyTo(context.Background(), asset, targetPath); err == nil {
		t.Fatal("applyTo: got nil error on checksum mismatch")
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("target should be unchanged on checksum failure: got %q", got)
	}
}

func TestApply_RetriesGiveUpOn500(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	targetPath := filepath.Join(t.TempDir(), "fake-binary")
	if err := os.WriteFile(targetPath, []byte("OLD"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	asset := Asset{URL: srv.URL, SHA256: computeSHA256Hex([]byte("x"))}
	if err := applyTo(context.Background(), asset, targetPath); err == nil {
		t.Fatal("applyTo: got nil error after persistent 5xx")
	}
	if got := calls.Load(); got != int32(retryMaxAttempts) {
		t.Errorf("request count: got %d, want %d", got, retryMaxAttempts)
	}
}

func TestApply_BadHexChecksum(t *testing.T) {
	asset := Asset{URL: "http://example.invalid", SHA256: "not-hex"}
	if err := applyTo(context.Background(), asset, ""); err == nil {
		t.Fatal("applyTo: got nil error on non-hex sha256")
	}
}

// -- ApplyAndRelaunch ---------------------------------------------------------

// TestApplyAndRelaunch_SwapsBinaryAndSpawnsChild proves the end-to-end shape:
// (a) the on-disk target is replaced with the new payload, and (b) running the
// new payload produces an observable side effect (the sentinel file the fake
// "new binary" touches on launch). We use a shell script as the fake binary
// because the test runner is itself a binary we don't want to swap.
func TestApplyAndRelaunch_SwapsBinaryAndSpawnsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary tests rely on /bin/sh")
	}
	shortenRetryBackoff(t)

	dir := t.TempDir()
	sentinel := filepath.Join(dir, "child-ran.txt")
	payload := []byte("#!/bin/sh\ntouch " + sentinel + "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	targetPath := filepath.Join(dir, "fake-bridge")
	if err := os.WriteFile(targetPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	asset := Asset{URL: srv.URL, SHA256: computeSHA256Hex(payload)}
	if err := applyAndRelaunchTo(context.Background(), asset, targetPath, targetPath, nil); err != nil {
		t.Fatalf("applyAndRelaunchTo: %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("target contents not swapped: got %q, want %q", got, payload)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sentinel); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("relaunched child never touched sentinel %q; spawn did not detach correctly", sentinel)
}

// TestApplyAndRelaunch_DoesNotSpawnWhenApplyFails confirms the relaunch step is
// gated on a successful swap. A checksum mismatch must return the Apply error
// verbatim with no child process, so the caller can keep running on the old
// binary without surprise relaunches.
func TestApplyAndRelaunch_DoesNotSpawnWhenApplyFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary tests rely on /bin/sh")
	}
	shortenRetryBackoff(t)

	dir := t.TempDir()
	sentinel := filepath.Join(dir, "should-not-exist.txt")
	payload := []byte("#!/bin/sh\ntouch " + sentinel + "\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	targetPath := filepath.Join(dir, "fake-bridge")
	original := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(targetPath, original, 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	asset := Asset{URL: srv.URL, SHA256: computeSHA256Hex([]byte("wrong"))}
	if err := applyAndRelaunchTo(context.Background(), asset, targetPath, targetPath, nil); err == nil {
		t.Fatal("applyAndRelaunchTo: got nil error on checksum mismatch")
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("target was modified despite checksum failure: got %q", got)
	}

	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(sentinel); err == nil {
		t.Errorf("child spawned despite Apply failure; sentinel %q exists", sentinel)
	}
}

// TestApplyAndRelaunch_PassesExtraArgsToChild proves the parent forwards every
// argument in extraArgs to the relaunched child. RelaunchUpdatedFlag is the
// only real caller, so this exercises the SingleInstanceLock handshake by
// having the fake child write its own argv to a sentinel file. Without this
// guarantee the child cannot tell it was relaunched from an update and will
// race the parent's flock instead of sleeping first.
//
// The script loops $@ explicitly rather than relying on printf reusing its
// format because some sh implementations (e.g. dash) only emit the first arg
// when printf is the very last redirection before exit.
func TestApplyAndRelaunch_PassesExtraArgsToChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary tests rely on /bin/sh")
	}
	shortenRetryBackoff(t)

	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	payload := []byte("#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> " + argvFile + "; done\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	targetPath := filepath.Join(dir, "fake-bridge")
	if err := os.WriteFile(targetPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	asset := Asset{URL: srv.URL, SHA256: computeSHA256Hex(payload)}
	if err := applyAndRelaunchTo(context.Background(), asset, targetPath, targetPath, []string{RelaunchUpdatedFlag, "another"}); err != nil {
		t.Fatalf("applyAndRelaunchTo: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(argvFile); err == nil && strings.Contains(string(data), "another") {
			got = string(data)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got == "" {
		// One last read so the error message shows whatever the child wrote
		// before timing out, instead of an empty string.
		if data, _ := os.ReadFile(argvFile); len(data) > 0 {
			got = string(data)
		}
		t.Fatalf("child argv never contained both args; got %q", got)
	}
	if !strings.Contains(got, RelaunchUpdatedFlag) {
		t.Errorf("argv missing %q: got %q", RelaunchUpdatedFlag, got)
	}
}

// TestResolveAppBundle covers the macOS bundle walk used by the default
// relaunch path. The success case has to land on a .app suffix or
// applyAndRelaunchTo falls back to direct exec (so we never run `open` on a
// path that points at the wrong thing).
func TestResolveAppBundle(t *testing.T) {
	cases := []struct {
		name     string
		exe      string
		wantOK   bool
		wantPath string
	}{
		{
			name:     "valid bundle path",
			exe:      "/Applications/Composer Bridge.app/Contents/MacOS/composer-bridge",
			wantOK:   true,
			wantPath: "/Applications/Composer Bridge.app",
		},
		{
			name:   "not under a .app",
			exe:    "/usr/local/bin/composer-bridge",
			wantOK: false,
		},
		{
			name:   "two levels up but no .app suffix",
			exe:    "/opt/foo/Contents/MacOS/composer-bridge",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := resolveAppBundle(tc.exe)
			if ok != tc.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if tc.wantOK && got != tc.wantPath {
				t.Errorf("path: got %q, want %q", got, tc.wantPath)
			}
		})
	}
}

// -- PollDaily ----------------------------------------------------------------

func TestPollDaily_ImmediateCheckFiresOnAvailable(t *testing.T) {
	shortenRetryBackoff(t)

	m := sampleManifest("99.0.0")
	// Ensure the running platform has an asset so Check doesn't error.
	if _, ok := m.Assets[runtime.GOOS]; !ok {
		m.Assets[runtime.GOOS] = map[string]Asset{}
	}
	m.Assets[runtime.GOOS][runtime.GOARCH] = Asset{URL: "https://example.invalid/x", SHA256: "deadbeef"}

	srv := newManifestServer(t, m)

	var fired atomic.Int32
	var latest atomic.Value
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		PollDaily(ctx, srv.URL, "0.0.1", func(info UpdateInfo) {
			fired.Add(1)
			latest.Store(info.Latest)
		})
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for fired.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PollDaily did not exit within 2s of cancel")
	}

	if fired.Load() != 1 {
		t.Errorf("onAvailable fired: got %d, want 1", fired.Load())
	}
	if v, _ := latest.Load().(string); v != "99.0.0" {
		t.Errorf("latest in callback: got %q, want 99.0.0", v)
	}
}

func TestPollDaily_NoUpdateDoesNotFireCallback(t *testing.T) {
	shortenRetryBackoff(t)

	m := sampleManifest("0.0.1")
	if _, ok := m.Assets[runtime.GOOS]; !ok {
		m.Assets[runtime.GOOS] = map[string]Asset{}
	}
	m.Assets[runtime.GOOS][runtime.GOARCH] = Asset{URL: "https://example.invalid/x", SHA256: "deadbeef"}

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(m)
	}))
	t.Cleanup(srv.Close)

	var fired atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		PollDaily(ctx, srv.URL, "0.0.1", func(_ UpdateInfo) { fired.Add(1) })
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PollDaily did not exit within 2s of cancel")
	}

	if fired.Load() != 0 {
		t.Errorf("onAvailable should not fire: fired %d times", fired.Load())
	}
}

func TestPollDaily_ServerErrorDoesNotPanic(t *testing.T) {
	shortenRetryBackoff(t)

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	var fired atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		PollDaily(ctx, srv.URL, "0.0.1", func(_ UpdateInfo) { fired.Add(1) })
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < int32(retryMaxAttempts) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PollDaily did not exit within 2s of cancel")
	}

	if fired.Load() != 0 {
		t.Errorf("onAvailable should not fire on errors: fired %d times", fired.Load())
	}
}

// -- DefaultManifestURL ------------------------------------------------------

func TestDefaultManifestURL_NonEmpty(t *testing.T) {
	if DefaultManifestURL == "" {
		t.Error("DefaultManifestURL must not be empty")
	}
	if !strings.HasPrefix(DefaultManifestURL, "https://") {
		t.Errorf("DefaultManifestURL should be https: got %q", DefaultManifestURL)
	}
}
