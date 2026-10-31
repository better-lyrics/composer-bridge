package server

import (
	"net/http"
	"slices"
	"strings"
)

// WithCORS returns next wrapped with CORS handling. allowedFn is called per
// request so changes to the underlying list (e.g. SaveConfig appending a new
// origin) take effect on the next request without restarting the server.
// Origins are matched exactly after trimming any trailing slash, except entries of
// the form scheme://*.host[:port], which match any subdomain of host. Requests without
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
// settings edit. Compiled in, not fetched, so an already-running old binary only
// picks it up after it updates.
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
		if matchOrigin(normalizeOrigin(o), target) {
			return true
		}
	}
	return false
}

// matchOrigin requires at least two non-empty labels after "*." so a pattern like
// https://*.com cannot open CORS to a bare TLD. Public suffixes such as *.co.uk are
// not detected.
func matchOrigin(pattern, origin string) bool {
	pattern, origin = strings.ToLower(pattern), strings.ToLower(origin)
	if pattern == origin {
		return true
	}
	scheme, suffix, ok := strings.Cut(pattern, "://*.")
	if !ok || !strings.Contains(suffix, ".") || slices.Contains(strings.Split(suffix, "."), "") {
		return false
	}
	host, ok := strings.CutPrefix(origin, scheme+"://")
	return ok && len(host) > len(suffix)+1 && strings.HasSuffix(host, "."+suffix) && !strings.HasPrefix(host, ".")
}

func normalizeOrigin(o string) string { return strings.TrimRight(o, "/") }
