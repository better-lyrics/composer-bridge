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

func TestSetUpdatePending_FlipsField(t *testing.T) {
	h := NewHolder()
	h.SetUpdatePending(true)
	if !h.Snapshot().UpdatePending {
		t.Error("UpdatePending: got false, want true after SetUpdatePending(true)")
	}
	h.SetUpdatePending(false)
	if h.Snapshot().UpdatePending {
		t.Error("UpdatePending: got true, want false after SetUpdatePending(false)")
	}
}

func TestSetUpdatePending_NoopWhenUnchanged(t *testing.T) {
	h := NewHolder()
	var calls int
	t.Cleanup(h.OnChange(func(_ State) { calls++ }))
	h.SetUpdatePending(false)
	if calls != 0 {
		t.Errorf("subscriber fired on no-op set: calls=%d", calls)
	}
	h.SetUpdatePending(true)
	h.SetUpdatePending(true)
	if calls != 1 {
		t.Errorf("subscriber fired on idempotent re-set: calls=%d, want 1", calls)
	}
}

func TestOnChange_FiresAfterStartDownload(t *testing.T) {
	h := NewHolder()
	got := make(chan State, 1)
	unsub := h.OnChange(func(s State) { got <- s })
	t.Cleanup(unsub)

	h.StartDownload("vid")

	select {
	case s := <-got:
		if s.Download != DownloadActive || s.DownloadVideoID != "vid" {
			t.Errorf("emitted state: %+v", s)
		}
	default:
		t.Fatal("subscriber was not called")
	}
}

func TestOnChange_FiresAfterSetServer(t *testing.T) {
	h := NewHolder()
	var calls int
	unsub := h.OnChange(func(_ State) { calls++ })
	t.Cleanup(unsub)
	h.SetServer(ServerRunning)
	if calls != 1 {
		t.Errorf("calls: got %d, want 1", calls)
	}
}

func TestOnChange_UnsubscribeStopsFurtherCalls(t *testing.T) {
	h := NewHolder()
	var calls int
	unsub := h.OnChange(func(_ State) { calls++ })
	unsub()
	h.SetServer(ServerRunning)
	if calls != 0 {
		t.Errorf("calls after unsub: got %d, want 0", calls)
	}
}

func TestOnChange_MultipleSubscribersAllFire(t *testing.T) {
	h := NewHolder()
	var a, b int
	t.Cleanup(h.OnChange(func(_ State) { a++ }))
	t.Cleanup(h.OnChange(func(_ State) { b++ }))
	h.SetServer(ServerRunning)
	if a != 1 || b != 1 {
		t.Errorf("a=%d b=%d, want 1 1", a, b)
	}
}

func TestOnChange_SubscriberCanCallHolderMethodsWithoutDeadlock(t *testing.T) {
	h := NewHolder()
	var snap State
	t.Cleanup(h.OnChange(func(_ State) {
		snap = h.Snapshot()
	}))
	h.SetServer(ServerRunning)
	if snap.Server != ServerRunning {
		t.Errorf("re-entrant Snapshot inside subscriber: got %q, want %q", snap.Server, ServerRunning)
	}
}

func TestHolder_ConcurrentMutationsAreRaceFree(t *testing.T) {
	h := NewHolder()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			h.SetServer(ServerRunning)
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		h.StartDownload("v")
		h.EndDownload("")
		_ = h.Snapshot()
	}
	<-done
}
