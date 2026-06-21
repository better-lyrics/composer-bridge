package server

import (
	"net/http"
	"strings"
)

// WithCORS returns next wrapped with CORS handling. allowedFn is called per
// request so changes to the underlying list (e.g. SaveConfig appending a new
// origin) take effect on the next request without restarting the server.
// Origins are matched exactly after trimming any trailing slash. Requests without
// an Origin header pass through untouched (server-to-server calls, curl). Requests
// with a non-allowed Origin still get Vary: Origin so caches don't poison cross-origin
// responses, but never receive Access-Control-Allow-Origin. OPTIONS preflights from
// an allowed origin return 204 No Content.
func WithCORS(next http.Handler, allowedFn func() []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Origin")
		if !originAllowed(origin, allowedFn()) {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type")
		h.Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// firstPartyOrigins are CORS-allowed on every request regardless of the user's
// configured AllowedOrigins. The composer web app gained a new domain; forcing it
// here means the move reaches installs whose saved config predates it, with no
// settings edit. ponytail: compiled in, not fetched, so an already-running old
// binary only picks it up after it updates; add a remote-fetched allowlist if that
// ever matters.
var firstPartyOrigins = []string{
	"https://composer.boidu.dev",
	"https://composer.betterlyrics.org",
}

func originAllowed(origin string, allowed []string) bool {
	target := normalizeOrigin(origin)
	for _, o := range firstPartyOrigins {
		if normalizeOrigin(o) == target {
			return true
		}
	}
	for _, o := range allowed {
		if normalizeOrigin(o) == target {
			return true
		}
	}
	return false
}

func normalizeOrigin(o string) string { return strings.TrimRight(o, "/") }
