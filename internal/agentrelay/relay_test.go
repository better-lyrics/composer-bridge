package agentrelay

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const composerOrigin = "https://composer.betterlyrics.org"

func newTestServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/mcp", &MCP{Hub: hub, Version: "test"})
	(&TabHandlers{Hub: hub, AllowedOrigin: func(o string) bool { return o == composerOrigin }}).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		hub.CloseAll()
		srv.Close()
	})
	return srv
}

// fakeTab connects like a Composer tab and answers every call by echoing the
// tool name and the project it was routed to.
type fakeTab struct {
	srv       *httptest.Server
	sessionID string
	token     string
	projectID string
}

func connectTab(t *testing.T, srv *httptest.Server, projectID string, focused bool) *fakeTab {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/agent/connect", nil)
	req.Header.Set("Origin", composerOrigin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connect status %d", resp.StatusCode)
	}
	tab := &fakeTab{srv: srv, projectID: projectID}
	events := make(chan [2]string)
	go func() {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		var name string
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				events <- [2]string{name, strings.TrimPrefix(line, "data: ")}
			}
		}
	}()
	ready := <-events
	var keys struct{ SessionID, Token string }
	if err := json.Unmarshal([]byte(ready[1]), &keys); err != nil || ready[0] != "ready" {
		t.Fatalf("bad ready event %v", ready)
	}
	tab.sessionID, tab.token = keys.SessionID, keys.Token
	tab.post(t, "/agent/state", map[string]any{
		"tools":   []map[string]any{{"name": "get_project", "inputSchema": map[string]any{"type": "object"}}},
		"project": map[string]any{"projectId": projectID, "title": "Song " + projectID},
		"focused": focused,
	})
	go func() {
		for ev := range events {
			var call callEvent
			if ev[0] != "call" || json.Unmarshal([]byte(ev[1]), &call) != nil {
				continue
			}
			tab.post(t, "/agent/result", map[string]any{
				"callId": call.CallID,
				"result": textResult(call.Name+" ran in "+projectID, false),
			})
		}
	}()
	return tab
}

func (tab *fakeTab) post(t *testing.T, path string, body map[string]any) {
	body["sessionId"], body["token"] = tab.sessionID, tab.token
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, tab.srv.URL+path, bytes.NewReader(data))
	req.Header.Set("Origin", composerOrigin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("POST %s status %d", path, resp.StatusCode)
	}
}

func rpc(t *testing.T, srv *httptest.Server, method string, params any) map[string]any {
	t.Helper()
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	resp, err := http.Post(srv.URL+"/mcp", "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func toolText(t *testing.T, out map[string]any) (string, bool) {
	t.Helper()
	result, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", out)
	}
	content := result["content"].([]any)[0].(map[string]any)
	isError, _ := result["isError"].(bool)
	return content["text"].(string), isError
}

func callTool(t *testing.T, srv *httptest.Server, args map[string]any) (string, bool) {
	return toolText(t, rpc(t, srv, "tools/call", map[string]any{"name": "get_project", "arguments": args}))
}

func TestCallsRouteToTheTabShowingTheProject(t *testing.T) {
	hub := NewHub("")
	srv := newTestServer(t, hub)

	if text, isError := callTool(t, srv, nil); !isError || !strings.Contains(text, "isn't connected") {
		t.Fatalf("no tab: got %q", text)
	}

	connectTab(t, srv, "a", false)
	if text, _ := callTool(t, srv, nil); text != "get_project ran in a" {
		t.Fatalf("single project should be the default, got %q", text)
	}

	connectTab(t, srv, "b", true)
	text, isError := callTool(t, srv, nil)
	if !isError || !strings.Contains(text, "projectId a") || !strings.Contains(text, "projectId b") {
		t.Fatalf("several projects must require projectId, got %q", text)
	}
	if text, _ := callTool(t, srv, map[string]any{"projectId": "a"}); text != "get_project ran in a" {
		t.Fatalf("projectId a routed to %q", text)
	}
	if text, _ := callTool(t, srv, map[string]any{"projectId": "b"}); text != "get_project ran in b" {
		t.Fatalf("projectId b routed to %q", text)
	}
	if text, isError := callTool(t, srv, map[string]any{"projectId": "c"}); !isError || !strings.Contains(text, "isn't open") {
		t.Fatalf("closed project: got %q", text)
	}

	text, _ = toolText(t, rpc(t, srv, "tools/call", map[string]any{"name": "list_projects"}))
	var listed struct{ Projects []OpenProject }
	if err := json.Unmarshal([]byte(text), &listed); err != nil || len(listed.Projects) != 2 || listed.Projects[0].ProjectID != "b" || !listed.Projects[0].InFocus {
		t.Fatalf("list_projects should put the focused tab's project first, got %s", text)
	}
}

func TestToolsAreCachedForClientsThatConnectBeforeComposer(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "agent-tools.json")
	srv := newTestServer(t, NewHub(cache))
	connectTab(t, srv, "a", true)

	later := newTestServer(t, NewHub(cache))
	tools := rpc(t, later, "tools/list", nil)["result"].(map[string]any)["tools"].([]any)
	names := []string{}
	for _, tool := range tools {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "list_projects,get_project" {
		t.Fatalf("tools = %v", names)
	}
}

func TestBrowsersCannotReachMCPOrPoseAsATab(t *testing.T) {
	srv := newTestServer(t, NewHub(""))

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Origin", composerOrigin)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("MCP with an Origin header should be forbidden, got %v %v", resp.StatusCode, err)
	}

	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Host = "evil.example:7777"
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("MCP with a rebound Host should be forbidden, got %v %v", resp.StatusCode, err)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/agent/connect", nil)
	req.Header.Set("Origin", "https://evil.example")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if resp, err := http.DefaultClient.Do(req.WithContext(ctx)); err != nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a foreign origin must not open a tab session, got %v %v", resp.StatusCode, err)
	}
}

func TestInitializeNegotiatesTheClientsVersion(t *testing.T) {
	srv := newTestServer(t, NewHub(""))
	result := rpc(t, srv, "initialize", map[string]any{"protocolVersion": "2025-06-18"})["result"].(map[string]any)
	if result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocolVersion = %v", result["protocolVersion"])
	}
	result = rpc(t, srv, "initialize", map[string]any{"protocolVersion": "1999-01-01"})["result"].(map[string]any)
	if result["protocolVersion"] != supportedProtocolVersions[0] {
		t.Fatalf("unknown version should fall back to the newest, got %v", result["protocolVersion"])
	}

	resp, err := http.Post(srv.URL+"/mcp", "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	if err != nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("notifications get 202, got %v %v", resp.StatusCode, err)
	}
}
