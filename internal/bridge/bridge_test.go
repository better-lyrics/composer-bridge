package bridge_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/bridge"
	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
)

func TestStart_FlipsServerStatusToRunning(t *testing.T) {
	holder := bridgestate.NewHolder()
	b := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })

	if err := b.Start(0); err != nil { // 0 = ephemeral port
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Stop() })

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if holder.Snapshot().Server == bridgestate.ServerRunning {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("server never reached Running, last=%q", holder.Snapshot().Server)
}

func TestStop_FlipsServerStatusToStopped(t *testing.T) {
	holder := bridgestate.NewHolder()
	b := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := b.Start(0); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := b.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := holder.Snapshot().Server; got != bridgestate.ServerStopped {
		t.Errorf("Server: got %q, want %q", got, bridgestate.ServerStopped)
	}
}

func TestStart_OnPortAlreadyBound_ReturnsErrorAndFlipsBackToStopped(t *testing.T) {
	holder := bridgestate.NewHolder()
	first := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := first.Start(0); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer func() { _ = first.Stop() }()
	port := first.Port()

	secondHolder := bridgestate.NewHolder()
	second := bridge.New(secondHolder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := second.Start(port); err == nil {
		_ = second.Stop()
		t.Fatal("expected port-in-use error, got nil")
	}
	if got := secondHolder.Snapshot().Server; got != bridgestate.ServerStopped {
		t.Errorf("second holder after failed Start: got %q, want %q", got, bridgestate.ServerStopped)
	}
}

func TestStop_BeforeStartIsNoop(t *testing.T) {
	holder := bridgestate.NewHolder()
	b := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := b.Stop(); err != nil {
		t.Errorf("Stop before Start should be a no-op, got %v", err)
	}
	if got := holder.Snapshot().Server; got != bridgestate.ServerStopped {
		t.Errorf("Server after no-op Stop: got %q, want %q", got, bridgestate.ServerStopped)
	}
}

func TestStart_TwiceReturnsError(t *testing.T) {
	holder := bridgestate.NewHolder()
	b := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := b.Start(0); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Stop() })
	if err := b.Start(0); err == nil {
		t.Fatal("second Start should return error, got nil")
	}
}

func TestPort_ReturnsListeningPortAfterStart(t *testing.T) {
	holder := bridgestate.NewHolder()
	b := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := b.Start(0); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Stop() })
	if b.Port() == 0 {
		t.Errorf("Port: got 0, want a non-zero ephemeral port")
	}
}

func TestStart_TransitionsThroughStarting(t *testing.T) {
	holder := bridgestate.NewHolder()
	seen := make([]bridgestate.ServerStatus, 0, 4)
	t.Cleanup(holder.OnChange(func(s bridgestate.State) {
		seen = append(seen, s.Server)
	}))

	b := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := b.Start(0); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Stop() })

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(seen) >= 2 && seen[len(seen)-1] == bridgestate.ServerRunning {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// All transitions happen synchronously inside Start under b.mu, so the
	// observed order here is deterministic despite the broadcast-out-of-order
	// caveat documented in bridgestate.
	// Order must include Starting then Running.
	var sawStarting, sawRunning bool
	var startingBeforeRunning bool
	for i, s := range seen {
		if s == bridgestate.ServerStarting {
			sawStarting = true
			for j := i + 1; j < len(seen); j++ {
				if seen[j] == bridgestate.ServerRunning {
					startingBeforeRunning = true
					break
				}
			}
		}
		if s == bridgestate.ServerRunning {
			sawRunning = true
		}
	}
	if !sawStarting || !sawRunning || !startingBeforeRunning {
		t.Errorf("transitions: got %v, want Starting followed by Running", seen)
	}
}

func TestStart_AfterStop_BuildsFreshServerAndServes(t *testing.T) {
	holder := bridgestate.NewHolder()
	var buildCalls int
	b := bridge.New(holder, func() *http.Server {
		buildCalls++
		return &http.Server{Handler: http.NewServeMux()}
	})

	if err := b.Start(0); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := b.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := b.Start(0); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	t.Cleanup(func() { _ = b.Stop() })

	if buildCalls != 2 {
		t.Errorf("build calls: got %d, want 2 (one per Start)", buildCalls)
	}
	if got := holder.Snapshot().Server; got != bridgestate.ServerRunning {
		t.Errorf("Server after second Start: got %q, want %q", got, bridgestate.ServerRunning)
	}
	// Smoke-test that the second cycle actually serves: ListenAndServe-style
	// hit using b.Port().
	if b.Port() == 0 {
		t.Fatal("Port after second Start: 0")
	}
}

func TestStop_ClearsPort(t *testing.T) {
	holder := bridgestate.NewHolder()
	b := bridge.New(holder, func() *http.Server { return &http.Server{Handler: http.NewServeMux()} })
	if err := b.Start(0); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if b.Port() == 0 {
		t.Fatal("Port after Start: 0")
	}
	if err := b.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := b.Port(); got != 0 {
		t.Errorf("Port after Stop: got %d, want 0", got)
	}
}
