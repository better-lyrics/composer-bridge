package ytdlp

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"time"
)

// denoLatestAPI is the GitHub Releases endpoint for deno. Declared as var so
// tests can redirect it at an httptest.Server.URL.
var denoLatestAPI = "https://api.github.com/repos/denoland/deno/releases/latest"

// denoFetchTimeout bounds the per-attempt HTTP read for the deno zip. Deno
// release zips are ~30-40 MiB so the timeout has to accommodate slow links.
const denoFetchTimeout = 5 * time.Minute

// denoBinSubdir is the dir under dataDir where the bundled deno binary lives.
// Kept in its own subdir (not next to yt-dlp_macos) so PATH augmentation
// exposes deno without also exposing yt-dlp itself, which has the same name
// as a system yt-dlp the user may have installed separately.
const denoBinSubdir = "bin"

// DenoBinDir is <dataDir>/bin. Exported so main.go can pass it to
// SetDenoBinDir at startup without re-deriving the path.
func DenoBinDir(dataDir string) string {
	return filepath.Join(dataDir, denoBinSubdir)
}

func denoBinaryName() string {
	if runtime.GOOS == "windows" {
		return "deno.exe"
	}
	return "deno"
}

func denoBinaryPath(dataDir string) string {
	return filepath.Join(DenoBinDir(dataDir), denoBinaryName())
}

// resolveDenoAssetName returns the deno GH release asset name for an
// OS/arch. The set of supported triples is intentionally narrow: anything we
// don't recognise returns an error and the caller logs a warning rather than
// failing startup, so users on exotic platforms still get a bridge they can
// run (just without YouTube extraction working).
func resolveDenoAssetName(goos, goarch string) (string, error) {
	switch goos {
	case "darwin":
		switch goarch {
		case "amd64":
			return "deno-x86_64-apple-darwin.zip", nil
		case "arm64":
			return "deno-aarch64-apple-darwin.zip", nil
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "deno-x86_64-unknown-linux-gnu.zip", nil
		case "arm64":
			return "deno-aarch64-unknown-linux-gnu.zip", nil
		}
	case "windows":
		if goarch == "amd64" {
			return "deno-x86_64-pc-windows-msvc.zip", nil
		}
	}
	return "", fmt.Errorf("no deno binary for %s/%s", goos, goarch)
}

func denoAssetName() (string, error) { return resolveDenoAssetName(runtime.GOOS, runtime.GOARCH) }

// EnsureDeno installs deno into <dataDir>/bin on first run. Subsequent calls
// are no-ops. Returns the path to the deno binary, or an empty path and a
// non-nil error when installation failed (callers should log and continue;
// missing deno only breaks YouTube extraction, not the rest of the app).
func EnsureDeno(dataDir string) (string, error) {
	binPath := denoBinaryPath(dataDir)
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}
	if err := os.MkdirAll(DenoBinDir(dataDir), 0o755); err != nil {
		return "", fmt.Errorf("mkdir deno bin: %w", err)
	}
	slog.Info("deno not found, downloading", "path", binPath)
	if err := downloadLatestDeno(context.Background(), binPath); err != nil {
		return "", fmt.Errorf("download deno: %w", err)
	}
	return binPath, nil
}

func downloadLatestDeno(ctx context.Context, binPath string) error {
	assetName, err := denoAssetName()
	if err != nil {
		return err
	}
	rel, err := fetchLatestRelease(ctx, denoLatestAPI)
	if err != nil {
		return err
	}
	idx := slices.IndexFunc(rel.Assets, func(a ghAsset) bool { return a.Name == assetName })
	if idx < 0 {
		return fmt.Errorf("asset %q not found in release %s", assetName, rel.TagName)
	}
	if err := downloadDenoAsset(ctx, rel.Assets[idx].DownloadURL, binPath); err != nil {
		return err
	}
	writeVersionSidecar(binPath, rel.TagName)
	slog.Info("deno installed", "version", rel.TagName, "path", binPath)
	return nil
}

// downloadDenoAsset GETs the release zip into memory, extracts the deno
// binary, and atomically installs it at binPath. Body is buffered because
// archive/zip needs a Reader+ReaderAt. ~40 MiB peak memory during install is
// acceptable for a once-per-machine bootstrap.
func downloadDenoAsset(ctx context.Context, assetURL, binPath string) error {
	return withRetry(ctx, func() error {
		reqCtx, cancel := context.WithTimeout(ctx, denoFetchTimeout)
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
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, resp.Body); err != nil {
			return err
		}
		return extractDenoBinary(buf.Bytes(), binPath)
	})
}

// extractDenoBinary scans zip for an entry whose basename matches the
// platform's deno binary name and installs it at binPath. Returns an error
// when the zip is malformed or doesn't contain the expected binary.
func extractDenoBinary(zipBytes []byte, binPath string) error {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return fmt.Errorf("open deno zip: %w", err)
	}
	want := denoBinaryName()
	for _, f := range zr.File {
		if filepath.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		err = installBinary(binPath, rc)
		_ = rc.Close()
		return err
	}
	return errors.New("deno binary not found in release zip")
}
