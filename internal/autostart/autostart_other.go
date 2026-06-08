//go:build !darwin

package autostart

import "errors"

// SetEnabled returns an error on non-Darwin platforms. The frontend should
// keep the toggle disabled in that case (see App.SupportsAutostart).
func SetEnabled(_ bool, _ string) error {
	return errors.New("autostart is only supported on macOS for now")
}

// IsEnabled always returns false on non-Darwin platforms.
func IsEnabled() bool {
	return false
}
