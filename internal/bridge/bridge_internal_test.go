package bridge

import (
	"net/http"
	"strings"
	"testing"

	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
)

// TestStart_ReturnsStopInProgressErrorWhenStoppingFlagSet locks in the
// stop-in-progress guard deterministically. The flag is unexported; we set
// it directly under the same mutex Stop uses, then assert Start refuses.
// The previous goroutine-race version of this test was timing-sensitive
// and flaked on fast CI runners where Stop completed before the loop ran.
func TestStart_ReturnsStopInProgressErrorWhenStoppingFlagSet(t *testing.T) {
	holder := bridgestate.NewHolder()
	b := New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })

	b.mu.Lock()
	b.stopping = true
	b.mu.Unlock()

	err := b.Start(0)
	if err == nil {
		t.Fatal("Start with stopping=true: got nil, want error")
	}
	if !strings.Contains(err.Error(), "stop in progress") {
		t.Errorf("Start error: got %q, want substring 'stop in progress'", err.Error())
	}
}
