package ytdlp

import (
	"os"
	"runtime"
	"strings"
	"sync/atomic"
)

// denoBinDir holds the dir containing the bundled deno binary, prepended to
// PATH for every yt-dlp child invocation. Set once at startup via
// SetDenoBinDir. Empty disables augmentation. Atomic.Pointer lets us update
// it after EnsureDeno completes without locking the hot path.
var denoBinDir atomic.Pointer[string]

// SetDenoBinDir records the dir where EnsureDeno installed the bundled deno
// binary. Subsequent yt-dlp invocations get PATH augmented with this dir so
// the youtube extractor can find deno when solving n-sig JS challenges.
// Passing "" disables the augmentation (used by tests that don't want a
// global modified). Safe to call from any goroutine.
func SetDenoBinDir(dir string) {
	d := dir
	denoBinDir.Store(&d)
}

// execEnv is the env yt-dlp child processes run with. It starts from the
// parent process env and prepends the bundled deno dir plus, on macOS, the
// standard Homebrew bin dirs to PATH.
//
// Why this matters: macOS apps launched by launchd (Finder, Dock, login
// item) inherit a minimal PATH that omits /opt/homebrew/bin. yt-dlp's
// youtube extractor needs an external JS engine (deno, node) to solve n-sig
// challenges for the web_safari client. Without one on PATH, n-sig solve
// silently fails, the format URLs are unusable, and yt-dlp reports
// "Requested format is not available" with zero formats extracted. Bundled
// deno under <dataDir>/bin is the primary mechanism; the Homebrew dirs are a
// fallback so users who have installed deno themselves still get the right
// binary even when EnsureDeno hasn't finished or failed.
func execEnv() []string {
	var extras []string
	if p := denoBinDir.Load(); p != nil && *p != "" {
		extras = append(extras, *p)
	}
	if runtime.GOOS == "darwin" {
		extras = append(extras, "/opt/homebrew/bin", "/opt/local/bin")
	}
	return augmentPATH(os.Environ(), extras)
}

// augmentPATH returns env with extras prepended to PATH, preserving order and
// skipping entries already present. Pure function; takes env explicitly so
// tests don't have to mutate the process env. Empty extras returns env
// unchanged. If PATH is missing entirely, a new PATH var is appended.
func augmentPATH(env, extras []string) []string {
	if len(extras) == 0 {
		return env
	}
	sep := string(os.PathListSeparator)
	var path string
	pathIdx := -1
	for i, kv := range env {
		if p, ok := strings.CutPrefix(kv, "PATH="); ok {
			path = p
			pathIdx = i
			break
		}
	}
	seen := make(map[string]bool, strings.Count(path, sep)+1)
	for p := range strings.SplitSeq(path, sep) {
		if p != "" {
			seen[p] = true
		}
	}
	var prepend []string
	for _, p := range extras {
		if p == "" || seen[p] {
			continue
		}
		prepend = append(prepend, p)
		seen[p] = true
	}
	if len(prepend) == 0 {
		return env
	}
	newPath := strings.Join(prepend, sep)
	if path != "" {
		newPath = newPath + sep + path
	}
	out := make([]string, len(env), len(env)+1)
	copy(out, env)
	if pathIdx >= 0 {
		out[pathIdx] = "PATH=" + newPath
		return out
	}
	return append(out, "PATH="+newPath)
}
