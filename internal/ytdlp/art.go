package ytdlp

import (
	"fmt"
	"regexp"
	"strings"
)

var artSizeRe = regexp.MustCompile(`=w\d{2,4}-h\d{2,4}`)

func RewriteArtSize(url string, size int) (string, bool) {
	if !strings.Contains(url, ytMusicArtHost) {
		return "", false
	}
	if !artSizeRe.MatchString(url) {
		return "", false
	}
	replacement := fmt.Sprintf("=w%d-h%d", size, size)
	return artSizeRe.ReplaceAllString(url, replacement), true
}

func PickLargestArt(thumbs []Thumbnail) (Thumbnail, bool) {
	if len(thumbs) == 0 {
		return Thumbnail{}, false
	}
	best := thumbs[0]
	for _, t := range thumbs[1:] {
		if t.Width > best.Width {
			best = t
		}
	}
	return best, true
}

func NormalizeArt(info *Info, size int) (string, bool, bool) {
	if info.IsMusic() {
		largest, ok := PickLargestArt(info.MusicArtThumbnails())
		if !ok {
			return "", false, false
		}
		rewritten, ok := RewriteArtSize(largest.URL, size)
		if !ok {
			return largest.URL, true, true
		}
		return rewritten, true, true
	}
	if info.ID == "" {
		return "", false, false
	}
	return fmt.Sprintf("https://i.ytimg.com/vi/%s/maxresdefault.jpg", info.ID), false, true
}
