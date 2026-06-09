package tray

import (
	"context"
	"testing"

	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
)

func TestNewReturnsNonNilController(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestControllerStartsWithUnsetContext(t *testing.T) {
	c := New()
	if c.HasContext() {
		t.Error("Controller should not have a context before BindContext")
	}
}

func TestBindContextStoresTheContext(t *testing.T) {
	c := New()
	c.BindContext(context.Background())
	if !c.HasContext() {
		t.Error("HasContext should be true after BindContext with a real ctx")
	}
}

func TestContextReturnsBoundContext(t *testing.T) {
	c := New()
	parent := context.WithValue(context.Background(), ctxSentinel{}, "ok")
	c.BindContext(parent)
	got := c.Context()
	if got == nil {
		t.Fatal("Context() returned nil after BindContext")
	}
	if got.Value(ctxSentinel{}) != "ok" {
		t.Errorf("Context() did not preserve bound value")
	}
}

type ctxSentinel struct{}

func TestRenderStateTitle(t *testing.T) {
	cases := []struct {
		name string
		s    bridgestate.State
		want string
	}{
		{"idle", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadIdle}, "Idle"},
		{"downloading", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadActive, DownloadVideoID: "abc"}, "Downloading abc"},
		{"stopped", bridgestate.State{Server: bridgestate.ServerStopped}, "Server stopped"},
		{"starting", bridgestate.State{Server: bridgestate.ServerStarting}, "Starting..."},
		{"stopping", bridgestate.State{Server: bridgestate.ServerStopping}, "Stopping..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderStateTitle(tc.s); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
