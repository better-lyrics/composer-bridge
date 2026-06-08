package autostart

import (
	"runtime"
	"testing"
)

func TestSetEnabledIsCallable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("autostart not yet implemented on this platform; covered by Tasks 9 and 10")
	}
	if err := SetEnabled(false, ""); err != nil {
		t.Errorf("SetEnabled(false, \"\"): %v", err)
	}
}
