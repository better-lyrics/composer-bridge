package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// BridgeVersion is reported in the User-Agent for GitHub Releases requests.
const BridgeVersion = "0.1.0"

const (
	ytdlpRefreshEvery   = 24 * time.Hour
	ytdlpFetchTimeout   = 2 * time.Minute
	ytdlpVersionTimeout = 5 * time.Second
	githubAPITimeout    = 15 * time.Second
	retryMaxAttempts    = 3
)

// ytdlpLatestAPI is the GitHub Releases endpoint. Declared as var so tests can
// redirect it at an httptest.Server.URL via t.Cleanup.
var ytdlpLatestAPI = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"

// retryBackoffBase is the first-attempt backoff before exponential growth.
// Declared as var so tests can shrink it to keep the suite fast.
var retryBackoffBase = 500 * time.Millisecond

func ytdlpAssetName() (string, error) { return resolveAssetName(runtime.GOOS, runtime.GOARCH) }

// resolveAssetName returns the yt-dlp release asset name for the given OS/arch.
func resolveAssetName(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		return "yt-dlp_macos", nil
	case "linux":
		switch goarch {
		case "amd64":
			return "yt-dlp_linux", nil
		case "arm64":
			return "yt-dlp_linux_aarch64", nil
		case "arm":
			return "yt-dlp_linux_armv7l", nil
		}
	case "windows":
		return "yt-dlp.exe", nil
	}
	return "", fmt.Errorf("no yt-dlp binary for %s/%s", goos, goarch)
}

func binaryPath(dataDir string) (string, error) {
	name, err := ytdlpAssetName()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, name), nil
}

// Ensure returns the path to a working yt-dlp in dataDir, downloading on first run.
func Ensure(dataDir string) (string, error) {
	binPath, err := binaryPath(dataDir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}
	slog.Info("yt-dlp not found, downloading", "path", binPath)
	if err := downloadLatest(context.Background(), binPath); err != nil {
		return "", fmt.Errorf("download yt-dlp: %w", err)
	}
	return binPath, nil
}

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

// retryableHTTPError marks an HTTP status code as worth retrying. 5xx are
// transient; 4xx are terminal.
type retryableHTTPError struct{ status int }

func (e *retryableHTTPError) Error() string { return fmt.Sprintf("http %d", e.status) }

// isRetryable decides whether an error from doHTTPGet warrants another attempt.
// 5xx responses and network-layer failures (net.OpError, context deadline) are
// retryable; 4xx responses are not.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *retryableHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.status >= 500 && httpErr.status <= 599
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	return false
}

// withRetry runs op up to retryMaxAttempts times with exponential backoff plus
// jitter on each retryable failure. The first retry waits retryBackoffBase, the
// second waits 2x that with up to 50% jitter, etc. Non-retryable errors return
// immediately. Cancellation of ctx aborts the backoff sleep so shutdown isn't
// blocked by a pending retry.
func withRetry(ctx context.Context, op func() error) error {
	var err error
	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		err = op()
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		if attempt == retryMaxAttempts-1 {
			break
		}
		delay := retryBackoffBase * time.Duration(1<<attempt)
		if attempt > 0 {
			jitter := time.Duration(rand.Int63n(int64(delay / 2)))
			delay += jitter
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return err
}

// fetchLatestRelease GETs apiURL (a GitHub Releases endpoint) with retries and
// returns the decoded release payload.
func fetchLatestRelease(ctx context.Context, apiURL string) (*ghRelease, error) {
	var rel ghRelease
	err := withRetry(ctx, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, githubAPITimeout)
		defer cancel()
		req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "composer-bridge/"+BridgeVersion)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return &retryableHTTPError{status: resp.StatusCode}
		}
		rel = ghRelease{}
		return json.NewDecoder(resp.Body).Decode(&rel)
	})
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

// downloadAsset GETs assetURL with retries and copies the body into binPath.
func downloadAsset(ctx context.Context, assetURL, binPath string) error {
	return withRetry(ctx, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, ytdlpFetchTimeout)
		defer cancel()
		req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, assetURL, nil)
		req.Header.Set("User-Agent", "composer-bridge/"+BridgeVersion)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return &retryableHTTPError{status: resp.StatusCode}
		}
		return installBinary(binPath, resp.Body)
	})
}

