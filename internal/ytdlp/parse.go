package ytdlp

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

type Info struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Track       string      `json:"track,omitempty"`
	Artist      string      `json:"artist,omitempty"`
	Album       string      `json:"album,omitempty"`
	ReleaseYear int         `json:"release_year,omitempty"`
	Duration    int         `json:"duration"`
	Thumbnails  []Thumbnail `json:"thumbnails"`
	WebpageURL  string      `json:"webpage_url"`
	UploaderID  string      `json:"uploader_id,omitempty"`
}

func Parse(raw []byte) (*Info, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("parse yt-dlp info: empty input")
	}
	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("parse yt-dlp info: %w", err)
	}
	if info.ID == "" {
		return nil, fmt.Errorf("parse yt-dlp info: missing id")
	}
	return &info, nil
}

const ytMusicArtHost = "yt3.googleusercontent.com"

func (i *Info) IsMusic() bool {
	for _, t := range i.Thumbnails {
		if strings.Contains(t.URL, ytMusicArtHost) {
			return true
		}
	}
	return false
}

func (i *Info) MusicArtThumbnails() []Thumbnail {
	out := make([]Thumbnail, 0, len(i.Thumbnails))
	for _, t := range i.Thumbnails {
		if strings.Contains(t.URL, ytMusicArtHost) {
			out = append(out, t)
		}
	}
	return out
}
