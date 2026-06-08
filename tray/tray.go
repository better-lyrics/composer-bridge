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
// onQuit, if set, runs from the Quit menu callback BEFORE wailsRuntime.Quit so
// the App can flip its quitting flag (see App.MarkQuitting) and OnBeforeClose
// can let the quit proceed instead of intercepting it.
type Controller struct {
	mu     sync.Mutex
	ctx    context.Context
	start  func()
	end    func()
	onQuit func()
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

// OnQuit registers a callback the Quit menu invokes before runtime.Quit so
// the App can flip its quitting flag. Safe to call once during startup.
func (c *Controller) OnQuit(fn func()) {
	c.mu.Lock()
	c.onQuit = fn
	c.mu.Unlock()
}

func (c *Controller) quitCallback() func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.onQuit
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

	mShow.Click(c.showWindow)
	mQuit.Click(c.quitApp)

	// energye/systray does not auto-attach the NSMenu to the status item.
	// Wire left-click to restore the window and right-click to open the menu
	// (the library's ShowMenu only works inside the OnRClick callback on macOS).
	systray.SetOnClick(func(_ systray.IMenu) { c.showWindow() })
	systray.SetOnRClick(func(m systray.IMenu) { _ = m.ShowMenu() })
}

func (c *Controller) showWindow() {
	ctx := c.Context()
	if ctx == nil {
		return
	}
	DockShow()
	wailsRuntime.WindowShow(ctx)
}

func (c *Controller) quitApp() {
	ctx := c.Context()
	if ctx == nil {
		return
	}
	if cb := c.quitCallback(); cb != nil {
		cb()
	}
	wailsRuntime.Quit(ctx)
}

// onExit satisfies systray's required callback signature; no cleanup is
// needed today because Stop owns the teardown path.
func (c *Controller) onExit() {}
