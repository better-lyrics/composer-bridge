package ytdlp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMpegTSFile_DetectsValidStream(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.opus")
	buf := make([]byte, 376)
	buf[0] = 0x47
	buf[188] = 0x47
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if !IsMpegTSFile(path) {
		t.Error("got false for two-packet MPEG-TS fixture, want true")
	}
}

func TestIsMpegTSFile_RejectsWebMHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.webm")
	buf := make([]byte, 256)
	// EBML header for a real WebM container.
	copy(buf, []byte{0x1A, 0x45, 0xDF, 0xA3})
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if IsMpegTSFile(path) {
		t.Error("got true for WebM EBML header, want false")
	}
}

func TestIsMpegTSFile_RejectsCoincidentalLeadingByte(t *testing.T) {
	// A file that starts with 0x47 but has nothing at offset 188 must not be
	// flagged: the second sync byte is what distinguishes a real TS stream
	// from any other format that happens to start with the same byte.
	dir := t.TempDir()
	path := filepath.Join(dir, "x.bin")
	buf := make([]byte, 256)
	buf[0] = 0x47
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if IsMpegTSFile(path) {
		t.Error("got true for single-0x47 fixture, want false")
	}
}

func TestIsMpegTSFile_ShortFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.bin")
	if err := os.WriteFile(path, []byte{0x47}, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if IsMpegTSFile(path) {
		t.Error("got true for 1-byte file, want false")
	}
}

func TestIsMpegTSFile_Missing(t *testing.T) {
	if IsMpegTSFile(filepath.Join(t.TempDir(), "nope")) {
		t.Error("got true for missing path, want false")
	}
}
