//go:build darwin

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withFakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestSetEnabled_EnableWritesPlist(t *testing.T) {
	home := withFakeHome(t)
	if err := SetEnabled(true, "/Applications/Composer Bridge.app/Contents/MacOS/composer-bridge"); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	path := filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}
	if !strings.Contains(string(raw), launchAgentLabel) {
		t.Errorf("plist missing label %q in:\n%s", launchAgentLabel, raw)
	}
	if !strings.Contains(string(raw), "/Applications/Composer Bridge.app") {
		t.Errorf("plist missing exec path in:\n%s", raw)
	}
	if !IsEnabled() {
		t.Error("IsEnabled: got false after enabling")
	}
}

func TestSetEnabled_DisableRemovesPlist(t *testing.T) {
	withFakeHome(t)
	if err := SetEnabled(true, "/usr/local/bin/composer-bridge"); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	if err := SetEnabled(false, ""); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	if IsEnabled() {
		t.Error("IsEnabled: got true after disabling")
	}
}

func TestSetEnabled_DisableIsIdempotent(t *testing.T) {
	withFakeHome(t)
	if err := SetEnabled(false, ""); err != nil {
		t.Errorf("SetEnabled(false) on fresh home: got %v, want nil", err)
	}
}

func TestSetEnabled_EmptyExecPathRejected(t *testing.T) {
	withFakeHome(t)
	if err := SetEnabled(true, ""); err == nil {
		t.Error("SetEnabled(true, \"\"): got nil, want error")
	}
}

func TestSetEnabled_RejectsNewlineInExecPath(t *testing.T) {
	withFakeHome(t)
	for _, bad := range []string{"/tmp/app\n.exe", "/tmp/app\r.exe"} {
		if err := SetEnabled(true, bad); err == nil {
			t.Errorf("SetEnabled(true, %q): got nil, want error", bad)
		}
	}
}
