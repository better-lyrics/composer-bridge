package ytdlp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatSelector_KnownKeys(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"opus", "bestaudio[acodec=opus][ext=webm]/bestaudio[ext=webm]/bestaudio[protocol!*=m3u8]/best[protocol!*=m3u8]/bestaudio/best"},
		{"", "bestaudio[acodec=opus][ext=webm]/bestaudio[ext=webm]/bestaudio[protocol!*=m3u8]/best[protocol!*=m3u8]/bestaudio/best"},
		{"m4a", "bestaudio[ext=m4a]/bestaudio/best"},
		{"webm", "bestaudio[ext=webm]/bestaudio/best"},
		{"mp3", "bestaudio/best"},
		{"garbage", "bestaudio[acodec=opus][ext=webm]/bestaudio[ext=webm]/bestaudio[protocol!*=m3u8]/best[protocol!*=m3u8]/bestaudio/best"},
	}
	for _, tt := range tests {
		if got := FormatSelector(tt.format); got != tt.want {
			t.Errorf("FormatSelector(%q): got %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestFormatExtension_KnownKeys(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"opus", "opus"},
		{"", "opus"},
		{"m4a", "m4a"},
		{"webm", "webm"},
		{"mp3", "mp3"},
		{"garbage", "opus"},
	}
	for _, tt := range tests {
		if got := FormatExtension(tt.format); got != tt.want {
			t.Errorf("FormatExtension(%q): got %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestDownloadToFile_PassesCookiesFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	dest := filepath.Join(dir, "out.opus")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; `+touchOutputArg)
	if _, err := DownloadToFile(context.Background(), script, "ZEcqHA7dbwM", "opus", dest, cookies, false); err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), "--cookies") || !strings.Contains(string(argv), cookies) {
		t.Fatalf("argv missing cookies flag: %s", argv)
	}
}

func TestDownloadToFile_EmptyCookiesPath_NoFlag(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	dest := filepath.Join(dir, "out.opus")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; `+touchOutputArg)
	if _, err := DownloadToFile(context.Background(), script, "ZEcqHA7dbwM", "opus", dest, "", false); err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if strings.Contains(string(argv), "--cookies") {
		t.Fatalf("argv unexpectedly contains --cookies: %s", argv)
	}
}

func TestDownloadToFile_PreferPremiumPrependsWebMusic(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	dest := filepath.Join(dir, "out.opus")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; `+touchOutputArg)
	if _, err := DownloadToFile(context.Background(), script, "ZEcqHA7dbwM", "opus", dest, "", true); err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if !strings.Contains(string(argv), "web_music") {
		t.Fatalf("argv missing web_music when preferPremium=true: %s", argv)
	}
}

func TestDownloadToFile_DefaultExcludesWebMusic(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv.txt")
	dest := filepath.Join(dir, "out.opus")
	script := writeFakeYtdlp(t, `printf '%s\n' "$@" > `+argvFile+` ; `+touchOutputArg)
	if _, err := DownloadToFile(context.Background(), script, "ZEcqHA7dbwM", "opus", dest, "", false); err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	if strings.Contains(string(argv), "web_music") {
		t.Fatalf("argv unexpectedly contains web_music when preferPremium=false: %s", argv)
	}
}

// touchOutputArg is a fake yt-dlp body that creates the file passed to -o.
const touchOutputArg = `prev=; for a in "$@"; do [ "$prev" = -o ] && : > "$a"; prev=$a; done`

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// yt-dlp with --force-overwrites deletes an existing -o target before it
// downloads, so a failed re-download must never point -o at the user's file.
func TestDownloadToFile_RegressionFailedRedownloadKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "Song [ZEcqHA7dbwM].opus")
	if err := os.WriteFile(dest, []byte("OLD"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	script := writeFakeYtdlp(t, `prev=; for a in "$@"; do [ "$prev" = -o ] && rm -f "$a"; prev=$a; done; exit 1`)

	if _, err := DownloadToFile(context.Background(), script, "ZEcqHA7dbwM", "opus", dest, "", false); err == nil {
		t.Fatal("DownloadToFile: want error from failing yt-dlp")
	}
	if body, err := os.ReadFile(dest); err != nil || string(body) != "OLD" {
		t.Errorf("existing file after failed re-download: got %q, %v; want OLD", body, err)
	}
	if names := listDir(t, dir); len(names) != 1 {
		t.Errorf("leftover files after failed download: %q", names)
	}
}

func TestDownloadToFile_SuccessfulRedownloadReplacesFileInPlace(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "Song [ZEcqHA7dbwM].opus")
	if err := os.WriteFile(dest, []byte("OLD"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	script := writeFakeYtdlp(t, `prev=; for a in "$@"; do [ "$prev" = -o ] && printf NEW > "$a"; prev=$a; done`)

	size, err := DownloadToFile(context.Background(), script, "ZEcqHA7dbwM", "opus", dest, "", false)
	if err != nil {
		t.Fatalf("DownloadToFile: %v", err)
	}
	if body, _ := os.ReadFile(dest); string(body) != "NEW" || size != 3 {
		t.Errorf("dest: got %q (size %d), want NEW (3)", body, size)
	}
	if names := listDir(t, dir); len(names) != 1 {
		t.Errorf("leftover files after download: %q", names)
	}
}
