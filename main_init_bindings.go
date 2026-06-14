//go:build bindings

package main

import "context"

// bootstrapYtdlp and scheduleYtdlpRefresh are no-ops in bindings-generation
// mode (wails build compiles main with -tags bindings and runs it to
// introspect the types passed to options.App.Bind via reflection; the wails
// substitute Run emits JSON and exits, so heavy init never matters). See
// wails v2.10.1 pkg/commands/bindings/bindings.go:66-90.
func bootstrapYtdlp(_ context.Context, _, _, _ string) error { return nil }

func scheduleYtdlpRefresh(_ context.Context, _ string, _, _ func() string, _ func(string)) {}

func bootstrapDeno(_ string) {}
