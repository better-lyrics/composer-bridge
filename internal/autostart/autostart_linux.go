//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const desktopFileName = "composer-bridge.desktop"

const desktopTemplate = `[Desktop Entry]
Type=Application
Name=Composer Bridge
Comment=Composer Bridge background service
Exec=%s
Terminal=false
Hidden=false
X-GNOME-Autostart-enabled=true
StartupNotify=false
`

func SetEnabled(enabled bool, execPath string) error {
	path, err := autostartFilePath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("autostart remove %s: %w", path, err)
		}
		return nil
	}
	if execPath == "" {
		return fmt.Errorf("autostart: execPath required when enabling")
	}
	if strings.ContainsAny(execPath, "\n\r") {
		return fmt.Errorf("autostart: execPath contains illegal character")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("autostart mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(fmt.Sprintf(desktopTemplate, execPath)), 0o644); err != nil {
		return fmt.Errorf("autostart write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("autostart install %s: %w", path, err)
	}
	return nil
}

func IsEnabled() bool {
	path, err := autostartFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func autostartFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("autostart user config dir: %w", err)
	}
	return filepath.Join(configDir, "autostart", desktopFileName), nil
}

// Refresh is a no-op on linux for the same reason as darwin: the .desktop
// file points at the exec path the user opted in with, and re-toggling
// from Settings is the supported way to update it.
func Refresh(_ string) error { return nil }
