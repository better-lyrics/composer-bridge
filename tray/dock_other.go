//go:build !darwin

package tray

// DockShow is a no-op on non-Darwin platforms; only macOS has the
// Dock-icon activation-policy concept.
func DockShow() {}

// DockHide is a no-op on non-Darwin platforms.
func DockHide() {}
