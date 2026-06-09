// Package bridgestate holds the single source of truth for the bridge's
// runtime state (HTTP server lifecycle, active download). All consumers
// (frontend events, tray controller, exposed Wails methods) read and
// mutate through a Holder so the state stays consistent across goroutines.
package bridgestate

import "sync"

// ServerStatus describes the HTTP server lifecycle.
type ServerStatus string

const (
	ServerStopped  ServerStatus = "stopped"
	ServerStarting ServerStatus = "starting"
	ServerRunning  ServerStatus = "running"
	ServerStopping ServerStatus = "stopping"
)

// DownloadStatus describes whether a yt-dlp download is in flight.
type DownloadStatus string

const (
	DownloadIdle   DownloadStatus = "idle"
	DownloadActive DownloadStatus = "active"
)

// State is the value-type snapshot of the bridge's runtime state.
// JSON tags match the shape emitted to the frontend.
type State struct {
	Server          ServerStatus   `json:"server"`
	Download        DownloadStatus `json:"download"`
	DownloadVideoID string         `json:"downloadVideoId"`
	LastError       string         `json:"lastError"`
}

// Holder guards a State with an RWMutex so reads and writes can happen
// from multiple goroutines safely.
type Holder struct {
	mu    sync.RWMutex
	state State
}

// NewHolder returns a Holder initialised with the default state:
// server stopped, no active download, no error.
func NewHolder() *Holder {
	return &Holder{
		state: State{
			Server:   ServerStopped,
			Download: DownloadIdle,
		},
	}
}

// Snapshot returns a value copy of the current state under the read lock.
func (h *Holder) Snapshot() State {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.state
}

// SetServer swaps in a new server status and returns the previous one.
func (h *Holder) SetServer(s ServerStatus) ServerStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	prev := h.state.Server
	h.state.Server = s
	return prev
}

// StartDownload marks a download as active and records its video ID.
func (h *Holder) StartDownload(videoID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Download = DownloadActive
	h.state.DownloadVideoID = videoID
}

// EndDownload resets the download to idle and clears the video ID. When
// errMsg is non-empty it is stored as the most recent error so callers
// can surface it in the UI.
func (h *Holder) EndDownload(errMsg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Download = DownloadIdle
	h.state.DownloadVideoID = ""
	if errMsg != "" {
		h.state.LastError = errMsg
	}
}
