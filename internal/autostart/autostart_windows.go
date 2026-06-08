//go:build windows

package autostart

import "errors"

func SetEnabled(_ bool, _ string) error {
	return errors.New("autostart windows: not implemented yet")
}

func IsEnabled() bool { return false }
