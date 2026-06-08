// Package tray wires the system tray icon and its menu (Show / Quit) into the
// Wails app. The tray runs in a background goroutine; Wails owns main.
package tray

// Controller is the long-lived handle to the tray menu. Owns the runtime
// context so menu click handlers can call WindowShow / Quit on the right app.
type Controller struct {
	ctx any
}

// New builds an unbound Controller. Call BindContext from OnStartup once Wails
// has handed you a runtime context.
func New() *Controller {
	return &Controller{}
}

// BindContext stores the Wails runtime context for use by menu handlers.
func (c *Controller) BindContext(ctx any) {
	c.ctx = ctx
}

// HasContext reports whether BindContext has been called yet. Used by tests
// and by the menu handler goroutine to drop clicks that arrive before Wails
// has finished starting up.
func (c *Controller) HasContext() bool {
	return c.ctx != nil
}
