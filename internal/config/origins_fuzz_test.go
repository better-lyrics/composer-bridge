package config

import (
	"slices"
	"strings"
	"testing"
)

func FuzzSplitAndCleanOrigins(f *testing.F) {
	f.Add("https://a.dev, https://b.dev", "http://localhost:5173")
	f.Add(",,", " ")
	f.Fuzz(func(t *testing.T, a, b string) {
		out := SplitAndCleanOrigins([]string{a, b})
		for _, o := range out {
			if o == "" || strings.Contains(o, ",") || strings.TrimSpace(o) != o {
				t.Fatalf("unclean entry %q in %q", o, out)
			}
		}
		if again := SplitAndCleanOrigins(out); !slices.Equal(again, out) && len(out) > 0 {
			t.Fatalf("not idempotent: %q then %q", out, again)
		}
	})
}
