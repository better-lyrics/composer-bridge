// Package tray wires the system tray icon and its menu (Show / Quit) into the
// Wails app. The tray runs in a background goroutine; Wails owns main. The
// energye/systray fork is used so it cooperates with Wails's NSApplication
// delegate on macOS via RunWithExternalLoop.
package tray

import (
	"context"
	"runtime"
	"sync"

	"github.com/energye/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/better-lyrics/composer-bridge/tray/icons"
)

// Controller is the long-lived handle to the tray menu. Owns the runtime
// context so menu click handlers can call WindowShow / Quit on the right app.
type Controller struct {
	mu    sync.Mutex
	ctx   context.Context
	start func()
	end   func()
}

// New builds an unbound Controller. Call Register before wails.Run to install
// the callbacks, then BindContext + Start from OnStartup once Wails has handed
// you a runtime context.
func New() *Controller {
	return &Controller{}
}

// BindContext stores the Wails runtime context for use by menu handlers.
func (c *Controller) BindContext(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	c.mu.Unlock()
}

// HasContext reports whether BindContext has been called yet.
func (c *Controller) HasContext() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ctx != nil
}

// Context returns the bound Wails runtime context, or nil if BindContext has
// not been called yet.
func (c *Controller) Context() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ctx
}

// Register wires up the tray with an external event loop so Wails keeps
// ownership of NSApplication's delegate on macOS. Safe to call BEFORE
// wails.Run because no setDelegate happens here.
func (c *Controller) Register() {
	c.start, c.end = systray.RunWithExternalLoop(c.onReady, c.onExit)
}

// Start fires the tray's deferred ApplicationDidFinishLaunching now that
// Wails has installed its own delegate. Must be called from OnStartup.
// dispatchStart hops onto the macOS main thread before invoking c.start
// because nativeStart touches AppKit, which is main-thread-only.
func (c *Controller) Start() {
	if c.start != nil {
		dispatchStart(c.start)
	}
}

// Stop tears down the tray. Call from OnShutdown.
func (c *Controller) Stop() {
	if c.end != nil {
		c.end()
	}
}

func (c *Controller) onReady() {
	if runtime.GOOS == "darwin" {
		// icons.Mac is the colour appicon for now; template mode desaturates it.
		// Swap for a monochrome silhouette PNG when one is designed.
		systray.SetTemplateIcon(icons.Mac, icons.Mac)
	} else {
		systray.SetIcon(icons.Default)
	}
	systray.SetTooltip("Composer Bridge")

	mShow := systray.AddMenuItem("Show Composer Bridge", "Open the window")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop the bridge and quit")

	mShow.Click(func() {
		if ctx := c.Context(); ctx != nil {
			wailsRuntime.WindowShow(ctx)
		}
	})
	mQuit.Click(func() {
		if ctx := c.Context(); ctx != nil {
			wailsRuntime.Quit(ctx)
		}
	})
}

// onExit satisfies systray's required callback signature; no cleanup is
// needed today because Stop owns the teardown path.
func (c *Controller) onExit() {}
