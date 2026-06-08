package ytdlp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseInfo_MusicTrack(t *testing.T) {
	info := loadFixture(t, "music_frank_sinatra.json")

	if info.ID != "ZEcqHA7dbwM" {
		t.Errorf("ID: got %q, want ZEcqHA7dbwM", info.ID)
	}
	if info.Title != "Fly Me To The Moon (2008 Remastered)" {
		t.Errorf("Title: got %q", info.Title)
	}
	if info.Track != "Fly Me To The Moon (2008 Remastered)" {
		t.Errorf("Track: got %q", info.Track)
	}
	if info.Artist != "Frank Sinatra, Count Basie And His Orchestra" {
		t.Errorf("Artist: got %q", info.Artist)
	}
	if info.Album != "Nothing But The Best" {
		t.Errorf("Album: got %q", info.Album)
	}
	if info.ReleaseYear != 2008 {
		t.Errorf("ReleaseYear: got %d, want 2008", info.ReleaseYear)
	}
	if info.Duration < 100 {
		t.Errorf("Duration: got %d, want >100 seconds", info.Duration)
	}
	if !info.IsMusic() {
		t.Error("IsMusic: got false, want true (yt3 thumbs present)")
	}
}

func TestParseInfo_RegularVideo_SeeYouAgain(t *testing.T) {
	info := loadFixture(t, "video_see_you_again.json")

	if info.ID != "RgKAFK5djSk" {
		t.Errorf("ID: got %q", info.ID)
	}
	if info.Track != "" {
		t.Errorf("Track: got %q, want empty for non-topic upload", info.Track)
	}
	if info.IsMusic() {
		t.Error("IsMusic: got true, want false (no yt3 thumbs)")
	}
}

func TestParseInfo_RegularVideo_NonMusic(t *testing.T) {
	info := loadFixture(t, "video_me_at_zoo.json")

	if info.ID != "jNQXAC9IVRw" {
		t.Errorf("ID: got %q", info.ID)
	}
	if info.IsMusic() {
		t.Error("IsMusic: got true, want false")
	}
	if info.Duration <= 0 {
		t.Errorf("Duration: got %d, want positive", info.Duration)
	}
}

func TestParseInfo_NullableFields(t *testing.T) {
	info := loadFixture(t, "video_me_at_zoo.json")

	if info.Track != "" {
		t.Errorf("Track on non-topic video: got %q, want empty", info.Track)
	}
	if info.Artist != "" {
		t.Errorf("Artist on non-topic video: got %q, want empty", info.Artist)
	}
	if info.ReleaseYear != 0 {
		t.Errorf("ReleaseYear on non-topic video: got %d, want 0", info.ReleaseYear)
	}
}

func TestParseInfo_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte("not json"))
	if err == nil {
		t.Fatal("Parse(garbage): got nil error, want failure")
	}
}

func TestParseInfo_EmptyBytes(t *testing.T) {
	_, err := Parse([]byte(""))
	if err == nil {
		t.Fatal("Parse(empty): got nil error, want failure")
	}
}

func loadFixture(t *testing.T, name string) *Info {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	info, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return info
}
