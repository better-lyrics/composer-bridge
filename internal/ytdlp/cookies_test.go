package ytdlp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildExtractorArgs_DefaultExcludesWebMusic(t *testing.T) {
	got := BuildExtractorArgs(false)
	if strings.Contains(got, "web_music") {
		t.Errorf("default args should not include web_music, got %q", got)
	}
	if !strings.Contains(got, "player_skip=configs,initial_data") {
		t.Errorf("default args should include player_skip, got %q", got)
	}
}

func TestBuildExtractorArgs_PreferPremiumPrependsWebMusic(t *testing.T) {
	got := BuildExtractorArgs(true)
	if !strings.HasPrefix(got, "youtube:player_client=web_music,") {
		t.Errorf("prefer-premium args should start with web_music, got %q", got)
	}
	if !strings.Contains(got, "android_vr") || !strings.Contains(got, "web_safari") {
		t.Errorf("prefer-premium args should still include fallback clients, got %q", got)
	}
}

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

func TestSaveCookies_AcceptsAndConvertsJSON(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `[{"domain":".youtube.com","name":"SID","value":"x","path":"/","secure":true,"expirationDate":1000}]`
	if err := SaveCookies(dir, jsonContent); err != nil {
		t.Fatalf("SaveCookies(JSON): want nil, got %v", err)
	}
	if !HasCookies(dir) {
		t.Fatalf("file should be written after JSON conversion")
	}
}

func TestSaveCookies_JSONRoundTripsAllFields(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `[
		{"domain":".youtube.com","name":"SID","value":"abc123","path":"/","secure":true,"expirationDate":1893456000.5},
		{"domain":".youtube.com","name":"LOGIN_INFO","value":"AFmmF2","path":"/","secure":true,"expirationDate":1893456000}
	]`
	if err := SaveCookies(dir, jsonContent); err != nil {
		t.Fatalf("SaveCookies: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "cookies.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	str := string(got)
	if !strings.HasPrefix(str, "# Netscape HTTP Cookie File\n") {
		t.Errorf("missing Netscape header: %q", str)
	}
	if !strings.Contains(str, ".youtube.com\tTRUE\t/\tTRUE\t1893456000\tSID\tabc123") {
		t.Errorf("missing SID line in: %s", str)
	}
	if !strings.Contains(str, ".youtube.com\tTRUE\t/\tTRUE\t1893456000\tLOGIN_INFO\tAFmmF2") {
		t.Errorf("missing LOGIN_INFO line in: %s", str)
	}
}

func TestSaveCookies_JSONObjectWrapper(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `{
		"cookies": [
			{"domain":".youtube.com","name":"SID","value":"x","path":"/","secure":true,"expirationDate":9999999999}
		]
	}`
	if err := SaveCookies(dir, jsonContent); err != nil {
		t.Fatalf("SaveCookies(wrapper): %v", err)
	}
}

func TestSaveCookies_JSONSessionCookieGetsZeroExpiry(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `[
		{"domain":".youtube.com","name":"SESS","value":"tmp","path":"/","secure":true,"session":true}
	]`
	if err := SaveCookies(dir, jsonContent); err != nil {
		t.Fatalf("SaveCookies(session): %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "cookies.txt"))
	if !strings.Contains(string(got), ".youtube.com\tTRUE\t/\tTRUE\t0\tSESS\ttmp") {
		t.Errorf("session cookie should have expiry 0, got %s", got)
	}
}

func TestSaveCookies_JSONEmptyArrayRejected(t *testing.T) {
	dir := t.TempDir()
	err := SaveCookies(dir, "[]")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty JSON array: want error mentioning empty, got %v", err)
	}
}

func TestSaveCookies_JSONNoUsableEntries(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `[
		{"domain":"","name":"X","value":"y","path":"/"},
		{"domain":".youtube.com","name":"","value":"y","path":"/"}
	]`
	err := SaveCookies(dir, jsonContent)
	if err == nil || !strings.Contains(err.Error(), "usable") {
		t.Fatalf("only invalid entries: want error mentioning usable, got %v", err)
	}
}

func TestSaveCookies_MalformedJSONRejected(t *testing.T) {
	dir := t.TempDir()
	err := SaveCookies(dir, "{not valid json")
	if err == nil {
		t.Fatalf("malformed JSON: want error")
	}
}

func TestSaveCookies_JSONSkipsCookiesWithoutDomainOrName(t *testing.T) {
	dir := t.TempDir()
	jsonContent := `[
		{"domain":"","name":"X","value":"y","path":"/","secure":true,"expirationDate":1},
		{"domain":".youtube.com","name":"","value":"y","path":"/","secure":true,"expirationDate":1},
		{"domain":".youtube.com","name":"OK","value":"y","path":"/","secure":true,"expirationDate":1}
	]`
	if err := SaveCookies(dir, jsonContent); err != nil {
		t.Fatalf("SaveCookies: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "cookies.txt"))
	if !strings.Contains(string(got), "\tOK\ty") {
		t.Errorf("valid cookie was dropped: %s", got)
	}
}

func TestLooksLikeJSON(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"[", true},
		{"  \n  [{}]", true},
		{`{"a":1}`, true},
		{"# Netscape HTTP Cookie File", false},
		{"", false},
		{"  ", false},
		{".domain\tTRUE\t/\tTRUE\t0\tname\tvalue", false},
	}
	for _, tc := range cases {
		if got := looksLikeJSON(tc.in); got != tc.want {
			t.Errorf("looksLikeJSON(%q) = %v, want %v", tc.in, got, tc.want)
		}
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
