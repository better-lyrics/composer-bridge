package autostart

import (
	"runtime"
	"testing"
)

func TestSetEnabledIsCallable(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Skip on windows because SetEnabled(false, "") mutates the real HKCU
		// Run key; the windows-only test file covers idempotent disable via
		// setEnabledWithName against a sandboxed value name.
		t.Skip("windows tested via setEnabledWithName(false, \"\", testValueName) in autostart_windows_test.go")
	}
	if err := SetEnabled(false, ""); err != nil {
		t.Errorf("SetEnabled(false, \"\"): %v", err)
	}
}
