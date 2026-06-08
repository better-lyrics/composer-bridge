//go:build windows

package autostart

import (
	"errors"
	"fmt"

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
