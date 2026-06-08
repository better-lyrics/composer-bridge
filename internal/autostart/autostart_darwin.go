// Package autostart manages the "open at login" toggle by writing the
// platform-appropriate auto-launch entry.
//
// On macOS the bridge installs a LaunchAgent plist under
// ~/Library/LaunchAgents. macOS reads that directory on user login and
// starts every Label it finds, so no launchctl invocation is required.
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const launchAgentLabel = "dev.boidu.composer-bridge"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func plistContents(execPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>
`, launchAgentLabel, execPath)
}

// SetEnabled writes or removes the LaunchAgent plist so the bridge starts
// (or doesn't) on next login. execPath is the absolute path to the bridge
// binary that the LaunchAgent should run.
func SetEnabled(enabled bool, execPath string) error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove launch agent: %w", err)
		}
		return nil
	}
	if execPath == "" {
		return fmt.Errorf("autostart: empty exec path")
	}
	if strings.ContainsAny(execPath, "\n\r") {
		return fmt.Errorf("autostart: execPath contains illegal character")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir launchagents: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(plistContents(execPath)), 0o644); err != nil {
		return fmt.Errorf("write launch agent: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("install launch agent: %w", err)
	}
	return nil
}

// IsEnabled returns true if a LaunchAgent plist exists. The contents are not
// validated: any plist at the canonical path counts as "enabled."
func IsEnabled() bool {
	path, err := plistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Refresh is a no-op on darwin because LaunchAgent plists encode the exec
// path on first enable and the user re-toggles in Settings if it moves.
func Refresh(_ string) error { return nil }
