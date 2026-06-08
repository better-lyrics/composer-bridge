//go:build windows

package autostart

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	valueName = "Composer Bridge"
)

func SetEnabled(enabled bool, execPath string) error {
	return setEnabledWithName(enabled, execPath, valueName)
}

func IsEnabled() bool {
	return isEnabledWithName(valueName)
}

func setEnabledWithName(enabled bool, execPath, name string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("autostart open run key: %w", err)
	}
	defer k.Close()
	if !enabled {
		if err := k.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("autostart delete value: %w", err)
		}
		return nil
	}
	if execPath == "" {
		return errors.New("autostart: execPath required when enabling")
	}
	if strings.ContainsAny(execPath, "\n\r\"") {
		return errors.New("autostart: execPath contains illegal character")
	}
	return k.SetStringValue(name, `"`+execPath+`"`)
}

func isEnabledWithName(name string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(name)
	return err == nil
}

// Refresh re-writes the Run key with execPath IF autostart is currently
// enabled. Idempotent when disabled. Call at app startup so a moved or
// renamed binary doesn't leave a stale Run entry pointing at the old
// location.
func Refresh(execPath string) error {
	return refreshWithName(execPath, valueName)
}

func refreshWithName(execPath, name string) error {
	if !isEnabledWithName(name) {
		return nil
	}
	return setEnabledWithName(true, execPath, name)
}
