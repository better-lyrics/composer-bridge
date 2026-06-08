package ytdlp

import "testing"

func TestFormatSelector_KnownKeys(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"opus", "bestaudio[acodec=opus]/bestaudio[ext=webm]/bestaudio/best"},
		{"", "bestaudio[acodec=opus]/bestaudio[ext=webm]/bestaudio/best"},
		{"m4a", "bestaudio[ext=m4a]/bestaudio/best"},
		{"webm", "bestaudio[ext=webm]/bestaudio/best"},
		{"mp3", "bestaudio/best"},
		{"garbage", "bestaudio[acodec=opus]/bestaudio[ext=webm]/bestaudio/best"},
	}
	for _, tt := range tests {
		if got := FormatSelector(tt.format); got != tt.want {
			t.Errorf("FormatSelector(%q): got %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestFormatExtension_KnownKeys(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"opus", "opus"},
		{"", "opus"},
		{"m4a", "m4a"},
		{"webm", "webm"},
		{"mp3", "mp3"},
		{"garbage", "opus"},
	}
	for _, tt := range tests {
		if got := FormatExtension(tt.format); got != tt.want {
			t.Errorf("FormatExtension(%q): got %q, want %q", tt.format, got, tt.want)
		}
	}
}
