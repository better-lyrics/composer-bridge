package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	ytdlpLatestAPI      = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"
	ytdlpRefreshEvery   = 24 * time.Hour
	ytdlpFetchTimeout   = 2 * time.Minute
	ytdlpVersionTimeout = 5 * time.Second
	githubAPITimeout    = 15 * time.Second
)

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
	if err := downloadLatest(binPath); err != nil {
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

func fetchLatestRelease() (*ghRelease, error) {
	ctx, cancel := context.WithTimeout(context.Background(), githubAPITimeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ytdlpLatestAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "composer-bridge/"+BridgeVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API: %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func downloadLatest(binPath string) error {
	assetName, err := ytdlpAssetName()
	if err != nil {
		return err
	}
	rel, err := fetchLatestRelease()
	if err != nil {
		return err
	}
	idx := slices.IndexFunc(rel.Assets, func(a ghAsset) bool { return a.Name == assetName })
	if idx < 0 {
		return fmt.Errorf("asset %q not found in release %s", assetName, rel.TagName)
	}
	assetURL := rel.Assets[idx].DownloadURL
	ctx, cancel := context.WithTimeout(context.Background(), ytdlpFetchTimeout)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	req.Header.Set("User-Agent", "composer-bridge/"+BridgeVersion)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %d", assetURL, resp.StatusCode)
	}
	if err := installBinary(binPath, resp.Body); err != nil {
		return err
	}
	slog.Info("yt-dlp installed", "version", rel.TagName, "path", binPath)
	return nil
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


// Version returns the trimmed output of `ytdlpPath --version`, or "unknown" on error.
func Version(ytdlpPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), ytdlpVersionTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, ytdlpPath, "--version").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// RefreshDaily blocks until ctx is cancelled, upgrading yt-dlp on a 24-hour tick.
// Failures are logged at warn level and never fatal.
func RefreshDaily(ctx context.Context, dataDir string) {
	tick := time.NewTicker(ytdlpRefreshEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			refreshIfNewer(dataDir)
		}
	}
}

func refreshIfNewer(dataDir string) {
	binPath, err := binaryPath(dataDir)
	if err != nil {
		slog.Warn("yt-dlp daily check: resolve path", "err", err)
		return
	}
	rel, err := fetchLatestRelease()
	if err != nil {
		slog.Warn("yt-dlp daily check: fetch release", "err", err)
		return
	}
	current := Version(binPath)
	if current == rel.TagName || current == "unknown" {
		return
	}
	slog.Info("yt-dlp upgrade", "from", current, "to", rel.TagName)
	if err := downloadLatest(binPath); err != nil {
		slog.Warn("yt-dlp upgrade failed", "err", err)
	}
}
