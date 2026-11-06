package server

import (
	"strings"
	"testing"
)

func FuzzMatchOrigin(f *testing.F) {
	f.Add("https://*.example.com", "https://app.example.com")
	f.Add("https://*.example.com", "https://example.com")
	f.Add("https://*.com", "https://evil.com")
	f.Add("http://*.example.com:5173", "http://a.example.com:5173")
	f.Add("https://composer.boidu.dev", "https://composer.boidu.dev")
	f.Fuzz(func(t *testing.T, pattern, origin string) {
		if !isSerializedOrigin(origin) || !matchOrigin(pattern, origin) {
			return
		}
		p, o := strings.ToLower(pattern), strings.ToLower(origin)
		if p == o {
			return
		}
		scheme, suffix, ok := strings.Cut(p, "://*.")
		if !ok {
			t.Fatalf("non-wildcard pattern %q matched different origin %q", pattern, origin)
		}
		host, ok := strings.CutPrefix(o, scheme+"://")
		if !ok {
			t.Fatalf("pattern %q matched origin %q with another scheme", pattern, origin)
		}
		if host == suffix || !strings.HasSuffix(host, "."+suffix) {
			t.Fatalf("pattern %q matched %q, which is not a subdomain", pattern, origin)
		}
		if !strings.Contains(suffix, ".") {
			t.Fatalf("pattern %q opened a bare TLD to %q", pattern, origin)
		}
	})
}

// Browsers send Origin as an ASCII serialized origin: scheme://host[:port],
// with punycode hosts and no path, query or userinfo.
func isSerializedOrigin(o string) bool {
	_, host, ok := strings.Cut(o, "://")
	if !ok || host == "" {
		return false
	}
	for _, r := range host {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == ':') {
			return false
		}
	}
	return true
}
