package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

// shortenGithubAPITimeout swaps githubAPITimeout down to 50ms for the duration
// of the test so per-request timeout paths fire quickly. Restored via t.Cleanup.
func shortenGithubAPITimeout(t *testing.T) {
	t.Helper()
	prev := githubAPITimeout
	githubAPITimeout = 50 * time.Millisecond
	t.Cleanup(func() { githubAPITimeout = prev })
}

// redirectLatestAPI points both ytdlpStableAPI and ytdlpNightlyAPI at url for
// the duration of the test so callers exercising either channel hit the
// httptest server. Restored via t.Cleanup.
func redirectLatestAPI(t *testing.T, url string) {
	t.Helper()
	prevStable, prevNightly := ytdlpStableAPI, ytdlpNightlyAPI
	ytdlpStableAPI, ytdlpNightlyAPI = url, url
	t.Cleanup(func() {
		ytdlpStableAPI, ytdlpNightlyAPI = prevStable, prevNightly
	})
}

// NOTE: the GitHub Releases download flow against the real network is not
// covered here; tests use httptest.Server through redirectLatestAPI.

func TestChannelAPIURL(t *testing.T) {
	cases := []struct {
		channel string
		want    string
	}{
		{"stable", "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"},
		{"nightly", "https://api.github.com/repos/yt-dlp/yt-dlp-nightly-builds/releases/latest"},
		{"", "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"},
		{"off", "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"},
		{"garbage", "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"},
	}
	for _, c := range cases {
		if got := channelAPIURL(c.channel); got != c.want {
			t.Errorf("channelAPIURL(%q) = %q, want %q", c.channel, got, c.want)
		}
	}
}

func TestResolveAssetName_Matrix(t *testing.T) {
	cases := []struct {
		goos    string
		goarch  string
		want    string
		wantErr bool
	}{
		{"darwin", "amd64", "yt-dlp_macos", false},
		{"darwin", "arm64", "yt-dlp_macos", false},
		{"linux", "amd64", "yt-dlp_linux", false},
		{"linux", "arm64", "yt-dlp_linux_aarch64", false},
		{"linux", "arm", "", true},
		{"linux", "mips", "", true},
		{"windows", "amd64", "yt-dlp.exe", false},
		{"windows", "arm64", "yt-dlp.exe", false},
		{"freebsd", "amd64", "", true},
		{"openbsd", "amd64", "", true},
		{"plan9", "amd64", "", true},
	}
	for _, c := range cases {
		t.Run(c.goos+"_"+c.goarch, func(t *testing.T) {
			got, err := resolveAssetName(c.goos, c.goarch)
			if c.wantErr {
				if err == nil {
					t.Errorf("resolveAssetName(%s,%s): got %q, want error", c.goos, c.goarch, got)
				}
				return
			}
			if err != nil {
				t.Errorf("resolveAssetName(%s,%s): unexpected error %v", c.goos, c.goarch, err)
			}
			if got != c.want {
				t.Errorf("resolveAssetName(%s,%s): got %q, want %q", c.goos, c.goarch, got, c.want)
			}
		})
	}
}

func TestYtdlpAssetName_DelegatesToRuntime(t *testing.T) {
	got, err := ytdlpAssetName()
	want, wantErr := resolveAssetName(runtime.GOOS, runtime.GOARCH)
	if (err == nil) != (wantErr == nil) {
		t.Fatalf("error mismatch: got %v, want %v", err, wantErr)
	}
	if got != want {
		t.Errorf("ytdlpAssetName(): got %q, want %q", got, want)
	}
}

func TestVersion_UnknownOnBadPath(t *testing.T) {
	got := Version("/nonexistent/binary/path")
	if got != "unknown" {
		t.Errorf("Version(missing): got %q, want %q", got, "unknown")
	}
}

func TestVersion_RealOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on /bin/sh")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")
	script := "#!/bin/sh\necho \"2025.06.30\"\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	got := Version(path)
	if got != "2025.06.30" {
		t.Errorf("Version(fake): got %q, want %q", got, "2025.06.30")
	}
}

