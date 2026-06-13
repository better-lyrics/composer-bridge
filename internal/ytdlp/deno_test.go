package ytdlp

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// redirectDenoAPI points denoLatestAPI at url for the duration of the test.
// Mirrors redirectLatestAPI from install_test.go.
func redirectDenoAPI(t *testing.T, url string) {
	t.Helper()
	prev := denoLatestAPI
	denoLatestAPI = url
	t.Cleanup(func() { denoLatestAPI = prev })
}

// makeDenoZip builds an in-memory zip containing a single binary entry with
// the platform-correct name. binaryContent is what extractDenoBinary should
// install verbatim. Used by the download/install integration tests.
func makeDenoZip(t *testing.T, binaryContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(denoBinaryName())
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := f.Write([]byte(binaryContent)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestResolveDenoAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{"darwin", "arm64", "deno-aarch64-apple-darwin.zip", false},
		{"darwin", "amd64", "deno-x86_64-apple-darwin.zip", false},
		{"linux", "amd64", "deno-x86_64-unknown-linux-gnu.zip", false},
		{"linux", "arm64", "deno-aarch64-unknown-linux-gnu.zip", false},
		{"windows", "amd64", "deno-x86_64-pc-windows-msvc.zip", false},
		{"darwin", "ppc64", "", true},
		{"freebsd", "amd64", "", true},
		{"linux", "mips", "", true},
		{"windows", "arm64", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			got, err := resolveDenoAssetName(tc.goos, tc.goarch)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error for %s/%s, got %q", tc.goos, tc.goarch, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDenoBinDir_UnderDataDir(t *testing.T) {
	dataDir := "/some/data"
	got := DenoBinDir(dataDir)
	want := filepath.Join(dataDir, "bin")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnsureDeno_NoOpWhenBinaryPresent(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(DenoBinDir(dataDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	const stamp = "PRE_EXISTING_DENO"
	binPath := denoBinaryPath(dataDir)
	if err := os.WriteFile(binPath, []byte(stamp), 0o755); err != nil {
		t.Fatalf("write fake deno: %v", err)
	}
	// Point the API at an httptest server that fails the test if hit, to
	// prove EnsureDeno short-circuits without ever calling the network.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("EnsureDeno hit the network despite an existing binary")
	}))
	t.Cleanup(srv.Close)
	redirectDenoAPI(t, srv.URL)

	got, err := EnsureDeno(dataDir)
	if err != nil {
		t.Fatalf("EnsureDeno: %v", err)
	}
	if got != binPath {
		t.Errorf("got path %q, want %q", got, binPath)
	}
	body, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(body) != stamp {
		t.Errorf("binary contents changed: got %q, want %q", body, stamp)
	}
}

func TestEnsureDeno_DownloadsAndInstalls(t *testing.T) {
	shortenRetryBackoff(t)

	const stamp = "FRESH_DENO_BINARY"
	zipBytes := makeDenoZip(t, stamp)

	mux := http.NewServeMux()
	var apiURL string
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(zipBytes)
	})
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		assetName, err := denoAssetName()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(ghRelease{
			TagName: "v1.99.0",
			Assets:  []ghAsset{{Name: assetName, DownloadURL: apiURL + "/asset"}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	apiURL = srv.URL
	redirectDenoAPI(t, srv.URL+"/releases/latest")

	dataDir := t.TempDir()
	got, err := EnsureDeno(dataDir)
	if err != nil {
		t.Fatalf("EnsureDeno: %v", err)
	}
	if got != denoBinaryPath(dataDir) {
		t.Errorf("path: got %q, want %q", got, denoBinaryPath(dataDir))
	}
	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read installed: %v", err)
	}
	if string(body) != stamp {
		t.Errorf("contents: got %q, want %q", body, stamp)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Errorf("installed deno not executable: mode=%v", info.Mode())
	}
	// Sidecar carries the version so Settings UI / health endpoints can
	// report it without execing the binary.
	side, err := os.ReadFile(versionSidecarPath(got))
	if err != nil {
		t.Errorf("version sidecar missing: %v", err)
	} else if strings.TrimSpace(string(side)) != "v1.99.0" {
		t.Errorf("sidecar: got %q, want v1.99.0", side)
	}
}

func TestEnsureDeno_ErrorsWhenAssetMissing(t *testing.T) {
	shortenRetryBackoff(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "v1.99.0", Assets: []ghAsset{}})
	}))
	t.Cleanup(srv.Close)
	redirectDenoAPI(t, srv.URL)
	if _, err := EnsureDeno(t.TempDir()); err == nil {
		t.Fatal("EnsureDeno: want error when asset missing from release, got nil")
	}
}

func TestExtractDenoBinary_RejectsZipWithoutBinaryEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("README.md")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	_, _ = f.Write([]byte("hi"))
	_ = zw.Close()

	dataDir := t.TempDir()
	if err := os.MkdirAll(DenoBinDir(dataDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = extractDenoBinary(buf.Bytes(), denoBinaryPath(dataDir))
	if err == nil {
		t.Fatal("extractDenoBinary: want error for zip missing the deno entry, got nil")
	}
	if !strings.Contains(err.Error(), "deno binary not found") {
		t.Errorf("error: got %v, want contains \"deno binary not found\"", err)
	}
}

func TestExtractDenoBinary_RejectsCorruptZip(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(DenoBinDir(dataDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err := extractDenoBinary([]byte("this is not a zip"), denoBinaryPath(dataDir))
	if err == nil {
		t.Fatal("want error for corrupt zip, got nil")
	}
	if !strings.Contains(err.Error(), "open deno zip") {
		t.Errorf("error: got %v, want contains \"open deno zip\"", err)
	}
}

func TestExtractDenoBinary_FindsBinaryInSubdirectory(t *testing.T) {
	// Some upstreams package the binary in a top-level dir; extract should
	// still find it by basename.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("deno-2.0.0/" + denoBinaryName())
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	const stamp = "NESTED_DENO_BINARY"
	_, _ = f.Write([]byte(stamp))
	_ = zw.Close()

	dataDir := t.TempDir()
	if err := os.MkdirAll(DenoBinDir(dataDir), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	binPath := denoBinaryPath(dataDir)
	if err := extractDenoBinary(buf.Bytes(), binPath); err != nil {
		t.Fatalf("extractDenoBinary: %v", err)
	}
	body, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != stamp {
		t.Errorf("contents: got %q, want %q", body, stamp)
	}
}

func TestEnsureDeno_RetriesTransientServerError(t *testing.T) {
	shortenRetryBackoff(t)
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 2 {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(ghRelease{TagName: "v1.99.0", Assets: []ghAsset{}})
	}))
	t.Cleanup(srv.Close)
	redirectDenoAPI(t, srv.URL)
	if _, err := EnsureDeno(t.TempDir()); err == nil {
		t.Fatal("expected error from empty asset list, got nil")
	}
	if attempts < 2 {
		t.Errorf("attempts: got %d, want >=2 (should have retried the 502)", attempts)
	}
}

// TestEnsureDeno_DoesNotRetryClientError covers the 404 case (asset URL gone)
// to make sure withRetry's 4xx-is-terminal classification carried over from
// the yt-dlp install path is exercised against the deno install path too.
func TestEnsureDeno_DoesNotRetryClientError(t *testing.T) {
	shortenRetryBackoff(t)
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(w, "gone", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	redirectDenoAPI(t, srv.URL)
	_, err := EnsureDeno(t.TempDir())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("attempts: got %d, want 1 (4xx must not retry)", attempts)
	}
}
