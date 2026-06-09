package tray

import (
	"bytes"
	"context"
	"testing"

	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
	"github.com/better-lyrics/composer-bridge/tray/icons"
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

func TestPickTrayIcon_PicksVariantPerState(t *testing.T) {
	cases := []struct {
		name string
		s    bridgestate.State
		want []byte
		mac  bool
	}{
		{"mac stopped", bridgestate.State{Server: bridgestate.ServerStopped}, icons.MacStopped, true},
		{"mac running idle", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadIdle}, icons.MacIdle, true},
		{"mac running downloading", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadActive, DownloadVideoID: "abc"}, icons.MacDownloading, true},
		{"mac running with last error", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadIdle, LastError: "yt-dlp exit 1"}, icons.MacError, true},
		{"mac starting (dim)", bridgestate.State{Server: bridgestate.ServerStarting}, icons.MacStopped, true},
		{"mac stopping (dim)", bridgestate.State{Server: bridgestate.ServerStopping}, icons.MacStopped, true},
		{"mac downloading wins over last error", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadActive, LastError: "old err"}, icons.MacDownloading, true},
		{"default stopped", bridgestate.State{Server: bridgestate.ServerStopped}, icons.DefaultStopped, false},
		{"default running idle", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadIdle}, icons.DefaultIdle, false},
		{"default downloading", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadActive}, icons.DefaultDownloading, false},
		{"default running with last error", bridgestate.State{Server: bridgestate.ServerRunning, Download: bridgestate.DownloadIdle, LastError: "yt-dlp exit 1"}, icons.DefaultError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, reg := pickTrayIcon(tc.s, tc.mac)
			got := reg
			if tc.mac {
				got = tmpl
			}
			if !bytes.Equal(got, tc.want) {
				t.Errorf("variant mismatch for %s", tc.name)
			}
		})
	}
}

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