func TestVersion_TrimmedOnMultilineOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on /bin/sh")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")
	script := "#!/bin/sh\nprintf '  2025.06.30  \\n'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	got := Version(path)
	if got != "2025.06.30" {
		t.Errorf("Version(spaced): got %q, want %q", got, "2025.06.30")
	}
}

func TestBridgeVersion_NonEmpty(t *testing.T) {
	if BridgeVersion == "" {
		t.Error("BridgeVersion must be set so the User-Agent is non-empty")
	}
}

// -- Retry path through fetchLatestRelease ------------------------------------

func TestFetchLatestRelease_RetriesOn500ThenSucceeds(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "2025.06.30"})
	}))
	t.Cleanup(srv.Close)

	rel, err := fetchLatestRelease(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if rel.TagName != "2025.06.30" {
		t.Errorf("TagName: got %q, want 2025.06.30", rel.TagName)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("request count: got %d, want 3", got)
	}
}

func TestFetchLatestRelease_GivesUpAfter3xRetries(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	_, err := fetchLatestRelease(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("fetchLatestRelease: got nil error after persistent 5xx")
	}
	if got := calls.Load(); got != int32(retryMaxAttempts) {
		t.Errorf("request count: got %d, want %d", got, retryMaxAttempts)
	}
}

func TestFetchLatestRelease_DoesNotRetryOn404(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, err := fetchLatestRelease(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("fetchLatestRelease: got nil error on 404")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("request count: got %d, want 1 (no retry on 4xx)", got)
	}
}

// TestFetchLatestRelease_RateLimitFriendlyError verifies that a GitHub
// rate-limit 403 (identified by X-RateLimit-Remaining: 0) surfaces as a
// human-readable error mentioning "rate limit" and is NOT retried, since the
// limit only resets at a known future time.
func TestFetchLatestRelease_RateLimitFriendlyError(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		http.Error(w, "rate limit", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	_, err := fetchLatestRelease(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error on rate-limit response")
	}
	if !strings.Contains(err.Error(), "resets in") {
		t.Errorf("error should report relative reset duration; got %q", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("rate-limit must not retry; got %d calls, want 1", got)
	}
}

// TestFetchLatestRelease_RateLimitFallbackWhenResetHeaderUnusable pins the
// fallback path: an absent or garbage X-RateLimit-Reset header still produces
// a rate-limit error (so the user sees actionable text) but uses the generic
// "try again in an hour" copy instead of inventing a wall-clock time.
func TestFetchLatestRelease_RateLimitFallbackWhenResetHeaderUnusable(t *testing.T) {
	shortenRetryBackoff(t)
	cases := []struct {
		name  string
		reset string
	}{
		{"missing header", ""},
		{"unparseable", "not-a-number"},
		{"in the past", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)},
		{"far future", strconv.FormatInt(time.Now().Add(48*time.Hour).Unix(), 10)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				if c.reset != "" {
					w.Header().Set("X-RateLimit-Reset", c.reset)
				}
				http.Error(w, "rate limit", http.StatusForbidden)
			}))
			t.Cleanup(srv.Close)
			_, err := fetchLatestRelease(context.Background(), srv.URL)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "try again in an hour") {
				t.Errorf("fallback path should suggest retry timeframe; got %q", err)
			}
		})
	}
}

