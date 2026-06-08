package events

import (
	"context"
	"testing"
)

func TestNoop_DropsEventsSilently(t *testing.T) {
	Noop.Emit(context.Background(), "anything", "payload", 42)
}

func TestEmitterFunc_ForwardsArgs(t *testing.T) {
	var gotName string
	var gotArgs []any
	var fn EmitterFunc = func(_ context.Context, name string, args ...any) {
		gotName = name
		gotArgs = args
	}
	fn.Emit(context.Background(), "activity:update", "hello", 7)
	if gotName != "activity:update" {
		t.Errorf("name: got %q, want activity:update", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "hello" || gotArgs[1] != 7 {
		t.Errorf("args: got %v, want [hello 7]", gotArgs)
	}
}
