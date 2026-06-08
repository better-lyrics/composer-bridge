package ytdlp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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

// redirectLatestAPI points the package-level ytdlpLatestAPI at url for the
// duration of the test. Restored via t.Cleanup.
func redirectLatestAPI(t *testing.T, url string) {
	t.Helper()
	prev := ytdlpLatestAPI
	ytdlpLatestAPI = url
	t.Cleanup(func() { ytdlpLatestAPI = prev })
}

// NOTE: the GitHub Releases download flow against the real network is not
// covered here; tests use httptest.Server through the ytdlpLatestAPI seam.

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
		{"linux", "arm", "yt-dlp_linux_armv7l", false},
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

	refreshIfNewer(context.Background(), dataDir)

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
		RefreshDaily(ctx, dataDir)
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