// TestFetchLatestRelease_NonRateLimit403StaysGenericError pins that a plain
// 403 without the rate-limit header keeps the existing generic error shape
// and stays non-retryable (4xx terminal).
func TestFetchLatestRelease_NonRateLimit403StaysGenericError(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	_, err := fetchLatestRelease(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if strings.Contains(strings.ToLower(err.Error()), "rate limit") {
		t.Errorf("non-rate-limit 403 should not mention rate limit; got %q", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("403 must not retry; got %d calls, want 1", got)
	}
}

// TestIsRetryable_URLErrorTimeoutRetries pins the contract that a
// *url.Error wrapping context.DeadlineExceeded (the typical shape Go's http
// client returns when a per-request timeout fires) is retryable. Without
// this, a single slow GitHub response hard-fails the install.
func TestIsRetryable_URLErrorTimeoutRetries(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://example.invalid", Err: context.DeadlineExceeded}
	if !isRetryable(err) {
		t.Fatal("url.Error wrapping context.DeadlineExceeded should be retryable (per-request timeout)")
	}
}

// TestIsRetryable_CallerCancelDoesNotRetry guards that caller-side
// cancellation (context.Canceled wrapped in *url.Error) is never retried, so
// shutdown propagates immediately instead of looping through backoffs.
func TestIsRetryable_CallerCancelDoesNotRetry(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://example.invalid", Err: context.Canceled}
	if isRetryable(err) {
		t.Fatal("url.Error wrapping context.Canceled must NOT be retryable (caller cancelled)")
	}
}

// TestFetchLatestRelease_RetriesOnPerRequestTimeout exercises the integration
// path: the first two requests block past githubAPITimeout so the per-attempt
// ctx fires (surfacing as *url.Error wrapping context.DeadlineExceeded), then
// the third returns a valid release payload. With the old isRetryable that
// only matched *net.OpError, the first timeout error would have been terminal.
func TestFetchLatestRelease_RetriesOnPerRequestTimeout(t *testing.T) {
	shortenRetryBackoff(t)
	shortenGithubAPITimeout(t)

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			// Block until the request's per-attempt ctx fires so the client
			// surfaces a deadline-exceeded url.Error. Cap with a safety
			// timeout in case ctx wiring ever regresses.
			select {
			case <-r.Context().Done():
				return
			case <-time.After(2 * time.Second):
				return
			}
		}
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "ok"})
	}))
	t.Cleanup(srv.Close)

	rel, err := fetchLatestRelease(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("expected success after timeout retries: %v", err)
	}
	if rel.TagName != "ok" {
		t.Errorf("TagName: got %q, want ok", rel.TagName)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("request count: got %d, want 3", got)
	}
}

// -- I3: unknown version triggers redownload ----------------------------------

// TestRefreshIfNewer_RedownloadsWhenVersionUnknown stages a fake yt-dlp binary
// that prints garbage to `--version` (so Version returns "unknown"), points
// the test server at an asset that emits known content, and asserts the
// existing binary gets replaced and the asset endpoint was hit.
func TestRefreshIfNewer_RedownloadsWhenVersionUnknown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires /bin/sh for the fake yt-dlp")
	}
	shortenRetryBackoff(t)

	const newBinaryContent = "REPLACED_BINARY_CONTENT"
	var assetCalls atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		assetCalls.Add(1)
		_, _ = w.Write([]byte(newBinaryContent))
	})
	var apiURL string
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		assetName, err := ytdlpAssetName()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: "2025.99.99",
			Assets: []ghAsset{
				{Name: assetName, DownloadURL: apiURL + "/asset"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	apiURL = srv.URL
	redirectLatestAPI(t, srv.URL+"/releases/latest")

	dataDir := t.TempDir()
	binPath, err := binaryPath(dataDir)
	if err != nil {
		t.Fatalf("binaryPath: %v", err)
	}
	// Existing fake binary that prints garbage to --version (forces "unknown").
	script := "#!/bin/sh\necho 'not a version number'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	if got := Version(binPath); got != "not a version number" && got != "unknown" {
		// Either way the redownload should be triggered: it's not equal to TagName.
		t.Logf("Version(fake): %q", got)
	}

	refreshIfNewer(context.Background(), dataDir, "stable", nil)

	if got := assetCalls.Load(); got < 1 {
		t.Errorf("asset endpoint: got %d hits, want >=1", got)
	}
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(got) != newBinaryContent {
		t.Errorf("binary contents: got %q, want %q", got, newBinaryContent)
	}
}

