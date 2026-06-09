package ytdlp

import (
	"errors"
	"fmt"
	"os"
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
