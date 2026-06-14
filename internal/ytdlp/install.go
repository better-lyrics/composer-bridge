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
	"strconv"
	"strings"
	"time"
)

// BridgeVersion is reported in the User-Agent for GitHub Releases requests.
const BridgeVersion = "0.1.0"

const (
	ytdlpFetchTimeout   = 2 * time.Minute
	ytdlpVersionTimeout = 5 * time.Second
	retryMaxAttempts    = 3
)

// githubAPITimeout caps each individual GitHub Releases request. Declared as
// var so tests can shrink it to exercise the per-request-timeout retry path
// without waiting the full production duration.
var githubAPITimeout = 15 * time.Second

// ytdlpRefreshEvery is the RefreshDaily tick interval. Declared as var so
// tests can shorten it without waiting 24h between ticks.
var ytdlpRefreshEvery = 24 * time.Hour

// ytdlpStableAPI and ytdlpNightlyAPI are the GitHub Releases endpoints for the
// two channels. Declared as vars so tests can redirect them at an
// httptest.Server.URL via t.Cleanup.
var (
	ytdlpStableAPI  = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"
	ytdlpNightlyAPI = "https://api.github.com/repos/yt-dlp/yt-dlp-nightly-builds/releases/latest"
)

func channelAPIURL(channel string) string {
	if channel == "nightly" {
		return ytdlpNightlyAPI
	}
	return ytdlpStableAPI
}

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

// BinaryPath returns the managed yt-dlp binary path inside dataDir for the
// current OS/arch, ignoring any user-set override. Callers that need the
// effective path (override or managed) should resolve that at a higher
// layer; this only knows the managed convention.
func BinaryPath(dataDir string) (string, error) {
	return binaryPath(dataDir)
}

// ErrChannelOff is returned by ForceUpdate when the caller asks to force an
// update with channel "off". Ensure intentionally does NOT return this:
// first-boot with channel=off still needs a working binary, so Ensure
// falls back to stable. ForceUpdate is the explicit user-driven path and
// respects the off preference. App-layer callers wrap this into a
// user-friendly message; package-internal callers can recover it via
// errors.Is.
var ErrChannelOff = errors.New("yt-dlp channel is off")

// ForceUpdate redownloads the channel's latest yt-dlp release into dataDir,
// bypassing Ensure's existence check. On failure the prior binary is preserved
// because installBinary uses atomic tmp+rename: the on-disk file is only
// replaced after a complete successful download. Returns ErrChannelOff if
// channel == "off". onUpgrade fires after a successful download with the
// path that was just installed so callers can refresh cached state (e.g.
// the version string main.go serves to /health) against the right binary;
// nil is allowed.
func ForceUpdate(ctx context.Context, dataDir, channel string, onUpgrade func(string)) (string, error) {
	if channel == "off" {
		return "", ErrChannelOff
	}
	binPath, err := binaryPath(dataDir)
	if err != nil {
		return "", err
	}
	if err := downloadLatest(ctx, binPath, channel); err != nil {
		return "", fmt.Errorf("download yt-dlp: %w", err)
	}
	if onUpgrade != nil {
		onUpgrade(binPath)
	}
	return binPath, nil
}

