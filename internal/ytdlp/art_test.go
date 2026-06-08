package ytdlp

import "testing"

const sampleArtURL = "https://yt3.googleusercontent.com/GCWcP0PYxKzb0-l04KRh2th6zKS6HMPYAWGoc_3cZGO8NwIiIzvplCcXMF1NDNmWo1Ke3MpDonqddxWJ=w60-h60-l90-rj"

func TestRewriteArtSize_BasicResize(t *testing.T) {
	got, ok := RewriteArtSize(sampleArtURL, 544)
	if !ok {
		t.Fatal("RewriteArtSize: ok=false on valid yt3 URL")
	}
	want := "https://yt3.googleusercontent.com/GCWcP0PYxKzb0-l04KRh2th6zKS6HMPYAWGoc_3cZGO8NwIiIzvplCcXMF1NDNmWo1Ke3MpDonqddxWJ=w544-h544-l90-rj"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestRewriteArtSize_PreservesNonSizeParams(t *testing.T) {
	in := "https://yt3.googleusercontent.com/abc123=w60-h60-l90-rj-c0xffffff"
	got, ok := RewriteArtSize(in, 1024)
	if !ok {
		t.Fatal("ok=false")
	}
	want := "https://yt3.googleusercontent.com/abc123=w1024-h1024-l90-rj-c0xffffff"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestRewriteArtSize_AcceptsThreeOrFourDigitSizes(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://yt3.googleusercontent.com/x=w120-h120-l90-rj", "https://yt3.googleusercontent.com/x=w544-h544-l90-rj"},
		{"https://yt3.googleusercontent.com/x=w1024-h1024-l90-rj", "https://yt3.googleusercontent.com/x=w544-h544-l90-rj"},
	}
	for _, tt := range tests {
		got, ok := RewriteArtSize(tt.in, 544)
		if !ok || got != tt.want {
			t.Errorf("RewriteArtSize(%q): got %q ok=%v, want %q ok=true", tt.in, got, ok, tt.want)
		}
	}
}

func TestRewriteArtSize_RejectsNonYt3Hosts(t *testing.T) {
	cases := []string{
		"https://i.ytimg.com/vi/abc/maxresdefault.jpg",
		"https://example.com/=w60-h60",
		"not a url",
	}
	for _, c := range cases {
		if got, ok := RewriteArtSize(c, 544); ok {
			t.Errorf("RewriteArtSize(%q) should reject; got %q ok=true", c, got)
		}
	}
}

func TestRewriteArtSize_NoSizeMarker(t *testing.T) {
	in := "https://yt3.googleusercontent.com/abc123"
	if _, ok := RewriteArtSize(in, 544); ok {
		t.Errorf("RewriteArtSize(%q) should reject when no size marker; got ok=true", in)
	}
}

func TestPickLargestArt(t *testing.T) {
	info := loadFixture(t, "music_frank_sinatra.json")
	chosen, ok := PickLargestArt(info.MusicArtThumbnails())
	if !ok {
		t.Fatal("PickLargestArt: ok=false on music fixture")
	}
	if chosen.Width != 544 {
		t.Errorf("largest width: got %d, want 544", chosen.Width)
	}
}

func TestPickLargestArt_EmptyList(t *testing.T) {
	if _, ok := PickLargestArt(nil); ok {
		t.Error("PickLargestArt(nil) should return ok=false")
	}
}

func TestNormalizeArt_MusicTrackProducesHiResURL(t *testing.T) {
	info := loadFixture(t, "music_frank_sinatra.json")
	url, isMusic, ok := NormalizeArt(info, 1024)
	if !ok {
		t.Fatal("NormalizeArt: ok=false")
	}
	if !isMusic {
		t.Error("isMusic: got false, want true")
	}
	want := "https://yt3.googleusercontent.com/GCWcP0PYxKzb0-l04KRh2th6zKS6HMPYAWGoc_3cZGO8NwIiIzvplCcXMF1NDNmWo1Ke3MpDonqddxWJ=w1024-h1024-l90-rj"
	if url != want {
		t.Errorf("\n got: %s\nwant: %s", url, want)
	}
}

func TestNormalizeArt_RegularVideoFallsBackToYtimg(t *testing.T) {
	info := loadFixture(t, "video_me_at_zoo.json")
	url, isMusic, ok := NormalizeArt(info, 1024)
	if !ok {
		t.Fatal("NormalizeArt: ok=false on non-music video")
	}
	if isMusic {
		t.Error("isMusic: got true, want false")
	}
	want := "https://i.ytimg.com/vi/jNQXAC9IVRw/maxresdefault.jpg"
	if url != want {
		t.Errorf("\n got: %s\nwant: %s", url, want)
	}
}
