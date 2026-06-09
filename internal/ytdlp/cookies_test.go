package ytdlp

import (
	"context"
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

func TestVerifyCookies_Authenticated(t *testing.T) {
	// Fake yt-dlp emits the "Found YouTube account cookies" marker on stderr,
	// stdout JSON, exit 0, the authenticated happy path.
	dir := t.TempDir()
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `
echo "[debug] Found YouTube account cookies" >&2
echo "{}"
`)
	res, err := VerifyCookies(context.Background(), script, cookies)
	if err != nil {
		t.Fatalf("VerifyCookies: %v", err)
	}
	if !res.Loaded {
		t.Errorf("Loaded = false, want true")
	}
	if !res.Authenticated {
		t.Errorf("Authenticated = false, want true; Detail = %q", res.Detail)
	}
	if res.Rotated {
		t.Errorf("Rotated = true, want false")
	}
}

func TestVerifyCookies_AnonymousFallback(t *testing.T) {
	// Cookies loaded but no auth marker, yt-dlp ran as anonymous.
	dir := t.TempDir()
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `echo "{}"`)
	res, err := VerifyCookies(context.Background(), script, cookies)
	if err != nil {
		t.Fatalf("VerifyCookies: %v", err)
	}
	if !res.Loaded {
		t.Errorf("Loaded = false, want true (file parsed, just no auth)")
	}
	if res.Authenticated {
		t.Errorf("Authenticated = true, want false")
	}
}

func TestVerifyCookies_JSONRejection(t *testing.T) {
	// yt-dlp's canonical hard-fail for a JSON cookies file.
	dir := t.TempDir()
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `
echo "ERROR: Cookies file must be Netscape formatted, not JSON. See FAQ" >&2
exit 1
`)
	res, err := VerifyCookies(context.Background(), script, cookies)
	if err != nil {
		t.Fatalf("VerifyCookies: %v", err)
	}
	if res.Loaded {
		t.Errorf("Loaded = true, want false on JSON rejection")
	}
	if res.Authenticated {
		t.Errorf("Authenticated = true on JSON rejection")
	}
	if !strings.Contains(res.Detail, "Netscape") {
		t.Errorf("Detail should mention Netscape format, got %q", res.Detail)
	}
}

func TestVerifyCookies_RotatedWarning(t *testing.T) {
	// Cookies parsed (exit 0) but YouTube rotated the session.
	dir := t.TempDir()
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `
echo "WARNING: The provided YouTube account cookies are no longer valid." >&2
echo "{}"
`)
	res, err := VerifyCookies(context.Background(), script, cookies)
	if err != nil {
		t.Fatalf("VerifyCookies: %v", err)
	}
	if !res.Rotated {
		t.Errorf("Rotated = false, want true; Detail = %q", res.Detail)
	}
	if res.Authenticated {
		t.Errorf("Authenticated = true on rotated cookies")
	}
}

func TestVerifyCookies_GenericExecFailure(t *testing.T) {
	// Exit non-zero with stderr that doesn't match any known marker.
	dir := t.TempDir()
	cookies := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatalf("seed cookies: %v", err)
	}
	script := writeFakeYtdlp(t, `echo "ERROR: network unreachable" >&2; exit 1`)
	res, err := VerifyCookies(context.Background(), script, cookies)
	if err != nil {
		t.Fatalf("VerifyCookies: %v", err)
	}
	if res.Loaded || res.Authenticated {
		t.Errorf("expected Loaded=false Authenticated=false on generic failure")
	}
	if !strings.Contains(res.Detail, "network unreachable") && !strings.Contains(res.Detail, "exit") {
		t.Errorf("Detail should describe the failure, got %q", res.Detail)
	}
}

func TestVerifyCookies_RejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.txt")
	_, err := VerifyCookies(context.Background(), "/bin/true", missing)
	if err == nil {
		t.Fatalf("missing file: want error, got nil")
	}
}
