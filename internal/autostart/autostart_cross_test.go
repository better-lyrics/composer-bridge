package autostart

import "testing"

func TestSetEnabledIsCallable(t *testing.T) {
	// On every platform SetEnabled(false, "") must succeed (idempotent disable).
	if err := SetEnabled(false, ""); err != nil {
		t.Errorf("SetEnabled(false, \"\"): %v", err)
	}
}