// TestRefreshIfNewer_FiresOnUpgradeAfterDownload verifies the onUpgrade
// callback runs exactly once when refreshIfNewer detects a newer release on
// disk and replaces the binary. main.go installs a closure that refreshes its
// atomic version cache, so without this hook the /health endpoint and
// Settings panel keep reporting the boot-time version until restart.
func TestRefreshIfNewer_FiresOnUpgradeAfterDownload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires /bin/sh for the fake yt-dlp")
	}
	shortenRetryBackoff(t)

	const newBinaryContent = "FRESH_BINARY"
	mux := http.NewServeMux()
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(newBinaryContent))
	})
	var apiURL string
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		assetName, err := ytdlpAssetName()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: "2025.99.99",
			Assets: []ghAsset{
				{Name: assetName, DownloadURL: apiURL + "/asset"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	apiURL = srv.URL
	redirectLatestAPI(t, srv.URL+"/releases/latest")

	dataDir := t.TempDir()
	binPath, err := binaryPath(dataDir)
	if err != nil {
		t.Fatalf("binaryPath: %v", err)
	}
	// Seed a stale binary so refreshIfNewer's Version != TagName branch fires.
	script := "#!/bin/sh\necho '2024.01.01'\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}

	var calls atomic.Int32
	var gotPath atomic.Pointer[string]
	refreshIfNewer(context.Background(), dataDir, "stable", func(p string) {
		calls.Add(1)
		gotPath.Store(&p)
	})

	if got := calls.Load(); got != 1 {
		t.Errorf("onUpgrade callback: got %d calls, want 1", got)
	}
	if p := gotPath.Load(); p == nil || *p != binPath {
		var have string
		if p != nil {
			have = *p
		}
		t.Errorf("onUpgrade path: got %q, want %q", have, binPath)
	}
}

// -- A2: ForceUpdate ---------------------------------------------------------

// TestForceUpdate_OffChannelReturnsErrChannelOff asserts the package owns the
// "force update is meaningless when channel is off" contract. Without a
// registered httptest server, hitting the real network would itself be a
// loud failure, so this also implicitly verifies no fetch is attempted.
func TestForceUpdate_OffChannelReturnsErrChannelOff(t *testing.T) {
	_, err := ForceUpdate(context.Background(), t.TempDir(), "off", nil)
	if !errors.Is(err, ErrChannelOff) {
		t.Errorf("ForceUpdate(channel=off): got err=%v, want ErrChannelOff", err)
	}
}

func TestForceUpdate_KeepsExistingBinaryOnFailure(t *testing.T) {
	dataDir := t.TempDir()
	name, err := ytdlpAssetName()
	if err != nil {
		t.Skipf("no yt-dlp asset for this platform: %v", err)
	}
	binPath := filepath.Join(dataDir, name)
	if err := os.WriteFile(binPath, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	redirectLatestAPI(t, srv.URL)
	shortenRetryBackoff(t)

	var onUpgrade atomic.Int32
	if _, err := ForceUpdate(context.Background(), dataDir, "stable", func(string) { onUpgrade.Add(1) }); err == nil {
		t.Fatal("expected error from forced update against a 500 server")
	}
	if got := onUpgrade.Load(); got != 0 {
		t.Errorf("failed force update fired onUpgrade %d times, want 0", got)
	}
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("binary should still exist: %v", err)
	}
	if string(got) != "OLD" {
		t.Errorf("binary mutated by failed force update: got %q, want %q", got, "OLD")
	}
}

func TestForceUpdate_OverwritesOnSuccess(t *testing.T) {
	dataDir := t.TempDir()
	binPath, err := binaryPath(dataDir)
	if err != nil {
		t.Skipf("no yt-dlp asset for this platform: %v", err)
	}
	if err := os.WriteFile(binPath, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	const newBinaryContent = "NEW"
	const tagName = "2025.99.99"

	mux := http.NewServeMux()
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(newBinaryContent))
	})
	var apiURL string
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		assetName, err := ytdlpAssetName()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: tagName,
			Assets: []ghAsset{
				{Name: assetName, DownloadURL: apiURL + "/asset"},
			},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	apiURL = srv.URL
	redirectLatestAPI(t, srv.URL+"/releases/latest")

	var onUpgrade atomic.Int32
	var onUpgradePath atomic.Pointer[string]
	gotPath, err := ForceUpdate(context.Background(), dataDir, "stable", func(p string) {
		onUpgrade.Add(1)
		onUpgradePath.Store(&p)
	})
	if err != nil {
		t.Fatalf("ForceUpdate: %v", err)
	}
	if got := onUpgrade.Load(); got != 1 {
		t.Errorf("onUpgrade fired %d times, want 1", got)
	}
	if p := onUpgradePath.Load(); p == nil || *p != binPath {
		var have string
		if p != nil {
			have = *p
		}
		t.Errorf("onUpgrade path: got %q, want %q", have, binPath)
	}
	if gotPath != binPath {
		t.Errorf("returned path: got %q, want %q", gotPath, binPath)
	}
	contents, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if string(contents) != newBinaryContent {
		t.Errorf("binary contents: got %q, want %q", contents, newBinaryContent)
	}
	sidecar, err := os.ReadFile(versionSidecarPath(binPath))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if string(sidecar) != tagName {
		t.Errorf("sidecar contents: got %q, want %q", sidecar, tagName)
	}
}

