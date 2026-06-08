package tray

import "testing"

func TestNewReturnsNonNilController(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
}

func TestControllerStartsWithUnsetContext(t *testing.T) {
	c := New()
	if c.HasContext() {
		t.Error("Controller should not have a context before BindContext")
	}
}

func TestBindContextStoresTheContext(t *testing.T) {
	c := New()
	c.BindContext(testCtx())
	if !c.HasContext() {
		t.Error("HasContext should be true after BindContext")
	}
}

func testCtx() ctxKey { return ctxKey{value: "stub"} }

type ctxKey struct{ value string }
