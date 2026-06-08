//go:build linux

package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setXDG(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	return tmp
}

func TestSetEnabledTrueWritesADesktopEntry(t *testing.T) {
	xdg := setXDG(t)
	if err := SetEnabled(true, "/home/me/.local/bin/composer-bridge.AppImage"); err != nil {
		t.Fatalf("SetEnabled true: %v", err)
	}
	path := filepath.Join(xdg, "autostart", "composer-bridge.desktop")
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read desktop file: %v", err)
	}
	contents := string(bytes)
	for _, want := range []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Composer Bridge",
		"Exec=/home/me/.local/bin/composer-bridge.AppImage",
		"Terminal=false",
		"X-GNOME-Autostart-enabled=true",
	} {
		if !strings.Contains(contents, want) {
			t.Errorf("desktop file missing %q\nfull contents:\n%s", want, contents)
		}
	}
}

func TestSetEnabledFalseRemovesTheFile(t *testing.T) {
	xdg := setXDG(t)
	_ = SetEnabled(true, "/home/me/.local/bin/composer-bridge.AppImage")
	if err := SetEnabled(false, ""); err != nil {
		t.Fatalf("SetEnabled false: %v", err)
	}
	path := filepath.Join(xdg, "autostart", "composer-bridge.desktop")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("desktop file still present after SetEnabled(false): %v", err)
	}
}

func TestSetEnabledFalseIsIdempotent(t *testing.T) {
	setXDG(t)
	if err := SetEnabled(false, ""); err != nil {
		t.Errorf("SetEnabled(false) on empty dir: %v", err)
	}
}

func TestIsEnabledReflectsFilePresence(t *testing.T) {
	setXDG(t)
	if IsEnabled() {
		t.Error("IsEnabled true before any SetEnabled call")
	}
	_ = SetEnabled(true, "/home/me/.local/bin/composer-bridge.AppImage")
	if !IsEnabled() {
		t.Error("IsEnabled false after SetEnabled(true)")
	}
	_ = SetEnabled(false, "")
	if IsEnabled() {
		t.Error("IsEnabled true after SetEnabled(false)")
	}
}

func TestSetEnabledRejectsNewlineInExecPath(t *testing.T) {
	setXDG(t)
	for _, bad := range []string{"/tmp/app\n.exe", "/tmp/app\r.exe", "/tmp/app\r\n.exe"} {
		if err := SetEnabled(true, bad); err == nil {
			t.Errorf("SetEnabled(true, %q): got nil, want error", bad)
		}
	}
}
