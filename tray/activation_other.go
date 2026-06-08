//go:build !darwin

package tray

// SetBackground is a no-op on non-Darwin platforms; only macOS has the
// activation-policy concept that hides the Dock icon.
func SetBackground() {}

// SetForeground is a no-op on non-Darwin platforms.
func SetForeground() {}
