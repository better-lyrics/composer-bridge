package ytdlp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCookiesPath_Canonical(t *testing.T) {
	dir := t.TempDir()
	got := CookiesPath(dir)
	want := filepath.Join(dir, "cookies.txt")
	if got != want {
		t.Fatalf("CookiesPath = %q, want %q", got, want)
	}
}

func TestSaveCookies_WritesAtomically(t *testing.T) {
	dir := t.TempDir()
	content := "# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tFALSE\t0\tSID\tfoo\n"
	if err := SaveCookies(dir, content); err != nil {
		t.Fatalf("SaveCookies: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "cookies.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != content {
		t.Fatalf("content mismatch:\n got: %q\nwant: %q", got, content)
	}
}

func TestSaveCookies_RejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	err := SaveCookies(dir, "")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("SaveCookies(empty) err = %v, want error containing %q", err, "empty")
	}
}

func TestSaveCookies_RejectsNonNetscape(t *testing.T) {
	dir := t.TempDir()
	err := SaveCookies(dir, "not a cookies file at all\nrandom text\n")
	if err == nil {
		t.Fatalf("SaveCookies non-Netscape: want error, got nil")
	}
}

func TestSaveCookies_RejectsJSON(t *testing.T) {
	dir := t.TempDir()
	err := SaveCookies(dir, `[{"name": "SID", "value": "foo"}]`)
	if err == nil {
		t.Fatalf("SaveCookies JSON: want error, got nil")
	}
}

func TestSaveCookies_AcceptsNetscapeHeader(t *testing.T) {
	dir := t.TempDir()
	// Header-only file is valid yt-dlp input.
	content := "# Netscape HTTP Cookie File\n"
	if err := SaveCookies(dir, content); err != nil {
		t.Fatalf("SaveCookies header-only: %v", err)
	}
}

func TestSaveCookies_AcceptsBareDataLines(t *testing.T) {
	dir := t.TempDir()
	// Some browser exports skip the header and start straight with data.
	content := ".youtube.com\tTRUE\t/\tTRUE\t1893456000\tLOGIN_INFO\tAFmmF2\n"
	if err := SaveCookies(dir, content); err != nil {
		t.Fatalf("SaveCookies bare data: %v", err)
	}
}

func TestHasCookies(t *testing.T) {
	dir := t.TempDir()
	if HasCookies(dir) {
		t.Fatalf("HasCookies(empty dir) = true, want false")
	}
	if err := os.WriteFile(filepath.Join(dir, "cookies.txt"), []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !HasCookies(dir) {
		t.Fatalf("HasCookies(after write) = false, want true")
	}
}

func TestRemoveCookies(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cookies.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := RemoveCookies(dir); err != nil {
		t.Fatalf("RemoveCookies: %v", err)
	}
	if HasCookies(dir) {
		t.Fatalf("cookies.txt still present after RemoveCookies")
	}
}

func TestRemoveCookies_AbsentIsOK(t *testing.T) {
	dir := t.TempDir()
	if err := RemoveCookies(dir); err != nil {
		t.Fatalf("RemoveCookies on absent file: want nil, got %v", err)
	}
}
