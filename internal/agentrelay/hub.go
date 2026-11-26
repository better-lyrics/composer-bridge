// Package agentrelay lets MCP clients (Claude Code, Codex, ...) reach the
// projects open in Composer browser tabs. Composer tabs that opted in hold a
// server-sent event stream to the bridge and advertise their tools; the MCP
// endpoint forwards each tool call to the tab showing the requested project
// and relays the tab's answer. The bridge never edits a project itself, so
// every change lands in Composer through its own undo history.
package agentrelay

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultCallTimeout bounds how long a tool call waits for the tab.
	DefaultCallTimeout = 60 * time.Second
	maxSessions        = 32
	sessionEventBuffer = 16
)

// ErrUnknownSession is returned for a state or result post whose session id
// and token don't match a connected tab.
var ErrUnknownSession = errors.New("agentrelay: unknown session")

// RoutingError explains to the agent why a call could not reach a tab. It is
// reported as a tool error, never as a protocol error, so the agent can act on it.
type RoutingError struct{ Message string }

func (e *RoutingError) Error() string { return e.Message }

// Project is what a tab reports about the project it has open.
type Project struct {
	ProjectID string   `json:"projectId"`
	Title     string   `json:"title"`
	Artists   []string `json:"artists"`
	Language  *string  `json:"language"`
	LineCount int      `json:"lineCount"`
}

// State is a tab's periodic report: its tools, its open project (nil when
// none) and whether the user is looking at it.
type State struct {
	Tools   json.RawMessage `json:"tools"`
	Project *Project        `json:"project"`
	Focused bool            `json:"focused"`
}

// Event is one server-sent event for a tab.
type Event struct {
	Name string
	Data any
}

// Session is one connected Composer tab.
type Session struct {
	ID    string
	token string

	events chan Event
	done   chan struct{}

	// Guarded by Hub.mu.
	project    *Project
	lastActive time.Time
	pending    map[string]chan json.RawMessage
	closed     bool
}

// Events delivers the stream for this tab. It is closed never; watch Done.
func (s *Session) Events() <-chan Event { return s.events }

// Done closes when the session is disconnected.
func (s *Session) Done() <-chan struct{} { return s.done }

// Token authenticates the tab's posts. Only the tab's own stream sees it.
func (s *Session) Token() string { return s.token }

// Hub tracks connected tabs and routes tool calls between MCP and tabs.
type Hub struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	tools       json.RawMessage
	cachePath   string
	callSeq     uint64
	now         func() time.Time
	CallTimeout time.Duration
}

// NewHub returns a Hub that remembers the last advertised tool list in
// cachePath, so an MCP client that connects before any tab still sees
// Composer's tools. An empty cachePath disables the cache.
func NewHub(cachePath string) *Hub {
	h := &Hub{
		sessions:    map[string]*Session{},
		cachePath:   cachePath,
		now:         time.Now,
		CallTimeout: DefaultCallTimeout,
	}
	if cachePath != "" {
		if data, err := os.ReadFile(cachePath); err == nil && json.Valid(data) {
			h.tools = data
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("agent tool cache unreadable", "path", cachePath, "err", err)
		}
	}
	return h
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("agentrelay: crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// Connect registers a new tab session.
func (h *Hub) Connect() (*Session, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.sessions) >= maxSessions {
		return nil, errors.New("agentrelay: too many Composer tabs connected")
	}
	s := &Session{
		ID:         randomHex(12),
		token:      randomHex(24),
		events:     make(chan Event, sessionEventBuffer),
		done:       make(chan struct{}),
		lastActive: h.now(),
		pending:    map[string]chan json.RawMessage{},
	}
	h.sessions[s.ID] = s
	return s, nil
}

// Disconnect removes a session and fails its pending calls.
func (h *Hub) Disconnect(s *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closeLocked(s)
}

func (h *Hub) closeLocked(s *Session) {
	if s.closed {
		return
	}
	s.closed = true
	delete(h.sessions, s.ID)
	close(s.done)
}

// CloseAll disconnects every tab, so their streams end and the HTTP server
// can shut down.
func (h *Hub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.sessions {
		h.closeLocked(s)
	}
}

func (h *Hub) authenticated(id, token string) (*Session, bool) {
	s, ok := h.sessions[id]
	if !ok || subtle.ConstantTimeCompare([]byte(s.token), []byte(token)) != 1 {
		return nil, false
	}
	return s, true
}

// UpdateState records a tab's report.
func (h *Hub) UpdateState(id, token string, st State) error {
	h.mu.Lock()
	s, ok := h.authenticated(id, token)
	if !ok {
		h.mu.Unlock()
		return ErrUnknownSession
	}
	s.project = st.Project
	if st.Focused {
		s.lastActive = h.now()
	}
	var write json.RawMessage
	if len(st.Tools) > 0 && json.Valid(st.Tools) && string(st.Tools) != string(h.tools) {
		h.tools = append(json.RawMessage(nil), st.Tools...)
		write = h.tools
	}
	h.mu.Unlock()
	if write != nil {
		h.writeCache(write)
	}
	return nil
}

func (h *Hub) writeCache(tools json.RawMessage) {
	if h.cachePath == "" {
		return
	}
	tmp := h.cachePath + ".tmp"
	if err := os.MkdirAll(filepath.Dir(h.cachePath), 0o755); err != nil {
		slog.Warn("agent tool cache dir", "err", err)
		return
	}
	if err := os.WriteFile(tmp, tools, 0o644); err != nil {
		slog.Warn("agent tool cache write", "err", err)
		return
	}
	if err := os.Rename(tmp, h.cachePath); err != nil {
		slog.Warn("agent tool cache rename", "err", err)
	}
}

// Deliver hands a tab's tool result to the waiting call.
func (h *Hub) Deliver(id, token, callID string, result json.RawMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.authenticated(id, token)
	if !ok {
		return ErrUnknownSession
	}
	ch, ok := s.pending[callID]
	if !ok {
		return nil
	}
	delete(s.pending, callID)
	ch <- result
	return nil
}

// Tools returns the last tool list a tab advertised, or nil.
func (h *Hub) Tools() json.RawMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tools
}