// -- I1: boot-time check ------------------------------------------------------

// TestRefreshDaily_RunsImmediateCheckOnBoot starts RefreshDaily, cancels its
// context well before the 24h tick, and asserts the release endpoint was hit
// at least once. Without the boot-time check, count would be zero because the
// first ticker fire is 24h out.
func TestRefreshDaily_RunsImmediateCheckOnBoot(t *testing.T) {
	shortenRetryBackoff(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		// Return a release with no matching asset so downloadLatest short-circuits
		// without touching the network further.
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "0.0.0"})
	}))
	t.Cleanup(srv.Close)
	redirectLatestAPI(t, srv.URL)

	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		RefreshDaily(ctx, dataDir, func() string { return "stable" }, nil, nil)
		close(done)
	}()
	// Poll for the immediate check, then cancel so the goroutine exits the
	// 24h ticker loop without making us wait for it.
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshDaily did not exit within 2s of cancel")
	}

	if got := calls.Load(); got < 1 {
		t.Errorf("release endpoint hits: got %d, want >=1 (boot-time check missing)", got)
	}
}

func TestRefreshDaily_OffChannelSkipsImmediateCheck(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"x"}`))
	}))
	t.Cleanup(srv.Close)
	redirectLatestAPI(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RefreshDaily(ctx, t.TempDir(), func() string { return "off" }, nil, nil)
		close(done)
	}()
	// Give the immediate-check path a chance to fire if it's going to.
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshDaily did not exit within 2s of cancel")
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("off channel should not call GitHub; got %d hits", got)
	}
}

// TestRefreshOnce_FetchesAndUpdatesBinary asserts the exported one-shot
// helper exercises the same code path as a RefreshDaily tick: it fetches the
// channel's latest release and re-downloads the binary when the on-disk
// version differs. SaveConfig wires this into channel changes so a Settings
// flip propagates without waiting for the next 24h tick.
func TestRefreshOnce_FetchesAndUpdatesBinary(t *testing.T) {
	shortenRetryBackoff(t)
	dataDir := t.TempDir()
	name, err := ytdlpAssetName()
	if err != nil {
		t.Skipf("no asset for platform: %v", err)
	}
	binPath := filepath.Join(dataDir, name)
	if err := os.WriteFile(binPath, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	const tagName = "2025.99.99"
	const newBinaryContent = "NEW_AFTER_REFRESH_ONCE"
	var apiURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(newBinaryContent))
	})
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: tagName,
			Assets:  []ghAsset{{Name: name, DownloadURL: apiURL + "/asset"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	apiURL = srv.URL
	redirectLatestAPI(t, srv.URL+"/releases/latest")

	RefreshOnce(context.Background(), dataDir, "stable", nil)

	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != newBinaryContent {
		t.Errorf("binary not updated: got %q, want %q", got, newBinaryContent)
	}
}

// TestInstallBinary_ConcurrentWritersDoNotCorrupt fans out N concurrent
// installBinary calls against the same finalPath. With a single shared tmp
// suffix the writers race on os.Create and io.Copy and the rename can land a
// truncated body. With a per-call unique tmp suffix the final file contents
// must equal one of the inputs verbatim.
func TestInstallBinary_ConcurrentWritersDoNotCorrupt(t *testing.T) {
	dir := t.TempDir()
	finalPath := filepath.Join(dir, "binary")
	const N = 8
	payload := func(i int) []byte {
		// Each payload is distinct AND large enough that io.Copy can't finish
		// before a sibling truncates the shared tmp.
		var b []byte
		for range 16 * 1024 {
			b = append(b, byte('A'+i))
		}
		return b
	}
	want := make(map[string]struct{}, N)
	for i := range N {
		want[string(payload(i))] = struct{}{}
	}
	var wg sync.WaitGroup
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := installBinary(finalPath, bytes.NewReader(payload(i))); err != nil {
				t.Errorf("installBinary[%d]: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if _, ok := want[string(got)]; !ok {
		t.Fatalf("final file is not any complete payload (len=%d); concurrent writers corrupted it", len(got))
	}
}

// TestRefreshDaily_TickerHonorsOffAndLiveChannelSwitch shortens the tick
// interval so the ticker branch actually runs in test time, and flips the
// channel from "off" to "stable" mid-run to assert RefreshDaily reads
// channelFn on every tick instead of capturing at start.
func TestRefreshDaily_TickerHonorsOffAndLiveChannelSwitch(t *testing.T) {
	prev := ytdlpRefreshEvery
	ytdlpRefreshEvery = 10 * time.Millisecond
	t.Cleanup(func() { ytdlpRefreshEvery = prev })

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"x"}`))
	}))
	t.Cleanup(srv.Close)
	redirectLatestAPI(t, srv.URL)

	var channel atomic.Pointer[string]
	off := "off"
	channel.Store(&off)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RefreshDaily(ctx, t.TempDir(), func() string { return *channel.Load() }, nil, nil)
		close(done)
	}()
	// Let several ticker iterations pass with the channel pinned at "off".
	time.Sleep(100 * time.Millisecond)
	if got := hits.Load(); got != 0 {
		t.Fatalf("off channel ticker hit GitHub %d times, want 0", got)
	}
	// Flip live to "stable" and confirm the next tick hits the server.
	stable := "stable"
	channel.Store(&stable)
	deadline := time.Now().Add(2 * time.Second)
	for hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hits.Load() == 0 {
		t.Fatal("stable channel ticker never hit GitHub after live switch")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshDaily did not exit within 2s of cancel")
	}
}

