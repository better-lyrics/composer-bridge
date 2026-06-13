package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeFakeYtdlp drops a shell script into t.TempDir() that runs `body` when
// invoked. The script is chmod 0755 so exec can spawn it. Skips on Windows
// because /bin/sh is not generally present. Returns the absolute script path.
func writeFakeYtdlp(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-script tests rely on /bin/sh")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "yt-dlp")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake yt-dlp: %v", err)
	}
	return path
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return raw
}

// echoFixtureScript writes the fixture into a temp file and returns a fake
// yt-dlp script that cats it to stdout. Routes through writeFakeYtdlp so it
// inherits the same Windows skip and the script ends up next to the fixture
// file under the same TempDir tree.
func echoFixtureScript(t *testing.T, fixture string) string {
	t.Helper()
	raw := readFixture(t, fixture)
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return writeFakeYtdlp(t, "cat "+jsonPath)
}

func TestFetchInfo_MusicFixture(t *testing.T) {
	script := echoFixtureScript(t, "music_frank_sinatra.json")
	info, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false)
	if err != nil {
		t.Fatalf("FetchInfo: %v", err)
	}
	if info.ID != "ZEcqHA7dbwM" {
		t.Errorf("ID: got %q, want ZEcqHA7dbwM", info.ID)
	}
	if !info.IsMusic() {
		t.Error("IsMusic: got false, want true")
	}
}

func TestFetchInfo_RegularVideoFixture(t *testing.T) {
	script := echoFixtureScript(t, "video_me_at_zoo.json")
	info, err := FetchInfo(context.Background(), script, "jNQXAC9IVRw", "", false)
	if err != nil {
		t.Fatalf("FetchInfo: %v", err)
	}
	if info.IsMusic() {
		t.Error("IsMusic: got true, want false")
	}
}

func TestStreamAudio_CopiesStdoutToWriter(t *testing.T) {
	script := writeFakeYtdlp(t, `printf 'hello world'`)
	var buf bytes.Buffer
	if err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", false, &buf); err != nil {
		t.Fatalf("StreamAudio: %v", err)
	}
	if buf.String() != "hello world" {
		t.Errorf("writer got %q, want %q", buf.String(), "hello world")
	}
}

func TestStreamAudio_LargePayload(t *testing.T) {
	// Emit exactly 1 MiB by repeating a 1 KiB block 1024 times.
	script := writeFakeYtdlp(t, `head -c 1048576 /dev/zero`)
	var buf bytes.Buffer
	if err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", false, &buf); err != nil {
		t.Fatalf("StreamAudio: %v", err)
	}
	if buf.Len() != 1<<20 {
		t.Errorf("writer Len: got %d, want %d", buf.Len(), 1<<20)
	}
}

func TestFetchInfo_RejectsInvalidVideoIDsWithoutForking(t *testing.T) {
	cases := []string{
		"too-short",
		"way_too_long_video_id",
		"has invalid space",
		"",
		"hasbadchar!",
	}
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			_, err := FetchInfo(context.Background(), "/nonexistent/binary/path", id, "", false)
			if err == nil {
				t.Fatalf("FetchInfo(%q): got nil error", id)
			}
			if !strings.Contains(err.Error(), "invalid videoID") {
				t.Errorf("FetchInfo(%q): got %v, want invalid videoID error", id, err)
			}
			// If we DID fork, we would get an exec error instead.
			if errors.Is(err, exec.ErrNotFound) {
				t.Errorf("FetchInfo(%q): forked subprocess instead of short-circuiting", id)
			}
		})
	}
}

func TestStreamAudio_RejectsInvalidVideoIDsWithoutForking(t *testing.T) {
	var buf bytes.Buffer
	err := StreamAudio(context.Background(), "/nonexistent/binary/path", "bad", "opus", "", false, &buf)
	if err == nil {
		t.Fatal("StreamAudio: got nil error")
	}
	if !strings.Contains(err.Error(), "invalid videoID") {
		t.Errorf("StreamAudio: got %v, want invalid videoID error", err)
	}
	if errors.Is(err, exec.ErrNotFound) {
		t.Error("StreamAudio: forked subprocess instead of short-circuiting")
	}
}

func TestFetchInfo_NonZeroExitSurfacesStderr(t *testing.T) {
	script := writeFakeYtdlp(t, `echo "boom" >&2 && exit 7`)
	_, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false)
	if err == nil {
		t.Fatal("FetchInfo: got nil error on exit 7")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ZEcqHA7dbwM") {
		t.Errorf("error missing videoID: %q", msg)
	}
	if !strings.Contains(msg, "stderr:") {
		t.Errorf("error missing stderr label: %q", msg)
	}
	if !strings.Contains(msg, "boom") {
		t.Errorf("error missing stderr content: %q", msg)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("error chain missing *exec.ExitError: %T %v", err, err)
	}
}

