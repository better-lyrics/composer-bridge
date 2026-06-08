//go:build linux

package autostart

import "errors"

func SetEnabled(_ bool, _ string) error {
	return errors.New("autostart linux: not implemented yet")
}

func IsEnabled() bool { return false }
