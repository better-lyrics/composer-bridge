//go:build !darwin

package tray

// dispatchStart on non-Darwin platforms is a direct call; only AppKit on macOS
// has the strict main-thread requirement for status-bar APIs.
func dispatchStart(fn func()) {
	fn()
}

// dispatchMain on non-Darwin platforms is a direct call for the same reason.
func dispatchMain(fn func()) {
	fn()
}
