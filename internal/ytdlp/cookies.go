package ytdlp

import (
	"bytes"
	"context"
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

// HasCookies reports whether a cookies.txt exists in dataDir. Treats stat
// errors other than ErrNotExist as "absent" because the caller can't act on
// them anyway.
func HasCookies(dataDir string) bool {
	_, err := os.Stat(CookiesPath(dataDir))
	return err == nil
}

// SaveCookies writes content to <dataDir>/cookies.txt atomically. Rejects
// empty input and content that doesn't look like a Netscape cookies file.
// Detection is intentionally loose: we want to catch the user pasting JSON
// from a browser extension, but not be brittle against minor format variants.
func SaveCookies(dataDir, content string) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("cookies file is empty")
	}
	if !looksLikeNetscape(content) {
		return errors.New("cookies file is not in Netscape format (use \"Get cookies.txt LOCALLY\" or similar; JSON exports are not supported)")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data dir: %w", err)
	}
	dest := CookiesPath(dataDir)
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write cookies tmp: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
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
// YouTube URL using --verbose --dump-json --simulate, then parses stderr for
// the auth/loaded/rotated markers. Returns an error only when the probe
// cannot be attempted (missing file). A probe that ran but reported "JSON
// rejected" or "anonymous fallback" returns a non-nil VerifyResult with the
// appropriate flags set, not an error.
func VerifyCookies(ctx context.Context, ytdlpPath, cookiesPath string) (VerifyResult, error) {
	if _, err := os.Stat(cookiesPath); err != nil {
		return VerifyResult{}, fmt.Errorf("cookies file unreadable: %w", err)
	}
	args := []string{
		"--verbose",
		"--dump-json",
		"--simulate",
		"--no-warnings",
		"--no-playlist",
		"--cookies", cookiesPath,
		verifyTestURL,
	}
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	cmd.WaitDelay = killWaitDelay
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = io.Discard
	runErr := cmd.Run()
	se := stderr.String()

	if strings.Contains(se, "Cookies file must be Netscape formatted") {
		return VerifyResult{
			Loaded:        false,
			Authenticated: false,
			Detail:        "yt-dlp rejected the file: cookies must be in Netscape (cookies.txt) format. JSON exports are not supported. Use \"Get cookies.txt LOCALLY\" or a similar Netscape export.",
		}, nil
	}
	if runErr != nil {
		return VerifyResult{
			Loaded:        false,
			Authenticated: false,
			Detail:        fmt.Sprintf("yt-dlp probe failed: %v (stderr: %s)", runErr, stderrTail(&stderr)),
		}, nil
	}

	res := VerifyResult{Loaded: true}
	if strings.Contains(se, "Found YouTube account cookies") {
		res.Authenticated = true
		res.Detail = "Cookies loaded and YouTube recognised an authenticated session."
	} else {
		res.Detail = "Cookies loaded but YouTube treated the request as anonymous. The exported file may have been missing the LOGIN_INFO / SAPISID cookies."
	}
	if strings.Contains(se, "no longer valid") || strings.Contains(se, "have likely been rotated") {
		res.Rotated = true
		res.Authenticated = false
		res.Detail = "The cookies have expired or been rotated. Export a fresh cookies.txt from a signed-in browser session."
	}
	return res, nil
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
