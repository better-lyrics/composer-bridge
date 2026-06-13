package ytdlp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeYtdlpThatLogsPATH writes a fake yt-dlp script that appends $PATH to
// logFile, then either cats the JSON fixture (for FetchInfo) or just exits
// 0 (for StreamAudio/DownloadToFile/VerifyCookies, where the test only
// cares about PATH propagation, not the output). Skips on Windows since the
// fake-script pattern relies on /bin/sh.
func fakeYtdlpThatLogsPATH(t *testing.T, logFile, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-script pattern relies on /bin/sh")
	}
	var body string
	switch mode {
	case "json":
		body = `printf "%s" "$PATH" > "` + logFile + `"
echo '{"id":"ZEcqHA7dbwM","title":"x"}'`
	case "stdout":
		body = `printf "%s" "$PATH" > "` + logFile + `"
printf "ok"`
	case "exit0":
		body = `printf "%s" "$PATH" > "` + logFile + `"
exit 0`
	default:
		t.Fatalf("unknown mode %q", mode)
	}
	return writeFakeYtdlp(t, body)
}

// envIntegrationSetup parks a sentinel deno dir, restores it on cleanup, and
// returns the dir. Callers then assert the dir appears at the front of the
// $PATH the fake yt-dlp saw. Centralised so every call-site test exercises
// the same wiring.
func envIntegrationSetup(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	sentinel := DenoBinDir(dataDir)
	if err := os.MkdirAll(sentinel, 0o755); err != nil {
		t.Fatalf("mkdir sentinel: %v", err)
	}
	prev := denoBinDir.Load()
	SetDenoBinDir(sentinel)
	t.Cleanup(func() {
		if prev != nil {
			SetDenoBinDir(*prev)
		} else {
			SetDenoBinDir("")
		}
	})
	return sentinel
}

func assertPATHStartsWithSentinel(t *testing.T, logFile, sentinel string) {
	t.Helper()
	raw, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read PATH log: %v", err)
	}
	got := string(raw)
	if !strings.HasPrefix(got, sentinel+string(os.PathListSeparator)) && got != sentinel {
		t.Errorf("PATH passed to yt-dlp does not start with sentinel deno dir:\nwant prefix: %q\ngot:         %q", sentinel, got)
	}
}

func TestFetchInfo_ChildEnvHasDenoBinDir(t *testing.T) {
	sentinel := envIntegrationSetup(t)
	logFile := filepath.Join(t.TempDir(), "path.log")
	script := fakeYtdlpThatLogsPATH(t, logFile, "json")
	if _, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false); err != nil {
		t.Fatalf("FetchInfo: %v", err)
	}
	assertPATHStartsWithSentinel(t, logFile, sentinel)
}

func TestStreamAudio_ChildEnvHasDenoBinDir(t *testing.T) {
	sentinel := envIntegrationSetup(t)
	logFile := filepath.Join(t.TempDir(), "path.log")
	script := fakeYtdlpThatLogsPATH(t, logFile, "stdout")
	var buf bytes.Buffer
	if err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", false, &buf); err != nil {
		t.Fatalf("StreamAudio: %v", err)
	}
	assertPATHStartsWithSentinel(t, logFile, sentinel)
}

func TestDownloadToFile_ChildEnvHasDenoBinDir(t *testing.T) {
	sentinel := envIntegrationSetup(t)
	logFile := filepath.Join(t.TempDir(), "path.log")
	dest := filepath.Join(t.TempDir(), "out.opus")
	// DownloadToFile stats dest after Run; the script must touch it so the
	// stat succeeds and we observe the PATH log.
	script := writeFakeYtdlp(t, `printf "%s" "$PATH" > "`+logFile+`"
touch "`+dest+`"`)
	if _, err := DownloadToFile(context.Background(), script, "ZEcqHA7dbwM", "opus", dest, "", false); err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}
	assertPATHStartsWithSentinel(t, logFile, sentinel)
}

func TestVerifyCookies_ChildEnvHasDenoBinDir(t *testing.T) {
	sentinel := envIntegrationSetup(t)
	logFile := filepath.Join(t.TempDir(), "path.log")
	// VerifyCookies requires the cookies file to stat-ok before forking.
	cookiesPath := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(cookiesPath, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("write cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `printf "%s" "$PATH" > "`+logFile+`"
echo 'Found YouTube account cookies' >&2
exit 0`)
	if _, err := VerifyCookies(context.Background(), script, cookiesPath); err != nil {
		t.Fatalf("VerifyCookies: %v", err)
	}
	assertPATHStartsWithSentinel(t, logFile, sentinel)
}

func TestVersion_ChildEnvHasDenoBinDir(t *testing.T) {
	sentinel := envIntegrationSetup(t)
	logFile := filepath.Join(t.TempDir(), "path.log")
	script := writeFakeYtdlp(t, `printf "%s" "$PATH" > "`+logFile+`"
echo 'v2099.01.01'`)
	got := Version(script)
	if got != "v2099.01.01" {
		t.Errorf("Version: got %q, want v2099.01.01", got)
	}
	assertPATHStartsWithSentinel(t, logFile, sentinel)
}

// TestAllExecSitesSetCmdEnv is the lint guard the user asked for: it scans
// the package source for exec.CommandContext calls and asserts every one is
// followed within a few lines by cmd.Env = execEnv(). Cheap insurance
// against someone adding a new exec site and forgetting the env wiring; if
// that happens the new site silently inherits the launchd-minimal PATH and
// the original bug reappears.
func TestAllExecSitesSetCmdEnv(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	const lookahead = 6 // lines to scan after each exec.CommandContext for the wiring
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		lines := strings.Split(string(raw), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "exec.CommandContext(") {
				continue
			}
			end := min(i+lookahead, len(lines))
			snippet := strings.Join(lines[i:end], "\n")
			if !strings.Contains(snippet, "cmd.Env = execEnv()") {
				t.Errorf("%s:%d: exec.CommandContext without cmd.Env = execEnv() in next %d lines:\n%s",
					f, i+1, lookahead, snippet)
			}
		}
	}
}