// TestRefreshDaily_OverrideSetMidSessionStopsTicker exercises the I-2 fix:
// the user's binary-path override is re-read on every tick, so flipping it
// from empty to a non-empty path mid-session stops the next poll from
// hitting GitHub (the auto-managed binary is unused once the override is
// set, so there's no point downloading into it).
func TestRefreshDaily_OverrideSetMidSessionStopsTicker(t *testing.T) {
	prev := ytdlpRefreshEvery
	ytdlpRefreshEvery = 10 * time.Millisecond
	t.Cleanup(func() { ytdlpRefreshEvery = prev })

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"x"}`))
	}))
	t.Cleanup(srv.Close)
	redirectLatestAPI(t, srv.URL)

	var override atomic.Pointer[string]
	empty := ""
	override.Store(&empty)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RefreshDaily(ctx, t.TempDir(),
			func() string { return "stable" },
			func() string { return *override.Load() },
			nil)
		close(done)
	}()
	// Confirm the empty-override state hits GitHub.
	deadline := time.Now().Add(2 * time.Second)
	for hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hits.Load() == 0 {
		t.Fatal("empty override should let RefreshDaily poll, never saw a hit")
	}
	// Flip override on, then let any in-flight tick (already past the override
	// gate at the top of tickOnce) drain so the baseline reflects post-flip
	// quiescence rather than mid-flight state.
	set := "/opt/yt-dlp/yt-dlp"
	override.Store(&set)
	time.Sleep(100 * time.Millisecond)
	baseline := hits.Load()
	time.Sleep(100 * time.Millisecond)
	if grew := hits.Load() - baseline; grew > 0 {
		t.Errorf("override set mid-session: ticker still polled %d more times after quiesce, want 0", grew)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshDaily did not exit within 2s of cancel")
	}
}