func TestStreamAudio_NonZeroExitSurfacesStderr(t *testing.T) {
	script := writeFakeYtdlp(t, `echo "audio boom" >&2 && exit 3`)
	var buf bytes.Buffer
	err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", false, &buf)
	if err == nil {
		t.Fatal("StreamAudio: got nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "ZEcqHA7dbwM") || !strings.Contains(msg, "audio boom") {
		t.Errorf("error missing context: %q", msg)
	}
}

func TestFetchInfo_StderrTailCapped(t *testing.T) {
	// 2000 bytes of "a" to stderr.
	script := writeFakeYtdlp(t, `head -c 2000 /dev/zero | tr '\0' a >&2 && exit 1`)
	_, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false)
	if err == nil {
		t.Fatal("got nil error")
	}
	msg := err.Error()
	idx := strings.Index(msg, "stderr:")
	if idx < 0 {
		t.Fatalf("no stderr label: %q", msg)
	}
	// Tail begins after "stderr: " and runs to the final ")".
	tail := strings.TrimSuffix(msg[idx+len("stderr:"):], ")")
	tail = strings.TrimSpace(tail)
	if len(tail) > stderrTailLimit {
		t.Errorf("stderr tail length %d > cap %d", len(tail), stderrTailLimit)
	}
	if len(tail) < stderrTailLimit-10 {
		t.Errorf("stderr tail length %d, expected near cap %d", len(tail), stderrTailLimit)
	}
}

func TestFetchInfo_ContextCancellationReturnsPromptly(t *testing.T) {
	script := writeFakeYtdlp(t, `sleep 30`)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := FetchInfo(ctx, script, "ZEcqHA7dbwM", "", false)
		done <- err
	}()
	// Give the subprocess a beat to actually start.
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected non-nil error on cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("FetchInfo did not return within 1s of ctx cancel")
	}
}

func TestStreamAudio_ContextCancellationReturnsPromptly(t *testing.T) {
	script := writeFakeYtdlp(t, `sleep 30`)
	ctx, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- StreamAudio(ctx, script, "ZEcqHA7dbwM", "opus", "", false, &buf)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("expected non-nil error on cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamAudio did not return within 1s of ctx cancel")
	}
}

func TestFetchInfo_EmptyStdoutPropagatesParseError(t *testing.T) {
	// Exit 0 with empty stdout: should hit Parse and fail there.
	script := writeFakeYtdlp(t, `exit 0`)
	_, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false)
	if err == nil {
		t.Fatal("expected parse error on empty stdout")
	}
	if !strings.Contains(err.Error(), "empty input") {
		t.Errorf("got %v, want parse error about empty input", err)
	}
}

func TestFetchInfo_ArgvIncludesRegressionFlags(t *testing.T) {
	// Script prints its args separated by newlines, then exits non-zero so
	// FetchInfo wraps stderr (the printed argv) into the error.
	script := writeFakeYtdlp(t, `for a in "$@"; do echo "$a" >&2; done; exit 1`)
	_, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false)
	if err == nil {
		t.Fatal("expected error to surface argv")
	}
	msg := err.Error()
	wantSubstrs := []string{
		"--no-playlist",
		"--no-warnings",
		"youtube:player_client=android_vr,web_safari;player_skip=configs,initial_data",
		"-j",
		"--skip-download",
		"https://www.youtube.com/watch?v=ZEcqHA7dbwM",
	}
	for _, s := range wantSubstrs {
		if !strings.Contains(msg, s) {
			t.Errorf("argv missing %q in: %q", s, msg)
		}
	}
}

func TestStreamAudio_ArgvIncludesRegressionFlags(t *testing.T) {
	script := writeFakeYtdlp(t, `for a in "$@"; do echo "$a" >&2; done; exit 1`)
	var buf bytes.Buffer
	err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", false, &buf)
	if err == nil {
		t.Fatal("expected error to surface argv")
	}
	msg := err.Error()
	wantSubstrs := []string{
		"-f",
		"bestaudio[acodec=opus][ext=webm]/bestaudio[ext=webm]/bestaudio[protocol!*=m3u8]/best[protocol!*=m3u8]/bestaudio/best",
		"-o",
		"--quiet",
		"--no-warnings",
		"--no-playlist",
		"youtube:player_client=android_vr,web_safari;player_skip=configs,initial_data",
		"https://www.youtube.com/watch?v=ZEcqHA7dbwM",
	}
	for _, s := range wantSubstrs {
		if !strings.Contains(msg, s) {
			t.Errorf("argv missing %q in: %q", s, msg)
		}
	}
}

