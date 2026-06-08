package ytdlp

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// NOTE: the GitHub Releases download flow is not covered here. It is exercised
// behind a `-tags=integration` build tag in a follow-up task once integration
// tests land. Unit tests stay network-free.

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