func downloadLatest(ctx context.Context, binPath string) error {
	assetName, err := ytdlpAssetName()
	if err != nil {
		return err
	}
	rel, err := fetchLatestRelease(ctx, ytdlpLatestAPI)
	if err != nil {
		return err
	}
	idx := slices.IndexFunc(rel.Assets, func(a ghAsset) bool { return a.Name == assetName })
	if idx < 0 {
		return fmt.Errorf("asset %q not found in release %s", assetName, rel.TagName)
	}
	if err := downloadAsset(ctx, rel.Assets[idx].DownloadURL, binPath); err != nil {
		return err
	}
	writeVersionSidecar(binPath, rel.TagName)
	slog.Info("yt-dlp installed", "version", rel.TagName, "path", binPath)
	return nil
}

// versionSidecarPath returns the path of the file Ensure writes alongside the
// binary recording the version we just installed. Version() prefers this file
// over execing the binary because the .app bundle's exec sandboxing on macOS
// kills the spawned yt-dlp_macos process before it can print its version.
func versionSidecarPath(binPath string) string {
	return binPath + ".version"
}

func writeVersionSidecar(binPath, version string) {
	if version == "" {
		return
	}
	if err := os.WriteFile(versionSidecarPath(binPath), []byte(version), 0o644); err != nil {
		slog.Warn("write yt-dlp version sidecar", "err", err)
	}
}

func readVersionSidecar(binPath string) string {
	raw, err := os.ReadFile(versionSidecarPath(binPath))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func installBinary(finalPath string, body io.Reader) error {
	tmpPath := finalPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)
	if _, err := io.Copy(out, body); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpPath, finalPath)
}


// Version returns the yt-dlp release tag we last installed at ytdlpPath. The
// sidecar file is preferred because execing yt-dlp_macos from inside the macOS
// .app bundle is killed by Gatekeeper (SIGKILL after ~30s) before it can print
// its version. Falls back to execing the binary if no sidecar exists yet
// (first run before Ensure has written it).
func Version(ytdlpPath string) string {
	if v := readVersionSidecar(ytdlpPath); v != "" {
		return v
	}
	ctx, cancel := context.WithTimeout(context.Background(), ytdlpVersionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, ytdlpPath, "--version")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	out, err := cmd.Output()
	if err != nil {
		slog.Warn("ytdlp.Version exec failed",
			"err", err,
			"path", ytdlpPath,
			"elapsed", time.Since(start),
			"stderr", strings.TrimSpace(stderr.String()),
			"ctx_err", ctx.Err())
		return "unknown"
	}
	v := strings.TrimSpace(string(out))
	writeVersionSidecar(ytdlpPath, v)
	return v
}

// RefreshDaily runs an immediate check on call, then polls every 24h until
// ctx is cancelled. Failures are logged at warn level and never fatal. The
// boot-time check matters: with no immediate run, restarts shorter than 24h
// (typical for a desktop app) would never trigger a refresh and YouTube
// extractor breakages would linger.
func RefreshDaily(ctx context.Context, dataDir string) {
	refreshIfNewer(ctx, dataDir)
	tick := time.NewTicker(ytdlpRefreshEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			refreshIfNewer(ctx, dataDir)
		}
	}
}

// refreshIfNewer fetches the latest GitHub release and redownloads when the
// local binary is stale or unrunnable. If Version returns "unknown", the
// existing binary is unrunnable; redownload to recover rather than skipping.
func refreshIfNewer(ctx context.Context, dataDir string) {
	binPath, err := binaryPath(dataDir)
	if err != nil {
		slog.Warn("yt-dlp daily check: resolve path", "err", err, "dataDir", dataDir)
		return
	}
	rel, err := fetchLatestRelease(ctx, ytdlpLatestAPI)
	if err != nil {
		slog.Warn("yt-dlp daily check: fetch release", "err", err, "binPath", binPath, "dataDir", dataDir)
		return
	}
	current := Version(binPath)
	if current == rel.TagName {
		return
	}
	slog.Info("yt-dlp upgrade", "from", current, "to", rel.TagName)
	if err := downloadLatest(ctx, binPath); err != nil {
		assetName, _ := ytdlpAssetName()
		slog.Warn("yt-dlp upgrade failed", "err", err, "binPath", binPath, "assetName", assetName, "dataDir", dataDir)
	}
}
