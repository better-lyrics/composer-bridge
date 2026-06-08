package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var corsAllowed = []string{
	"https://composer.boidu.dev",
	"http://localhost:5173",
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	})
}

func TestWithCORS_AllowedOriginEchoed(t *testing.T) {
	srv := httptest.NewServer(WithCORS(okHandler(), corsAllowed))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/anything", nil)
	req.Header.Set("Origin", "https://composer.boidu.dev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://composer.boidu.dev" {
		t.Errorf("allow-origin: got %q, want https://composer.boidu.dev", got)
	}
	if got := resp.Header.Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary: got %q, want contains Origin", got)
	}
}

func TestWithCORS_DisallowedOriginNotEchoed(t *testing.T) {
	srv := httptest.NewServer(WithCORS(okHandler(), corsAllowed))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/anything", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin: got %q, want empty for disallowed origin", got)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200 (handler passthrough)", resp.StatusCode)
	}
}

func TestWithCORS_DisallowedOriginStillGetsVary(t *testing.T) {
	srv := httptest.NewServer(WithCORS(okHandler(), corsAllowed))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/anything", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary: got %q, want contains Origin for cache safety", got)
	}
}

func TestWithCORS_EmptyOriginPassesThrough(t *testing.T) {
	srv := httptest.NewServer(WithCORS(okHandler(), corsAllowed))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/anything")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin: got %q, want empty for no-origin request", got)
	}
	if got := resp.Header.Get("Vary"); strings.Contains(got, "Origin") {
		t.Errorf("Vary: got %q, want no Origin for no-origin request", got)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestWithCORS_PreflightReturns204WithHeaders(t *testing.T) {
	srv := httptest.NewServer(WithCORS(okHandler(), corsAllowed))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/import", nil)
	req.Header.Set("Origin", "https://composer.boidu.dev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", resp.StatusCode)
	}
	wantHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "https://composer.boidu.dev",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type",
		"Access-Control-Max-Age":       "86400",
	}
	for k, want := range wantHeaders {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("header %s: got %q, want %q", k, got, want)
		}
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("preflight body: got %q, want empty", body)
	}
}

func TestWithCORS_TrailingSlashOriginMatches(t *testing.T) {
	srv := httptest.NewServer(WithCORS(okHandler(), corsAllowed))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/anything", nil)
	req.Header.Set("Origin", "https://composer.boidu.dev/")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://composer.boidu.dev/" {
		t.Errorf("allow-origin: got %q, want echo of trailing-slash origin", got)
	}
}

func TestWithCORS_AllowedListWithTrailingSlashMatchesPlainOrigin(t *testing.T) {
	allowed := []string{"https://composer.boidu.dev/"}
	srv := httptest.NewServer(WithCORS(okHandler(), allowed))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/anything", nil)
	req.Header.Set("Origin", "https://composer.boidu.dev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://composer.boidu.dev" {
		t.Errorf("allow-origin: got %q, want bidirectional trailing-slash normalization", got)
	}
}

func TestWithCORS_NoWildcardSuffixMatching(t *testing.T) {
	srv := httptest.NewServer(WithCORS(okHandler(), []string{"https://composer.boidu.dev"}))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/anything", nil)
	req.Header.Set("Origin", "https://attacker.composer.boidu.dev")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin: got %q, want empty (no suffix matching)", got)
	}
}
