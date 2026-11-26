package agentrelay

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	keepAliveInterval = 20 * time.Second
	maxPostBytes      = 8 << 20
)

// TabHandlers serves the endpoints Composer tabs use. AllowedOrigin decides
// which web origins may act as a tab; it should be the bridge's CORS policy,
// so a page on any other site can neither open a session nor post into one.
type TabHandlers struct {
	Hub           *Hub
	AllowedOrigin func(origin string) bool
}

// Register mounts the tab endpoints on mux.
func (t *TabHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /agent/connect", t.Connect)
	mux.HandleFunc("POST /agent/state", t.State)
	mux.HandleFunc("POST /agent/result", t.Result)
	mux.HandleFunc("OPTIONS /agent/{path}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func (t *TabHandlers) originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin != "" && t.AllowedOrigin != nil && t.AllowedOrigin(origin)
}

func writeSSE(w http.ResponseWriter, ev Event) error {
	data, err := json.Marshal(ev.Data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Name, data)
	return err
}

// Connect opens a tab's event stream. The first event, "ready", carries the
// session id and token the tab signs its posts with; "call" events follow.
func (t *TabHandlers) Connect(w http.ResponseWriter, r *http.Request) {
	if !t.originOK(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	s, err := t.Hub.Connect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer t.Hub.Disconnect(s)

	// The server's read and write timeouts suit short requests. Left in place,
	// they would cut this stream (and cancel its context) after a few seconds.
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		slog.Debug("agent stream read deadline", "err", err)
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.Debug("agent stream write deadline", "err", err)
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if err := writeSSE(w, Event{Name: "ready", Data: map[string]string{"sessionId": s.ID, "token": s.Token()}}); err != nil {
		return
	}
	if err := rc.Flush(); err != nil {
		slog.Warn("agent stream flush", "err", err)
		return
	}
	slog.Info("composer tab connected for agents", "session", s.ID)
	defer slog.Info("composer tab disconnected", "session", s.ID)

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.Done():
			return
		case ev := <-s.Events():
			if err := writeSSE(w, ev); err != nil {
				return
			}
		case <-keepAlive.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}

type signedPost struct {
	SessionID string `json:"sessionId"`
	Token     string `json:"token"`
}

func decodePost(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxPostBytes)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return false
	}
	return true
}

// State records a tab's tools, open project and focus.
func (t *TabHandlers) State(w http.ResponseWriter, r *http.Request) {
	if !t.originOK(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	var body struct {
		signedPost
		State
	}
	if !decodePost(w, r, &body) {
		return
	}
	if err := t.Hub.UpdateState(body.SessionID, body.Token, body.State); err != nil {
		http.Error(w, "unknown session", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Result hands a tab's answer to the MCP call waiting for it.
func (t *TabHandlers) Result(w http.ResponseWriter, r *http.Request) {
	if !t.originOK(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	var body struct {
		signedPost
		CallID string          `json:"callId"`
		Result json.RawMessage `json:"result"`
	}
	if !decodePost(w, r, &body) {
		return
	}
	if len(body.Result) == 0 || !json.Valid(body.Result) {
		http.Error(w, "missing result", http.StatusBadRequest)
		return
	}
	if err := t.Hub.Deliver(body.SessionID, body.Token, body.CallID, body.Result); err != nil {
		http.Error(w, "unknown session", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
