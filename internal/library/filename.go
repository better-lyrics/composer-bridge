package library

import "strings"

// AudioFilename returns the canonical on-disk name for a track's audio file:
// "<sanitized title> [<videoID>].<ext>". The videoID disambiguates titles
// that would otherwise collide after sanitization, so two different videos
// that happen to share a title never overwrite each other.
func AudioFilename(title, videoID, ext string) string {
	return sanitizeTrackTitle(title) + " [" + videoID + "]." + ext
}

// sanitizeTrackTitle strips characters that would break a filename on any
// supported OS and rune-truncates to a sensible length, so multi-byte titles
// (CJK, emoji) never split a rune and yield invalid-UTF-8 filenames that
// some filesystems (notably APFS) reject at the syscall layer.
func sanitizeTrackTitle(name string) string {
	if name == "" {
		return "track"
	}
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	out := repl.Replace(name)
	runes := []rune(out)
	if len(runes) > 120 {
		out = string(runes[:120])
	}
	return out
}
