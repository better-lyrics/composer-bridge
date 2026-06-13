package ytdlp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"time"
)

// VideoIDRe matches the canonical 11-character YouTube video ID.
var VideoIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`)

const (
	stderrTailLimit = 500
	// killWaitDelay bounds the time we wait for orphaned child processes
	// (e.g. ffmpeg) holding inherited stdout/stderr pipes after the parent
	// is killed by context cancellation.
	killWaitDelay = 250 * time.Millisecond
)

func videoURL(videoID string) string {
	return "https://www.youtube.com/watch?v=" + videoID
}

func validateVideoID(videoID string) error {
	if !VideoIDRe.MatchString(videoID) {
		return fmt.Errorf("invalid videoID %q", videoID)
	}
	return nil
}

func stderrTail(buf *bytes.Buffer) string {
	b := buf.Bytes()
	if len(b) <= stderrTailLimit {
		return string(b)
	}
	return string(b[len(b)-stderrTailLimit:])
}

func wrapRunError(verb, videoID string, runErr error, stderr *bytes.Buffer) error {
	return fmt.Errorf("yt-dlp %s %s: %w (stderr: %s)", verb, videoID, runErr, stderrTail(stderr))
}

// FetchInfo runs `yt-dlp -j` for the given videoID and parses the JSON output.
// Rejects malformed video IDs before forking. Honors ctx cancellation. An empty
// cookiesPath omits the --cookies flag. When preferPremium is true, the
// extractor-args chain tries YouTube Music's higher quality tier first.
func FetchInfo(ctx context.Context, ytdlpPath, videoID, cookiesPath string, preferPremium bool) (*Info, error) {
	if err := validateVideoID(videoID); err != nil {
		return nil, err
	}
	args := []string{
		"-j",
		"--skip-download",
		"--no-warnings",
		"--no-playlist",
		"--extractor-args", BuildExtractorArgs(preferPremium),
	}
	if cookiesPath != "" {
		args = append(args, "--cookies", cookiesPath)
	}
	args = append(args, videoURL(videoID))
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	cmd.WaitDelay = killWaitDelay
	cmd.Env = execEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, wrapRunError("info", videoID, err, &stderr)
	}
	info, err := Parse(stdout.Bytes())
	if err != nil {
		return nil, fmt.Errorf("yt-dlp info %s: %w", videoID, err)
	}
	return info, nil
}

// StreamAudio runs yt-dlp for the given videoID using the format selector chosen
// by FormatSelector(format) and copies its stdout into w. Rejects malformed video
// IDs before forking. Honors ctx cancellation. An empty format defaults to opus.
// An empty cookiesPath omits the --cookies flag. When preferPremium is true, the
// extractor-args chain tries YouTube Music's higher quality tier first.
func StreamAudio(ctx context.Context, ytdlpPath, videoID, format, cookiesPath string, preferPremium bool, w io.Writer) error {
	if err := validateVideoID(videoID); err != nil {
		return err
	}
	args := []string{
		"-f", FormatSelector(format),
		"-o", "-",
		"--quiet",
		"--no-warnings",
		"--no-playlist",
		"--extractor-args", BuildExtractorArgs(preferPremium),
	}
	if cookiesPath != "" {
		args = append(args, "--cookies", cookiesPath)
	}
	args = append(args, videoURL(videoID))
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	cmd.WaitDelay = killWaitDelay
	cmd.Env = execEnv()
	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return wrapRunError("audio", videoID, err, &stderr)
	}
	return nil
}
