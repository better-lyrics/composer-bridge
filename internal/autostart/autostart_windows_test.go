//go:build windows

package autostart

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// Each test cleans up the Run-key value it touched.
const testValueName = "Composer Bridge Test"

func clean(t *testing.T) {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		t.Fatalf("clean OpenKey: %v", err)
	}
	defer k.Close()
	_ = k.DeleteValue(testValueName)
}

func TestSetEnabledTrueWritesQuotedExecPath(t *testing.T) {
	t.Cleanup(func() { clean(t) })
	if err := setEnabledWithName(true, `C:\Program Files\App\app.exe`, testValueName); err != nil {
		t.Fatalf("SetEnabled true: %v", err)
	}
	k, _ := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	defer k.Close()
	val, _, _ := k.GetStringValue(testValueName)
	want := `"C:\Program Files\App\app.exe"`
	if val != want {
		t.Errorf("registry value: got %q, want %q", val, want)
	}
}

func TestSetEnabledFalseDeletesValueAndIsIdempotent(t *testing.T) {
	t.Cleanup(func() { clean(t) })
	_ = setEnabledWithName(true, `C:\app.exe`, testValueName)
	if err := setEnabledWithName(false, "", testValueName); err != nil {
		t.Errorf("SetEnabled false: %v", err)
	}
	if err := setEnabledWithName(false, "", testValueName); err != nil {
		t.Errorf("SetEnabled false (second call): %v", err)
	}
}
