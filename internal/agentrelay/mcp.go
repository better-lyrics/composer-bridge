package agentrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"
)

// supportedProtocolVersions lists the MCP revisions this server speaks, newest
// first. It only uses tools, whose wire format is the same in all of them.
var supportedProtocolVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const serverInstructions = "Composer is a lyrics editor running in the user's browser. " +
	"Call list_projects to see the projects open in Composer, then get_project and get_lines (withIssues: true) to find the warnings the Languages tab shows. " +
	"Write with set_transliterations, set_translations and align_transliteration. Each write is one undo step the user can revert in Composer."

var listProjectsTool = json.RawMessage(`{
  "name": "list_projects",
  "title": "List open projects",
  "description": "List the projects open in Composer browser tabs. Pass a projectId to the other tools when more than one is open. inFocus marks the tab the user used last.",
  "inputSchema": {"type": "object", "properties": {}},
  "annotations": {"readOnlyHint": true}
}`)

// MCP serves the Streamable HTTP transport of the Model Context Protocol in
// its simplest form: every POST gets one JSON response, and there is no
// server-initiated stream.
type MCP struct {
	Hub     *Hub
	Version string
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ServeHTTP handles /mcp.
func (m *MCP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// MCP clients are not browsers. Refusing any Origin keeps web pages, and
	// DNS rebinding through a non-loopback Host, from driving the user's project.
	if r.Header.Get("Origin") != "" || !isLoopbackHost(r.Host) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		m.notifications(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPostBytes))
	if err != nil {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	body = bytes.TrimSpace(body)

	var responses []rpcResponse
	batch := len(body) > 0 && body[0] == '['
	if batch {
		var reqs []rpcRequest
		if err := json.Unmarshal(body, &reqs); err != nil {
			writeJSON(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}})
			return
		}
		for _, req := range reqs {
			if resp := m.handle(r.Context(), req); resp != nil {
				responses = append(responses, *resp)
			}
		}
	} else {
		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}})
			return
		}
		if resp := m.handle(r.Context(), req); resp != nil {
			responses = append(responses, *resp)
		}
	}

	if len(responses) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if batch {
		writeJSON(w, responses)
		return
	}
	writeJSON(w, responses[0])
}

var toolsChangedNotification = []byte(`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`)

// notifications is the transport's optional server-to-client stream. Its only
// message is tools/list_changed: a client that connected before any Composer
// tab sees just list_projects, and lists again once a tab advertises its tools.
func (m *MCP) notifications(w http.ResponseWriter, r *http.Request) {
	changes, stop := m.Hub.WatchTools()
	defer stop()

	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		slog.Debug("mcp stream read deadline", "err", err)
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.Debug("mcp stream write deadline", "err", err)
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if err := rc.Flush(); err != nil {
		return
	}

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case _, open := <-changes:
			if !open {
				return
			}
			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", toolsChangedNotification); err != nil {
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("mcp response write", "err", err)
	}
}

// handle returns nil for notifications and for responses sent by the client.
func (m *MCP) handle(ctx context.Context, req rpcRequest) *rpcResponse {
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return nil
	}
	resp := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = m.initialize(req.Params)
	case "ping":
		resp.Result = struct{}{}
	case "tools/list":
		resp.Result = map[string]any{"tools": m.tools()}
	case "tools/call":
		result, rpcErr := m.callTool(ctx, req.Params)
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
}

func (m *MCP) initialize(params json.RawMessage) any {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &p)
	version := supportedProtocolVersions[0]
	if slices.Contains(supportedProtocolVersions, p.ProtocolVersion) {
		version = p.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
		"serverInfo":      map[string]any{"name": "composer", "title": "Composer", "version": m.Version},
		"instructions":    serverInstructions,
	}
}

func (m *MCP) tools() []json.RawMessage {
	tools := []json.RawMessage{listProjectsTool}
	var tabTools []json.RawMessage
	if raw := m.Hub.Tools(); len(raw) > 0 {
		if err := json.Unmarshal(raw, &tabTools); err != nil {
			slog.Warn("advertised agent tools unreadable", "err", err)
		}
	}
	return append(tools, tabTools...)
}

func textResult(text string, isError bool) map[string]any {
	result := map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
	if isError {
		result["isError"] = true
	}
	return result
}

func (m *MCP) callTool(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
		return nil, &rpcError{Code: -32602, Message: "tools/call needs a tool name"}
	}
	if p.Name == "list_projects" {
		data, err := json.MarshalIndent(map[string]any{"projects": m.Hub.Projects()}, "", " ")
		if err != nil {
			return nil, &rpcError{Code: -32603, Message: err.Error()}
		}
		return textResult(string(data), false), nil
	}
	result, err := m.Hub.Call(ctx, p.Name, p.Arguments)
	if err != nil {
		var routing *RoutingError
		if errors.As(err, &routing) {
			return textResult(routing.Message, true), nil
		}
		return nil, &rpcError{Code: -32603, Message: err.Error()}
	}
	return result, nil
}
