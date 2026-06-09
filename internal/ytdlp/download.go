package ytdlp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// FormatSelector maps a user-facing format key (opus / m4a / webm / mp3) to the
// yt-dlp -f expression we hand to the subprocess. Every selector ends in
// `bestaudio/best` so a video with no preferred codec still falls back to
// whatever audio (or audio+video) yt-dlp can produce. Unknown keys behave like
// opus.
func FormatSelector(format string) string {
	switch format {
	case "m4a":
		return "bestaudio[ext=m4a]/bestaudio/best"
	case "webm":
		return "bestaudio[ext=webm]/bestaudio/best"
	case "mp3":
		return "bestaudio/best"
	case "opus", "":
		return "bestaudio[acodec=opus]/bestaudio[ext=webm]/bestaudio/best"
	default:
		return "bestaudio[acodec=opus]/bestaudio[ext=webm]/bestaudio/best"
	}
}

// FormatExtension is the file extension yt-dlp will produce for a given format key.
func FormatExtension(format string) string {
	switch format {
	case "m4a":
		return "m4a"
	case "webm":
		return "webm"
	case "mp3":
		return "mp3"
	default:
		return "opus"
	}
}

// DownloadToFile runs yt-dlp to fetch audio for videoID using the chosen format
// and writes it to destPath. Returns the resulting file size in bytes. Honors
// ctx cancellation and rejects malformed video IDs before forking. An empty
// cookiesPath omits the --cookies flag.
func DownloadToFile(ctx context.Context, ytdlpPath, videoID, format, destPath, cookiesPath string) (int64, error) {
	if err := validateVideoID(videoID); err != nil {
		return 0, err
	}
	args := []string{
		"-f", FormatSelector(format),
		"-o", destPath,
		"--no-warnings",
		"--no-playlist",
		"--force-overwrites",
		"--extractor-args", "youtube:player_client=android_vr,web_safari;player_skip=configs,initial_data",
	}
	if cookiesPath != "" {
		args = append(args, "--cookies", cookiesPath)
	}
	args = append(args, videoURL(videoID))
	if format == "mp3" {
		args = append([]string{"--extract-audio", "--audio-format", "mp3"}, args...)
	}
	cmd := exec.CommandContext(ctx, ytdlpPath, args...)
	cmd.WaitDelay = killWaitDelay
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("yt-dlp download %s: %w (stderr: %s)", videoID, err, stderrTail(&stderr))
	}
	stat, err := os.Stat(destPath)
	if err != nil {
		return 0, fmt.Errorf("stat downloaded file: %w", err)
	}
	return stat.Size(), nil
}
