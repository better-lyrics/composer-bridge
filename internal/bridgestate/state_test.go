package bridgestate

import (
	"testing"
)

func TestNewHolder_DefaultsToServerStoppedAndIdle(t *testing.T) {
	h := NewHolder()
	got := h.Snapshot()
	if got.Server != ServerStopped {
		t.Errorf("Server: got %q, want %q", got.Server, ServerStopped)
	}
	if got.Download != DownloadIdle {
		t.Errorf("Download: got %q, want %q", got.Download, DownloadIdle)
	}
	if got.DownloadVideoID != "" {
		t.Errorf("DownloadVideoID: got %q, want empty", got.DownloadVideoID)
	}
	if got.LastError != "" {
		t.Errorf("LastError: got %q, want empty", got.LastError)
	}
}

func TestSetServer_StoresAndReturnsPrevious(t *testing.T) {
	h := NewHolder()
	prev := h.SetServer(ServerRunning)
	if prev != ServerStopped {
		t.Errorf("returned previous: got %q, want %q", prev, ServerStopped)
	}
	if got := h.Snapshot().Server; got != ServerRunning {
		t.Errorf("Server after set: got %q, want %q", got, ServerRunning)
	}
}

func TestStartDownload_FlipsToDownloadingAndStoresVideoID(t *testing.T) {
	h := NewHolder()
	h.StartDownload("dQw4w9WgXcQ")
	s := h.Snapshot()
	if s.Download != DownloadActive {
		t.Errorf("Download: got %q, want %q", s.Download, DownloadActive)
	}
	if s.DownloadVideoID != "dQw4w9WgXcQ" {
		t.Errorf("DownloadVideoID: got %q, want dQw4w9WgXcQ", s.DownloadVideoID)
	}
}

func TestEndDownload_ResetsToIdleAndClearsVideoID(t *testing.T) {
	h := NewHolder()
	h.StartDownload("vid")
	h.EndDownload("")
	s := h.Snapshot()
	if s.Download != DownloadIdle {
		t.Errorf("Download: got %q, want %q", s.Download, DownloadIdle)
	}
	if s.DownloadVideoID != "" {
		t.Errorf("DownloadVideoID: got %q, want empty", s.DownloadVideoID)
	}
	if s.LastError != "" {
		t.Errorf("LastError: got %q, want empty", s.LastError)
	}
}

func TestEndDownload_WithErrorMessageStoresIt(t *testing.T) {
	h := NewHolder()
	h.StartDownload("vid")
	h.EndDownload("yt-dlp exit 1")
	s := h.Snapshot()
	if s.LastError != "yt-dlp exit 1" {
		t.Errorf("LastError: got %q, want yt-dlp exit 1", s.LastError)
	}
}
