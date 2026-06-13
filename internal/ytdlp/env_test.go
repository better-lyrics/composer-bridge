package ytdlp

import (
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestAugmentPATH(t *testing.T) {
	sep := string(os.PathListSeparator)
	cases := []struct {
		name      string
		env       []string
		extras    []string
		wantPath  string
		wantOther []string
	}{
		{
			name:     "missing dirs prepended in order",
			env:      []string{"PATH=/usr/bin:/bin"},
			extras:   []string{"/opt/homebrew/bin", "/opt/local/bin"},
			wantPath: "/opt/homebrew/bin" + sep + "/opt/local/bin" + sep + "/usr/bin:/bin",
		},
		{
			name:     "already-present dirs are not duplicated",
			env:      []string{"PATH=/opt/homebrew/bin:/usr/bin"},
			extras:   []string{"/opt/homebrew/bin", "/opt/local/bin"},
			wantPath: "/opt/local/bin" + sep + "/opt/homebrew/bin:/usr/bin",
		},
		{
			name:     "all dirs already present returns env unchanged",
			env:      []string{"PATH=/opt/homebrew/bin:/opt/local/bin:/usr/bin"},
			extras:   []string{"/opt/homebrew/bin", "/opt/local/bin"},
			wantPath: "/opt/homebrew/bin:/opt/local/bin:/usr/bin",
		},
		{
			name:     "dir present in the middle of PATH is still treated as present",
			env:      []string{"PATH=/foo:/opt/homebrew/bin:/bar"},
			extras:   []string{"/opt/homebrew/bin"},
			wantPath: "/foo:/opt/homebrew/bin:/bar",
		},
		{
			name:     "empty PATH var still gets extras",
			env:      []string{"PATH="},
			extras:   []string{"/opt/homebrew/bin"},
			wantPath: "/opt/homebrew/bin",
		},
		{
			name:     "missing PATH var gets one appended",
			env:      []string{"HOME=/h"},
			extras:   []string{"/opt/homebrew/bin"},
			wantPath: "/opt/homebrew/bin",
		},
		{
			name:     "empty extras returns env unchanged",
			env:      []string{"PATH=/usr/bin"},
			extras:   nil,
			wantPath: "/usr/bin",
		},
		{
			name:     "empty-string extra is skipped, not prepended as ::",
			env:      []string{"PATH=/usr/bin"},
			extras:   []string{"", "/opt/homebrew/bin", ""},
			wantPath: "/opt/homebrew/bin:/usr/bin",
		},
		{
			name:     "duplicate extras are deduped within the prepend list",
			env:      []string{"PATH=/usr/bin"},
			extras:   []string{"/opt/homebrew/bin", "/opt/homebrew/bin"},
			wantPath: "/opt/homebrew/bin:/usr/bin",
		},
		{
			name:      "other env vars are preserved",
			env:       []string{"HOME=/h", "PATH=/usr/bin", "USER=u"},
			extras:    []string{"/opt/homebrew/bin"},
			wantPath:  "/opt/homebrew/bin:/usr/bin",
			wantOther: []string{"HOME=/h", "USER=u"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := augmentPATH(tc.env, tc.extras)
			gotPath := ""
			gotOther := []string{}
			for _, kv := range got {
				if p, ok := strings.CutPrefix(kv, "PATH="); ok {
					gotPath = p
					continue
				}
				gotOther = append(gotOther, kv)
			}
			if gotPath != tc.wantPath {
				t.Errorf("PATH: got %q, want %q", gotPath, tc.wantPath)
			}
			if tc.wantOther != nil && !slices.Equal(gotOther, tc.wantOther) {
				t.Errorf("other env: got %v, want %v", gotOther, tc.wantOther)
			}
		})
	}
}

// TestAugmentPATH_DoesNotMutateInputSlice guards against an easy bug: if
// augmentPATH wrote back into the caller's env slice, repeated calls in the
// same process would compound entries.
func TestAugmentPATH_DoesNotMutateInputSlice(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/h"}
	orig := slices.Clone(env)
	_ = augmentPATH(env, []string{"/opt/homebrew/bin"})
	if !slices.Equal(env, orig) {
		t.Errorf("input env mutated: got %v, want %v", env, orig)
	}
}

// TestExecEnv_RegressionForLaunchdMinimalPATH locks in the fix for the
// scenario that motivated this file: macOS apps spawned by launchd inherit
// PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin with no /opt/homebrew/bin,
// which made yt-dlp's youtube extractor silently fail to solve n-sig
// challenges. The fix is execEnv() prepending the bundled-deno dir plus the
// Homebrew dirs on macOS. If anyone removes those prepends, this test fires.
func TestExecEnv_RegressionForLaunchdMinimalPATH(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("regression is darwin-specific")
	}
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin")
	defer SetDenoBinDir("")
	SetDenoBinDir("/fake/data/bin")

	env := execEnv()
	var gotPath string
	for _, kv := range env {
		if p, ok := strings.CutPrefix(kv, "PATH="); ok {
			gotPath = p
			break
		}
	}
	required := []string{"/fake/data/bin", "/opt/homebrew/bin", "/opt/local/bin"}
	for _, dir := range required {
		if !strings.Contains(gotPath, dir) {
			t.Errorf("PATH %q missing required dir %q (regression: launchd minimal PATH must be augmented)", gotPath, dir)
		}
	}
	// And the bundled-deno dir must come first, ahead of system dirs, so a
	// bundled binary wins over whatever happens to be installed system-wide.
	if !strings.HasPrefix(gotPath, "/fake/data/bin"+string(os.PathListSeparator)) {
		t.Errorf("PATH %q does not start with bundled deno dir; bundled binary won't win lookup", gotPath)
	}
}

func TestSetDenoBinDir_TakesEffectInExecEnv(t *testing.T) {
	defer SetDenoBinDir("")
	t.Setenv("PATH", "/usr/bin")
	SetDenoBinDir("/some/data/bin")
	env := execEnv()
	var got string
	for _, kv := range env {
		if p, ok := strings.CutPrefix(kv, "PATH="); ok {
			got = p
			break
		}
	}
	if !strings.HasPrefix(got, "/some/data/bin") {
		t.Errorf("PATH %q does not start with set deno bin dir", got)
	}
}

func TestSetDenoBinDir_EmptyDoesNotPrependEmptyEntry(t *testing.T) {
	defer SetDenoBinDir("")
	t.Setenv("PATH", "/usr/bin")
	SetDenoBinDir("")
	env := execEnv()
	var got string
	for _, kv := range env {
		if p, ok := strings.CutPrefix(kv, "PATH="); ok {
			got = p
			break
		}
	}
	// Must not start with the path list separator (which would mean we
	// prepended an empty entry, equivalent to "." in PATH lookup rules).
	if strings.HasPrefix(got, string(os.PathListSeparator)) {
		t.Errorf("PATH %q starts with separator, indicating an empty prepended entry", got)
	}
}