// Ensure returns the path to a working yt-dlp in dataDir, downloading on first
// run. ctx governs the HTTP fetch and retry backoff so a stuck download can be
// cancelled at app shutdown.
func Ensure(ctx context.Context, dataDir, channel string) (string, error) {
	binPath, err := binaryPath(dataDir)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}
	slog.Info("yt-dlp not found, downloading", "path", binPath, "channel", channel)
	if err := downloadLatest(ctx, binPath, channel); err != nil {
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

// githubRateLimitError builds a friendlier error message for GitHub API
// rate-limit responses. resetUnix is the value of the X-RateLimit-Reset
// header (Unix seconds). The message reports the remaining time as a
// duration rather than a wall-clock so a stale or far-future reset can't
// render as a plausible-looking current time. Falls back to a generic
// "try again in an hour" string when the header is missing, unparseable,
// in the past, or implausibly far in the future. Non-retryable because
// burning retries before reset is wasted.
func githubRateLimitError(resetUnix string) error {
	if sec, err := strconv.ParseInt(resetUnix, 10, 64); err == nil {
		delta := time.Until(time.Unix(sec, 0))
		if delta > 0 && delta < 24*time.Hour {
			return fmt.Errorf("GitHub API rate limit hit; resets in %s", delta.Round(time.Minute))
		}
	}
	return errors.New("GitHub API rate limit hit; try again in an hour")
}

// isRetryable decides whether an error from an HTTP attempt warrants another
// try. Caller-side cancellation (context.Canceled, e.g. app shutdown) is never
// retryable so it propagates immediately. Per-request timeouts
// (context.DeadlineExceeded, typically wrapped in a *url.Error when the
// per-attempt ctx fires inside http.Client.Do) ARE retryable: a single slow
// GitHub response should not hard-fail the install. 5xx responses and
// lower-level network failures (*net.OpError) are retryable; 4xx responses
// are not.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var httpErr *retryableHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.status >= 500 && httpErr.status <= 599
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

// withRetry runs op up to retryMaxAttempts times with exponential backoff plus
// jitter on each retryable failure. The first retry waits retryBackoffBase, the
// second waits 2x that with up to 50% jitter, etc. Non-retryable errors return
// immediately. Cancellation of ctx aborts the backoff sleep so shutdown isn't
// blocked by a pending retry.
func withRetry(ctx context.Context, op func() error) error {
	var err error
	for attempt := range retryMaxAttempts {
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
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return githubRateLimitError(resp.Header.Get("X-RateLimit-Reset"))
		}
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

func downloadLatest(ctx context.Context, binPath, channel string) error {
	assetName, err := ytdlpAssetName()
	if err != nil {
		return err
	}
	rel, err := fetchLatestRelease(ctx, channelAPIURL(channel))
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
	// CreateTemp gives a unique tmp path per call so concurrent installBinary
	// invocations (e.g. two SaveConfig-driven RefreshOnce goroutines racing)
	// cannot truncate each other mid-write.
	out, err := os.CreateTemp(filepath.Dir(finalPath), filepath.Base(finalPath)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := out.Name()
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
	cmd.Env = execEnv()
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
// ctx is cancelled. channelFn and overrideFn are re-read on every tick so
// Settings changes take effect without restarting the app: a non-empty
// override means the user manages the binary themselves and we skip the
// fetch entirely, and "off" from channelFn skips the fetch for that tick.
// Failures are logged at warn level and never fatal. The boot-time check
// matters: with no immediate run, restarts shorter than 24h (typical for a
// desktop app) would never trigger a refresh and YouTube extractor
// breakages would linger. onUpgrade fires after every successful download
// with the path that was just installed, so callers can refresh any cached
// version string against the right binary; nil is allowed.
func RefreshDaily(ctx context.Context, dataDir string, channelFn, overrideFn func() string, onUpgrade func(string)) {
	if channelFn == nil {
		channelFn = func() string { return "stable" }
	}
	if overrideFn == nil {
		overrideFn = func() string { return "" }
	}
	tickOnce := func() {
		if overrideFn() != "" {
			return
		}
		if ch := channelFn(); ch != "off" {
			refreshIfNewer(ctx, dataDir, ch, onUpgrade)
		}
	}
	tickOnce()
	tick := time.NewTicker(ytdlpRefreshEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			tickOnce()
		}
	}
}

// RefreshOnce performs a single upgrade check against the channel's GitHub
// release and re-downloads the binary if a newer version is available. Same
// semantics as one RefreshDaily tick. Exposed so callers outside the package
// (notably SaveConfig handlers) can kick a refresh without waiting for the
// next 24h tick. Returns nothing because refreshIfNewer logs warns rather
// than bubbling errors, consistent with the daily-poll pattern. onUpgrade
// fires after a successful download with the path that was just installed
// so the caller can refresh any cached version string against the right
// binary; nil is allowed.
func RefreshOnce(ctx context.Context, dataDir, channel string, onUpgrade func(string)) {
	if channel == "off" {
		return
	}
	refreshIfNewer(ctx, dataDir, channel, onUpgrade)
}

// refreshIfNewer fetches the latest GitHub release and redownloads when the
// local binary is stale or unrunnable. If Version returns "unknown", the
// existing binary is unrunnable; redownload to recover rather than skipping.
// onUpgrade fires once after a successful download with the installed path
// so callers can refresh any cached version string against the right
// binary; nil skips the notification.
func refreshIfNewer(ctx context.Context, dataDir, channel string, onUpgrade func(string)) {
	binPath, err := binaryPath(dataDir)
	if err != nil {
		slog.Warn("yt-dlp daily check: resolve path", "err", err, "dataDir", dataDir)
		return
	}
	rel, err := fetchLatestRelease(ctx, channelAPIURL(channel))
	if err != nil {
		slog.Warn("yt-dlp daily check: fetch release", "err", err, "channel", channel, "binPath", binPath, "dataDir", dataDir)
		return
	}
	current := Version(binPath)
	if current == rel.TagName {
		return
	}
	slog.Info("yt-dlp upgrade", "from", current, "to", rel.TagName, "channel", channel)
	if err := downloadLatest(ctx, binPath, channel); err != nil {
		assetName, _ := ytdlpAssetName()
		slog.Warn("yt-dlp upgrade failed", "err", err, "binPath", binPath, "assetName", assetName, "dataDir", dataDir)
		return
	}
	if onUpgrade != nil {
		onUpgrade(binPath)
	}
}
