package tray

import (
	"context"
	"testing"
)

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
	c.BindContext(context.Background())
	if !c.HasContext() {
		t.Error("HasContext should be true after BindContext with a real ctx")
	}
}

func TestContextReturnsBoundContext(t *testing.T) {
	c := New()
	parent := context.WithValue(context.Background(), ctxSentinel{}, "ok")
	c.BindContext(parent)
	got := c.Context()
	if got == nil {
		t.Fatal("Context() returned nil after BindContext")
	}
	if got.Value(ctxSentinel{}) != "ok" {
		t.Errorf("Context() did not preserve bound value")
	}
}

type ctxSentinel struct{}
