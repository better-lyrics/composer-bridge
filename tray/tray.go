// Package tray wires the system tray icon and its menu (Show / Quit) into the
// Wails app. The tray runs in a background goroutine; Wails owns main.
package tray

import (
	"context"
	"runtime"
	"sync"

	"github.com/getlantern/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/better-lyrics/composer-bridge/tray/icons"
)

// Controller is the long-lived handle to the tray menu. Owns the runtime
// context so menu click handlers can call WindowShow / Quit on the right app.
type Controller struct {
	mu  sync.Mutex
	ctx context.Context
}

// New builds an unbound Controller. Call BindContext from OnStartup once Wails
// has handed you a runtime context.
func New() *Controller {
	return &Controller{}
}

// BindContext stores the Wails runtime context for use by menu handlers.
func (c *Controller) BindContext(ctx context.Context) {
	c.mu.Lock()
	c.ctx = ctx
	c.mu.Unlock()
}

// HasContext reports whether BindContext has been called yet. Used by tests
// and by the menu handler goroutine to drop clicks that arrive before Wails
// has finished starting up.
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

// Register installs the tray icon and menu via systray.Register (which does
// NOT take the main thread). Must be called BEFORE wails.Run on the same
// goroutine.
func (c *Controller) Register() {
	systray.Register(c.onReady, c.onExit)
}

func (c *Controller) onReady() {
	iconBytes := icons.Default
	if runtime.GOOS == "darwin" {
		systray.SetTemplateIcon(icons.Mac, icons.Mac)
	} else {
		systray.SetIcon(iconBytes)
	}
	systray.SetTooltip("Composer Bridge")

	mShow := systray.AddMenuItem("Show Composer Bridge", "Open the window")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop the bridge and quit")

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				if ctx := c.Context(); ctx != nil {
					wailsRuntime.WindowShow(ctx)
				}
			case <-mQuit.ClickedCh:
				if ctx := c.Context(); ctx != nil {
					wailsRuntime.Quit(ctx)
				}
				return
			}
		}
	}()
}

func (c *Controller) onExit() {
	// systray cleanup runs here; nothing to do for now.
}
