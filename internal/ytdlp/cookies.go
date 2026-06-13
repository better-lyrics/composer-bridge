package ytdlp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const cookiesFilename = "cookies.txt"

// CookiesPath returns the canonical location of the user-uploaded yt-dlp
// cookies file inside dataDir.
func CookiesPath(dataDir string) string {
	return filepath.Join(dataDir, cookiesFilename)
}

// BuildExtractorArgs returns the youtube:player_client + player_skip
// extractor-args string for yt-dlp. When preferPremium is true, web_music
// is prepended so YouTube Music's high-quality tier is tried first; this
// costs latency when Premium isn't actually available, hence the opt-in.
// player_skip=configs,initial_data eliminates redundant Innertube round
// trips and is always applied.
func BuildExtractorArgs(preferPremium bool) string {
	clients := "android_vr,web_safari"
	if preferPremium {
		clients = "web_music," + clients
	}
	return "youtube:player_client=" + clients + ";player_skip=configs,initial_data"
}

// HasCookies reports whether a cookies.txt exists in dataDir. Treats stat
// errors other than ErrNotExist as "absent" because the caller can't act on
// them anyway.
func HasCookies(dataDir string) bool {
	_, err := os.Stat(CookiesPath(dataDir))
	return err == nil
}

// SaveCookies writes content to <dataDir>/cookies.txt atomically. Rejects
// empty input and content that isn't either a Netscape cookies file or a
// browser-extension JSON export. JSON input is converted to Netscape on the
// way to disk because yt-dlp hard-rejects JSON cookies files.
func SaveCookies(dataDir, content string) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("cookies file is empty")
	}
	if looksLikeJSON(content) {
		converted, err := convertJSONToNetscape(content)
		if err != nil {
			return fmt.Errorf("converting JSON cookies to Netscape: %w", err)
		}
		content = converted
	}
	if !looksLikeNetscape(content) {
		return errors.New("cookies file is not in Netscape or JSON format (export from Get cookies.txt LOCALLY, Cookie-Editor, or similar)")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}
	dest := CookiesPath(dataDir)
	f, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create cookies tmp: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write([]byte(content)); err != nil {
		f.Close()
		return fmt.Errorf("write cookies tmp: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return fmt.Errorf("chmod cookies tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close cookies tmp: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename cookies tmp: %w", err)
	}
	return nil
}