// OpenProject is a project open in at least one tab.
type OpenProject struct {
	Project
	Tabs      int  `json:"tabs"`
	InFocus   bool `json:"inFocus"`
	lastFocus time.Time
}

// Projects lists the projects open in connected tabs, most recently used first.
func (h *Hub) Projects() []OpenProject {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.projectsLocked()
}

func (h *Hub) projectsLocked() []OpenProject {
	byID := map[string]*OpenProject{}
	var latest time.Time
	for _, s := range h.sessions {
		if s.lastActive.After(latest) {
			latest = s.lastActive
		}
		if s.project == nil || s.project.ProjectID == "" {
			continue
		}
		p, ok := byID[s.project.ProjectID]
		if !ok {
			p = &OpenProject{Project: *s.project}
			byID[s.project.ProjectID] = p
		}
		p.Tabs++
		if s.lastActive.After(p.lastFocus) {
			p.lastFocus = s.lastActive
			p.Project = *s.project
		}
	}
	out := make([]OpenProject, 0, len(byID))
	for _, p := range byID {
		p.InFocus = !latest.IsZero() && p.lastFocus.Equal(latest)
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].lastFocus.After(out[j].lastFocus) })
	return out
}

func describe(projects []OpenProject) string {
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		title := p.Title
		if title == "" {
			title = "Untitled"
		}
		names = append(names, fmt.Sprintf("%q (projectId %s)", title, p.ProjectID))
	}
	return strings.Join(names, ", ")
}

// route picks the tab for a call. A project open in several tabs goes to the
// one the user touched last.
func (h *Hub) routeLocked(projectID string) (*Session, error) {
	if len(h.sessions) == 0 {
		return nil, &RoutingError{Message: "Composer isn't connected. Ask the user to open Composer and turn on \"Let AI agents edit through Composer Bridge\" in its Advanced settings."}
	}
	projects := h.projectsLocked()
	if projectID == "" {
		switch len(projects) {
		case 0:
			return nil, &RoutingError{Message: "No project is open in Composer. Ask the user to open one."}
		case 1:
			projectID = projects[0].ProjectID
		default:
			return nil, &RoutingError{Message: "Several projects are open in Composer: " + describe(projects) + ". Pass projectId to pick one."}
		}
	}
	var best *Session
	for _, s := range h.sessions {
		if s.project == nil || s.project.ProjectID != projectID {
			continue
		}
		if best == nil || s.lastActive.After(best.lastActive) {
			best = s
		}
	}
	if best == nil {
		msg := fmt.Sprintf("Project %s isn't open in Composer.", projectID)
		if len(projects) > 0 {
			msg += " Open projects: " + describe(projects) + "."
		} else {
			msg += " Ask the user to open it."
		}
		return nil, &RoutingError{Message: msg}
	}
	return best, nil
}

type callEvent struct {
	CallID    string          `json:"callId"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// Call forwards a tool call to the tab showing the requested project and
// returns the tab's MCP CallToolResult as raw JSON.
func (h *Hub) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	var target struct {
		ProjectID string `json:"projectId"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &target)
	}

	h.mu.Lock()
	s, err := h.routeLocked(target.ProjectID)
	if err != nil {
		h.mu.Unlock()
		return nil, err
	}
	h.callSeq++
	callID := fmt.Sprintf("c%d", h.callSeq)
	reply := make(chan json.RawMessage, 1)
	s.pending[callID] = reply
	h.mu.Unlock()

	cleanup := func() {
		h.mu.Lock()
		delete(s.pending, callID)
		h.mu.Unlock()
	}

	timeout := time.NewTimer(h.CallTimeout)
	defer timeout.Stop()

	select {
	case s.events <- Event{Name: "call", Data: callEvent{CallID: callID, Name: name, Arguments: args}}:
	case <-s.done:
		cleanup()
		return nil, &RoutingError{Message: "The Composer tab disconnected. Try again."}
	case <-ctx.Done():
		cleanup()
		return nil, ctx.Err()
	case <-timeout.C:
		cleanup()
		return nil, &RoutingError{Message: "The Composer tab is busy. Try again."}
	}

	select {
	case result := <-reply:
		return result, nil
	case <-s.done:
		cleanup()
		return nil, &RoutingError{Message: "The Composer tab disconnected before answering. Check the project before retrying."}
	case <-ctx.Done():
		cleanup()
		return nil, ctx.Err()
	case <-timeout.C:
		cleanup()
		return nil, &RoutingError{Message: "Composer didn't answer in time. Check the project before retrying."}
	}
}
