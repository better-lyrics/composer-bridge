package library

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAudioFilename_BasicShape(t *testing.T) {
	got := AudioFilename("Hello", "abc123", "opus")
	if got != "Hello [abc123].opus" {
		t.Fatalf("AudioFilename basic shape: %q", got)
	}
}

func TestAudioFilename_DistinctVideoIDsProduceDistinctNames(t *testing.T) {
	a := AudioFilename("Intro", "vid_A", "opus")
	b := AudioFilename("Intro", "vid_B", "opus")
	if a == b {
		t.Fatalf("same-title different-id should not collide: %q == %q", a, b)
	}
}

func TestAudioFilename_StripsReservedCharacters(t *testing.T) {
	got := AudioFilename(`a/b\c:d*e?f"g<h>i|j`, "id", "opus")
	for _, ch := range []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"} {
		if strings.Contains(got, ch) {
			t.Fatalf("filename still contains reserved char %q: %q", ch, got)
		}
	}
}

func TestAudioFilename_EmptyTitleFallsBack(t *testing.T) {
	got := AudioFilename("", "id", "opus")
	if got != "track [id].opus" {
		t.Fatalf("empty title fallback: %q", got)
	}
}

func TestAudioFilename_LongMultiByteTitleStaysValidUTF8(t *testing.T) {
	// 200 CJK runes; each is 3 bytes in UTF-8, so a byte-slice at 120
	// would land mid-rune and yield invalid UTF-8.
	title := strings.Repeat("中", 200)
	got := AudioFilename(title, "id", "opus")
	if !utf8.ValidString(got) {
		t.Fatalf("result not valid UTF-8: %q", got)
	}
	// 120 runes from the title + " [id].opus" suffix.
	wantRunes := 120 + utf8.RuneCountInString(" [id].opus")
	if gotRunes := utf8.RuneCountInString(got); gotRunes != wantRunes {
		t.Fatalf("rune count = %d, want %d", gotRunes, wantRunes)
	}
}

func TestAudioFilename_LongEmojiTitleStaysValidUTF8(t *testing.T) {
	// 4-byte runes hit the boundary even more often.
	title := strings.Repeat("🎵", 200)
	got := AudioFilename(title, "id", "opus")
	if !utf8.ValidString(got) {
		t.Fatalf("result not valid UTF-8: %q", got)
	}
}

func TestAudioFilename_ShortTitleIsNotTruncated(t *testing.T) {
	got := AudioFilename("Short Title", "id", "opus")
	if got != "Short Title [id].opus" {
		t.Fatalf("short title was modified: %q", got)
	}
}

func TestAudioFilename_TitleContainingBracketsStillDisambiguates(t *testing.T) {
	// Titles can legitimately contain "[...]" segments (remix tags, year
	// markers). videoIDs are validated to YouTube's [A-Za-z0-9_-]{11} alphabet
	// upstream, so they can never contain brackets themselves; the trailing
	// "[<videoID>]" suffix is therefore always unambiguous and same-id same-title
	// inputs cannot accidentally produce the same name as a different-id track.
	a := AudioFilename("Song [Remix]", "vidAAAAAAAA", "opus")
	b := AudioFilename("Song", "vidBBBBBBBB", "opus")
	if a == b {
		t.Fatalf("bracketed title collided with plain title under different ids: %q", a)
	}
	if want := "Song [Remix] [vidAAAAAAAA].opus"; a != want {
		t.Fatalf("bracketed title: got %q, want %q", a, want)
	}
}