// RemoveCookies deletes <dataDir>/cookies.txt. Absent file is a no-op.
func RemoveCookies(dataDir string) error {
	err := os.Remove(CookiesPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// VerifyResult summarises the outcome of a yt-dlp cookies probe.
type VerifyResult struct {
	// Loaded is true when yt-dlp parsed the cookies file without a hard
	// LoadError. False when the file was JSON, unreadable, etc.
	Loaded bool `json:"loaded"`
	// Authenticated is true when yt-dlp logged "Found YouTube account
	// cookies" on stderr, indicating the cookies contained LOGIN_INFO and a
	// SAPISID variant. Always false when Loaded is false.
	Authenticated bool `json:"authenticated"`
	// Rotated is true when yt-dlp emitted "The provided YouTube account
	// cookies are no longer valid" on stderr, indicating Google rotated
	// the session since the cookies were exported.
	Rotated bool `json:"rotated"`
	// Detail is a human-readable summary suitable for display in the
	// Settings UI.
	Detail string `json:"detail"`
}

const verifyTestURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

// VerifyCookies probes yt-dlp with the given cookies file against a stable
// YouTube URL using --verbose, then watches stderr line-by-line for the
// auth/loaded/rotated markers. As soon as one of the decisive markers
// appears, the process is killed; this avoids waiting for the full ~40-80s
// of YouTube extraction (JS challenge, format probing, etc.) when we
// already know the answer. Returns an error only when the probe cannot be
// attempted (missing file). A probe that ran but reported "JSON rejected"
// or "anonymous fallback" returns a non-nil VerifyResult with the
// appropriate flags set, not an error.
func VerifyCookies(ctx context.Context, ytdlpPath, cookiesPath string) (VerifyResult, error) {
	if _, err := os.Stat(cookiesPath); err != nil {
		return VerifyResult{}, fmt.Errorf("cookies file unreadable: %w", err)
	}
	args := []string{
		"--verbose",
		"--skip-download",
		"--print", "%(id)s",
		"--no-warnings",
		"--no-playlist",
		"--cookies", cookiesPath,
		verifyTestURL,
	}
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	cmd.WaitDelay = killWaitDelay
	cmd.Env = execEnv()
	cmd.Stdout = io.Discard
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return VerifyResult{}, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return VerifyResult{}, fmt.Errorf("start yt-dlp: %w", err)
	}

	var stderr strings.Builder
	authenticated := false
	rotated := false
	scanner := bufio.NewScanner(stderrPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		stderr.WriteString(line)
		stderr.WriteByte('\n')
		switch {
		case strings.Contains(line, "no longer valid"),
			strings.Contains(line, "have likely been rotated"):
			rotated = true
		case strings.Contains(line, "Found YouTube account cookies"):
			authenticated = true
		}
		if rotated || authenticated {
			_ = cmd.Process.Kill()
			break
		}
	}
	_, _ = io.Copy(io.Discard, stderrPipe)
	waitErr := cmd.Wait()
	se := stderr.String()

	if rotated {
		return VerifyResult{
			Loaded: true, Rotated: true,
			Detail: "The cookies have expired or been rotated. Export a fresh cookies.txt from a signed-in browser session.",
		}, nil
	}
	if authenticated {
		return VerifyResult{
			Loaded: true, Authenticated: true,
			Detail: "Cookies loaded and YouTube recognised an authenticated session.",
		}, nil
	}
	// Process exited or was killed by ctx without any decisive marker. Treat a
	// clean exit as anonymous fallback; treat a non-zero exit as a probe
	// failure with the captured stderr tail for the user to copy.
	if waitErr != nil && ctx.Err() == nil {
		return VerifyResult{
			Detail: fmt.Sprintf("yt-dlp probe failed: %v (stderr: %s)", waitErr, stderrTailFromString(se)),
		}, nil
	}
	return VerifyResult{
		Loaded: true,
		Detail: "Cookies loaded but YouTube treated the request as anonymous. The exported file may have been missing the LOGIN_INFO / SAPISID cookies.",
	}, nil
}

func stderrTailFromString(s string) string {
	if len(s) <= stderrTailLimit {
		return s
	}
	return s[len(s)-stderrTailLimit:]
}

// looksLikeJSON returns true when content's first non-whitespace character is
// "[" or "{", indicating a browser-extension JSON cookie export.
func looksLikeJSON(content string) bool {
	trimmed := strings.TrimLeft(content, " \t\n\r")
	if trimmed == "" {
		return false
	}
	c := trimmed[0]
	return c == '[' || c == '{'
}

// jsonCookie matches the shape browser extensions emit. Fields we don't use
// (sameSite, httpOnly, storeId, hostOnly, etc.) are intentionally ignored.
type jsonCookie struct {
	Domain         string  `json:"domain"`
	Name           string  `json:"name"`
	Value          string  `json:"value"`
	Path           string  `json:"path"`
	Secure         bool    `json:"secure"`
	ExpirationDate float64 `json:"expirationDate"`
	Session        bool    `json:"session"`
}

// convertJSONToNetscape parses a JSON cookie export and emits the equivalent
// Netscape cookies.txt content that yt-dlp's parser accepts. Session cookies
// (no expirationDate or session=true) are emitted with expiry 0, matching
// what the browser does at session end.
func convertJSONToNetscape(content string) (string, error) {
	var cookies []jsonCookie
	if err := json.Unmarshal([]byte(content), &cookies); err != nil {
		// Some extensions wrap the array in a top-level object.
		var wrapped struct {
			Cookies []jsonCookie `json:"cookies"`
		}
		if err2 := json.Unmarshal([]byte(content), &wrapped); err2 != nil || wrapped.Cookies == nil {
			return "", fmt.Errorf("parse JSON cookies: %w", err)
		}
		cookies = wrapped.Cookies
	}
	if len(cookies) == 0 {
		return "", errors.New("JSON cookies file is empty")
	}
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	b.WriteString("# Auto-converted from JSON by composer-bridge.\n")
	written := 0
	for _, c := range cookies {
		if c.Domain == "" || c.Name == "" {
			continue
		}
		includeSubdomains := "FALSE"
		if strings.HasPrefix(c.Domain, ".") {
			includeSubdomains = "TRUE"
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		secure := "FALSE"
		if c.Secure {
			secure = "TRUE"
		}
		var expiry int64
		if !c.Session && c.ExpirationDate > 0 {
			expiry = int64(c.ExpirationDate)
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			c.Domain, includeSubdomains, path, secure, expiry, c.Name, c.Value)
		written++
	}
	if written == 0 {
		return "", errors.New("JSON cookies file contained no usable entries")
	}
	return b.String(), nil
}

// looksLikeNetscape recognises the canonical Netscape cookies.txt header
// comment OR a line that looks like a tab-separated cookie record. yt-dlp's
// own parser accepts both header-only files and files starting straight with
// data, so we mirror that.
func looksLikeNetscape(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# Netscape HTTP Cookie File") {
			return true
		}
		if strings.HasPrefix(trimmed, "# HTTP Cookie File") {
			return true
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Count(line, "\t") >= 6 {
			return true
		}
		return false
	}
	return false
}
