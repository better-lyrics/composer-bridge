// Package events decouples activity-style notifications from the Wails runtime.
// The bridge's HTTP handlers can't import wailsapp/wails/v2/pkg/runtime without
// pulling the whole desktop stack into the test binary; they take an Emitter
// interface and the main package injects a Wails-backed implementation at
// startup, while tests pass either Noop or a recording fake.
package events

import "context"

// Emitter forwards a named event with arbitrary args to interested listeners.
// The ctx is the Wails runtime context captured at OnStartup; implementations
// that need it (such as the Wails-backed Emitter) pull it from there. Tests
// and the Noop default ignore ctx.
type Emitter interface {
	Emit(ctx context.Context, name string, args ...any)
}

// Noop is the default Emitter used when no live runtime is wired (tests, or
// the bridge running headless before Startup fires). It drops every event.
var Noop Emitter = noopEmitter{}

type noopEmitter struct{}

func (noopEmitter) Emit(_ context.Context, _ string, _ ...any) {}

// EmitterFunc adapts a plain function into the Emitter interface. main.go uses
// this to wrap wailsRuntime.EventsEmit without defining a named type.
type EmitterFunc func(ctx context.Context, name string, args ...any)

// Emit calls the underlying function.
func (f EmitterFunc) Emit(ctx context.Context, name string, args ...any) {
	f(ctx, name, args...)
}
