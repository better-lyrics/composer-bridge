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
//
// The opus chain is the wide one because the bridge's player_client config
// (web_music, android_vr, web_safari) frequently does not expose itag 251
// (WebM-Opus); the only audio-bearing formats those clients return are HLS
// m3u8 (MPEG-TS) plus a single non-HLS mp4 with bundled AAC. So:
//
//  1. itag 251 if available (WebM-Opus, smallest playable)
//  2. any audio-only WebM (covers itag 249/250 / opus-low)
//  3. any audio-only non-HLS format (avoids the MPEG-TS trap)
//  4. any non-HLS format at all (catches videos where only a combined
//     mp4 is exposed; the browser ignores the video track and plays the
//     bundled AAC just fine in an <audio> element)
//  5. bestaudio (last resort: even HLS audio is better than nothing)
//  6. best (would be a combined HLS format; effectively a no-go but
//     keeps the selector total)
//
// MPEG-TS-over-HLS is intentionally pushed to the last two rungs because no
// browser <audio> element can decode video/mp2t natively, so saving it to
// disk would just reproduce the v1.4.x cache-broken state.
func FormatSelector(format string) string {
	switch format {
	case "m4a":
		return "bestaudio[ext=m4a]/bestaudio/best"
	case "webm":
		return "bestaudio[ext=webm]/bestaudio/best"
	case "mp3":
		return "bestaudio/best"
	case "opus", "":
		return "bestaudio[acodec=opus][ext=webm]/bestaudio[ext=webm]/bestaudio[protocol!*=m3u8]/best[protocol!*=m3u8]/bestaudio/best"
	default:
		return "bestaudio[acodec=opus][ext=webm]/bestaudio[ext=webm]/bestaudio[protocol!*=m3u8]/best[protocol!*=m3u8]/bestaudio/best"
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
// cookiesPath omits the --cookies flag. When preferPremium is true, the
// extractor-args chain tries YouTube Music's higher quality tier first.
func DownloadToFile(ctx context.Context, ytdlpPath, videoID, format, destPath, cookiesPath string, preferPremium bool) (int64, error) {
	if err := validateVideoID(videoID); err != nil {
		return 0, err
	}
	args := []string{
		"-f", FormatSelector(format),
		"-o", destPath,
		"--no-warnings",
		"--no-playlist",
		"--force-overwrites",
		"--extractor-args", BuildExtractorArgs(preferPremium),
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
	cmd.Env = execEnv()
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
