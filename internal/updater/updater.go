package updater

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"golang.org/x/mod/semver"
)

// DefaultManifestURL is the publish location for the bridge's release manifest.
const DefaultManifestURL = "https://github.com/better-lyrics/composer-bridge/releases/latest/download/manifest.json"

const (
	manifestFetchTimeout = 15 * time.Second
	assetFetchTimeout    = 2 * time.Minute
	pollInterval         = 24 * time.Hour
	retryMaxAttempts     = 3
)

// retryBackoffBase is the first-attempt backoff before exponential growth.
// Declared as var so tests can shrink it to keep the suite fast.
var retryBackoffBase = 500 * time.Millisecond

// Asset describes a single platform binary published in the manifest.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Manifest is the JSON document published alongside each GitHub Release.
// Assets is keyed by runtime.GOOS, then runtime.GOARCH.
type Manifest struct {
	Version    string                      `json:"version"`
	ReleasedAt time.Time                   `json:"released_at"`
	Notes      string                      `json:"notes"`
	Assets     map[string]map[string]Asset `json:"assets"`
}

// UpdateInfo is the result of a Check call. Fields are populated even when
// Available is false so callers can show "you're on the latest" copy.
type UpdateInfo struct {
	Available  bool      `json:"available"`
	Current    string    `json:"current"`
	Latest     string    `json:"latest"`
	ReleasedAt time.Time `json:"released_at"`
	Notes      string    `json:"notes"`
	Asset      Asset     `json:"asset"`
}

// retryableHTTPError marks a 5xx response (transient) vs 4xx (terminal).
type retryableHTTPError struct{ status int }

func (e *retryableHTTPError) Error() string { return fmt.Sprintf("http %d", e.status) }

// isRetryable: 5xx and net errors retryable, 4xx terminal.
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
// jitter on each retryable failure. Non-retryable errors return immediately.
// Cancellation of ctx aborts the backoff sleep so shutdown isn't blocked by a
// pending retry.
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
			delay += time.Duration(rand.Int63n(int64(delay / 2)))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return err
}

// FetchManifest GETs manifestURL with retries and returns the decoded payload.
// The Version field is required; an empty value is rejected.
func FetchManifest(ctx context.Context, manifestURL string) (*Manifest, error) {
	var m Manifest
	err := withRetry(ctx, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, manifestFetchTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, manifestURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "composer-bridge-updater/self")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return &retryableHTTPError{status: resp.StatusCode}
		}
		m = Manifest{}
		if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
			return fmt.Errorf("decode manifest: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(m.Version) == "" {
		return nil, errors.New("manifest: empty version field")
	}
	return &m, nil
}

// Check fetches the manifest and reports whether an update is available for
// the given goos/goarch. Returns an error only when the manifest cannot be
// fetched or the platform has no published asset; version-parse failures
// degrade to Available: false with a slog warn.
func Check(ctx context.Context, manifestURL, currentVersion, goos, goarch string) (*UpdateInfo, error) {
	m, err := FetchManifest(ctx, manifestURL)
	if err != nil {
		return nil, err
	}
	archMap, ok := m.Assets[goos]
	if !ok {
		return nil, fmt.Errorf("no asset for %s/%s", goos, goarch)
	}
	asset, ok := archMap[goarch]
	if !ok {
		return nil, fmt.Errorf("no asset for %s/%s", goos, goarch)
	}
	info := &UpdateInfo{
		Current:    currentVersion,
		Latest:     m.Version,
		ReleasedAt: m.ReleasedAt,
		Notes:      m.Notes,
		Asset:      asset,
	}
	currentTag := semverTag(currentVersion)
	latestTag := semverTag(m.Version)
	if !semver.IsValid(currentTag) || !semver.IsValid(latestTag) {
		slog.Warn("updater: skipping check, unparseable semver", "current", currentVersion, "latest", m.Version)
		return info, nil
	}
	if semver.Compare(latestTag, currentTag) > 0 {
		info.Available = true
	}
	return info, nil
}

// semverTag adapts a bare semver into the "vX.Y.Z" form that
// golang.org/x/mod/semver expects.
func semverTag(v string) string {
	if v == "" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// Apply downloads the asset, verifies the SHA256 checksum from the manifest,
// and atomically replaces the running binary via selfupdate (rolls back on
// failure).
func Apply(ctx context.Context, asset Asset) error { return applyTo(ctx, asset, "") }

// ApplyAndRelaunch runs Apply and then spawns a fresh child process pointing at
// the current executable path so the OS picks up the swapped binary. The child
// is detached (cmd.Process.Release) so it survives the caller's exit. The
// caller is responsible for tearing down its own Wails / HTTP / DB resources
// and calling runtime.Quit after this returns nil; the updater package stays
// Wails-agnostic on purpose so the swap mechanics remain independently
// testable.
//
// Returns the Apply error verbatim on failure. selfupdate's rollback semantics
// mean a failed Apply leaves the on-disk binary intact, so it's safe for the
// caller to keep running.
func ApplyAndRelaunch(ctx context.Context, asset Asset) error {
	return applyAndRelaunchTo(ctx, asset, "", "")
}

// applyAndRelaunchTo is ApplyAndRelaunch with overridable swap target +
// relaunch executable so tests don't replace and spawn the test runner.
func applyAndRelaunchTo(ctx context.Context, asset Asset, targetPath, launchPath string) error {
	if err := applyTo(ctx, asset, targetPath); err != nil {
		return err
	}
	exe := launchPath
	if exe == "" {
		got, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
		exe = got
	}
	cmd := exec.Command(exe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("relaunch start: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("relaunch release: %w", err)
	}
	return nil
}

// applyTo is Apply with an overridable TargetPath so tests don't replace
// their own executable.
func applyTo(ctx context.Context, asset Asset, targetPath string) error {
	checksum, err := hex.DecodeString(asset.SHA256)
	if err != nil {
		return fmt.Errorf("decode sha256: %w", err)
	}
	return withRetry(ctx, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, assetFetchTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, asset.URL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "composer-bridge-updater/self")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return &retryableHTTPError{status: resp.StatusCode}
		}
		opts := selfupdate.Options{Checksum: checksum, TargetPath: targetPath}
		if err := selfupdate.Apply(resp.Body, opts); err != nil {
			return fmt.Errorf("selfupdate apply: %w", err)
		}
		return nil
	})
}

// PollDaily runs an immediate check on call, then polls every 24h until ctx
// is cancelled. When an update is available, onAvailable is invoked with the
// populated UpdateInfo. Repeated failures of the same shape are only logged
// once until the next success.
func PollDaily(ctx context.Context, manifestURL, currentVersion string, onAvailable func(UpdateInfo)) {
	var lastErrMsg string
	check := func() {
		info, err := Check(ctx, manifestURL, currentVersion, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			msg := err.Error()
			if msg != lastErrMsg {
				slog.Warn("updater: daily check failed", "err", err)
				lastErrMsg = msg
			}
			return
		}
		lastErrMsg = ""
		if info != nil && info.Available && onAvailable != nil {
			onAvailable(*info)
		}
	}
	check()
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			check()
		}
	}
}