func TestVideoIDRe_KnownGood(t *testing.T) {
	good := []string{
		"ZEcqHA7dbwM",
		"jNQXAC9IVRw",
		"RgKAFK5djSk",
		"abcdefghijk",
		"___________",
		"-----------",
	}
	for _, id := range good {
		if !VideoIDRe.MatchString(id) {
			t.Errorf("VideoIDRe rejected known good ID %q", id)
		}
	}
}

func TestVideoIDRe_KnownBad(t *testing.T) {
	bad := []string{
		"",
		"too-short",
		"toolongvideoid",
		"has@bad@chars",
		"with space1",
		"with/slash1",
		"with.dot.id",
	}
	for _, id := range bad {
		if VideoIDRe.MatchString(id) {
			t.Errorf("VideoIDRe accepted known bad ID %q", id)
		}
	}
}

func TestFetchInfo_PassesCookiesFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; cat <<'JSON'
{"id":"ZEcqHA7dbwM","title":"x","duration":120,"webpage_url":"https://www.youtube.com/watch?v=ZEcqHA7dbwM"}
JSON`)
	if _, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", cookies, false); err != nil {
		t.Fatalf("FetchInfo: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), "--cookies") || !strings.Contains(string(argv), cookies) {
		t.Fatalf("argv missing cookies flag: %s", argv)
	}
}

func TestFetchInfo_EmptyCookiesPath_NoFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; cat <<'JSON'
{"id":"ZEcqHA7dbwM","title":"x","duration":120,"webpage_url":"https://www.youtube.com/watch?v=ZEcqHA7dbwM"}
JSON`)
	if _, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false); err != nil {
		t.Fatalf("FetchInfo: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if strings.Contains(string(argv), "--cookies") {
		t.Fatalf("argv unexpectedly contains --cookies: %s", argv)
	}
}

func TestStreamAudio_PassesCookiesFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile)
	if err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", cookies, false, io.Discard); err != nil {
		t.Fatalf("StreamAudio: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), "--cookies") || !strings.Contains(string(argv), cookies) {
		t.Fatalf("argv missing cookies flag: %s", argv)
	}
}

func TestStreamAudio_EmptyCookiesPath_NoFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile)
	if err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", false, io.Discard); err != nil {
		t.Fatalf("StreamAudio: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if strings.Contains(string(argv), "--cookies") {
		t.Fatalf("argv unexpectedly contains --cookies: %s", argv)
	}
}

func TestFetchInfo_PreferPremiumPrependsWebMusic(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; cat <<'JSON'
{"id":"ZEcqHA7dbwM","title":"x","duration":120,"webpage_url":"https://www.youtube.com/watch?v=ZEcqHA7dbwM"}
JSON`)
	if _, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", true); err != nil {
		t.Fatalf("FetchInfo: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), "web_music") {
		t.Fatalf("argv missing web_music when preferPremium=true: %s", argv)
	}
}

func TestFetchInfo_DefaultExcludesWebMusic(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; cat <<'JSON'
{"id":"ZEcqHA7dbwM","title":"x","duration":120,"webpage_url":"https://www.youtube.com/watch?v=ZEcqHA7dbwM"}
JSON`)
	if _, err := FetchInfo(context.Background(), script, "ZEcqHA7dbwM", "", false); err != nil {
		t.Fatalf("FetchInfo: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if strings.Contains(string(argv), "web_music") {
		t.Fatalf("argv unexpectedly contains web_music when preferPremium=false: %s", argv)
	}
}

func TestStreamAudio_PreferPremiumPrependsWebMusic(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile)
	if err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", true, io.Discard); err != nil {
		t.Fatalf("StreamAudio: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), "web_music") {
		t.Fatalf("argv missing web_music when preferPremium=true: %s", argv)
	}
}

func TestStreamAudio_DefaultExcludesWebMusic(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile)
	if err := StreamAudio(context.Background(), script, "ZEcqHA7dbwM", "opus", "", false, io.Discard); err != nil {
		t.Fatalf("StreamAudio: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if strings.Contains(string(argv), "web_music") {
		t.Fatalf("argv unexpectedly contains web_music when preferPremium=false: %s", argv)
	}
}
