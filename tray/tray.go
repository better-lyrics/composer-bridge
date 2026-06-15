// Package tray wires the system tray icon and its menu (live state, recent
// downloads, server toggle, settings, quit) into the Wails app. The tray runs
// in a background goroutine; Wails owns main. The energye/systray fork is
// used so it cooperates with Wails's NSApplication delegate on macOS via
// RunWithExternalLoop.
package tray

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
	"github.com/better-lyrics/composer-bridge/tray/icons"
)

// RecentEntry is the slim shape the tray needs to render the Recent
// Downloads submenu. main.go owns the conversion from internal/activity so
// the tray package stays cycle-free.
type RecentEntry struct {
	VideoID string
	Title   string
}

const recentSubmenuLimit = 5

// Controller is the long-lived handle to the tray menu. Owns the runtime
// context so menu click handlers can call WindowShow / Quit on the right app.
// onQuit, if set, runs from the Quit menu callback BEFORE wailsRuntime.Quit so
// the App can flip its quitting flag (see App.MarkQuitting) and OnBeforeClose
// can let the quit proceed instead of intercepting it. onStartServer /
// onStopServer drive the Bridge server toggle. recentDownloads supplies the
// submenu entries. state is the bridgestate Holder used for live updates.
type Controller struct {
	mu                sync.Mutex
	ctx               context.Context
	start             func()
	end               func()
	onQuit            func()
	onStartServer     func() error
	onStopServer      func() error
	onCheckForUpdates func() error
	recentDownloads   func() []RecentEntry
	state             *bridgestate.Holder
	unsubState        func()
	pulseRunning      atomic.Bool
}

// pulseInterval is the cadence of the downloading-state pulse: long enough
// that the alternation reads as a heartbeat rather than a flicker.
const pulseInterval = 600 * time.Millisecond

// Labels for the update menu item. Default is the macOS "..." convention for
// "this opens a dialog" since the click goes through to the in-window banner.
const (
	updateRowDefaultLabel = "Check for updates..."
	updateRowInstallLabel = "Install update..."
)

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

// SetState wires the bridgestate Holder used to render the live state row and
// the server-toggle checkbox. Call before Register so onReady can subscribe.
func (c *Controller) SetState(h *bridgestate.Holder) {
	c.mu.Lock()
	c.state = h
	c.mu.Unlock()
}

// SetOnStartServer installs the callback the server-toggle uses to bring the
// HTTP bridge back up. Returning an error logs but otherwise no-ops.
func (c *Controller) SetOnStartServer(fn func() error) {
	c.mu.Lock()
	c.onStartServer = fn
	c.mu.Unlock()
}

// SetOnStopServer installs the callback the server-toggle uses to take the
// HTTP bridge down.
func (c *Controller) SetOnStopServer(fn func() error) {
	c.mu.Lock()
	c.onStopServer = fn
	c.mu.Unlock()
}

// SetOnCheckForUpdates installs the callback the update menu item invokes
// when no update is currently stashed. main.go wires it to App.CheckForUpdates
// so the tray can trigger a one-shot manifest fetch without dragging the App
// type into this package.
func (c *Controller) SetOnCheckForUpdates(fn func() error) {
	c.mu.Lock()
	c.onCheckForUpdates = fn
	c.mu.Unlock()
}

// SetRecentDownloads installs a callback that returns the most recent audio
// download entries (newest first). The tray reads it lazily each time the
// menu opens; main.go converts from internal/activity to avoid a tray ->
// activity import cycle.
func (c *Controller) SetRecentDownloads(fn func() []RecentEntry) {
	c.mu.Lock()
	c.recentDownloads = fn
	c.mu.Unlock()
}

func (c *Controller) quitCallback() func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.onQuit
}

func (c *Controller) startServerCallback() func() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.onStartServer
}

func (c *Controller) stopServerCallback() func() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.onStopServer
}

func (c *Controller) recentDownloadsCallback() func() []RecentEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.recentDownloads
}

func (c *Controller) checkForUpdatesCallback() func() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.onCheckForUpdates
}

func (c *Controller) stateHolder() *bridgestate.Holder {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
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
	c.mu.Lock()
	unsub := c.unsubState
	c.unsubState = nil
	c.mu.Unlock()
	if unsub != nil {
		unsub()
	}
	if c.end != nil {
		c.end()
	}
}

func (c *Controller) onReady() {
	isMac := runtime.GOOS == "darwin"
	initial := bridgestate.State{Server: bridgestate.ServerStopped, Download: bridgestate.DownloadIdle}
	if holder := c.stateHolder(); holder != nil {
		initial = holder.Snapshot()
	}
	applyTrayIcon(initial, isMac)
	systray.SetTooltip("Composer Bridge")

	mHeader := systray.AddMenuItem("Composer Bridge", "")
	mHeader.Disable()
	mState := systray.AddMenuItem("Idle", "")
	mState.Disable()
	systray.AddSeparator()

	mShow := systray.AddMenuItem("Open Composer Bridge", "Show the window")
	applyItemIcon(mShow, icons.MenuWindow)

	mRecent := systray.AddMenuItem("Recent Downloads", "Last audio downloads")
	applyItemIcon(mRecent, icons.MenuClock)
	c.populateRecentSubmenu(mRecent)

	mUpdate := systray.AddMenuItem(updateRowDefaultLabel, "Check for a newer release")
	applyItemIcon(mUpdate, icons.MenuCloudDownload)

	systray.AddSeparator()

	mServer := systray.AddMenuItemCheckbox("Bridge server", "Toggle the local HTTP bridge", false)
	applyItemIcon(mServer, icons.MenuPower)

	systray.AddSeparator()

	mSettings := systray.AddMenuItem("Settings...", "Open settings")
	applyItemIcon(mSettings, icons.MenuGear)

	mQuit := systray.AddMenuItem("Quit", "Stop the bridge and quit")
	applyItemIcon(mQuit, icons.MenuX)

	// AppKit invokes click handlers on the main thread inside its menu
	// tracking loop. Any work that touches systray's AppKit-backed APIs
	// (SetTitle, SetIcon) must run off the main thread, so each handler
	// hands off to a fresh goroutine.
	mShow.Click(func() { go c.showWindow() })
	mSettings.Click(func() { go c.showWindow() })
	mServer.Click(func() { go c.toggleServer(mServer) })
	mUpdate.Click(func() { go c.handleUpdateMenuClick() })
	mQuit.Click(func() { go c.quitApp() })

	if holder := c.stateHolder(); holder != nil {
		snap := holder.Snapshot()
		applyState(mState, mServer, snap)
		applyUpdateRow(mUpdate, snap)
		c.maybeStartPulse(snap, isMac)
		unsub := holder.OnChange(func(s bridgestate.State) {
			// systray's SetTitle/Check/SetTemplateIcon internally call
			// performSelectorOnMainThread:waitUntilDone:YES. If this
			// callback fires from the main thread (e.g. inside a click
			// handler) the main thread tries to dispatch to itself and
			// deadlocks/crashes during AppKit menu tracking. Run the
			// mutation in a goroutine so we are not on main.
			go func() {
				applyState(mState, mServer, s)
				applyUpdateRow(mUpdate, s)
				applyTrayIcon(s, isMac)
				c.maybeStartPulse(s, isMac)
			}()
		})
		c.mu.Lock()
		c.unsubState = unsub
		c.mu.Unlock()
	}

	// energye/systray does not auto-attach the NSMenu to the status item.
	// Wire left-click to restore the window and right-click to open the menu
	// (the library's ShowMenu only works inside the OnRClick callback on macOS).
	systray.SetOnClick(func(_ systray.IMenu) { go c.showWindow() })
	systray.SetOnRClick(func(m systray.IMenu) { _ = m.ShowMenu() })
}

func (c *Controller) populateRecentSubmenu(parent *systray.MenuItem) {
	fn := c.recentDownloadsCallback()
	if fn == nil {
		empty := parent.AddSubMenuItem("No recent downloads", "")
		empty.Disable()
		return
	}
	entries := fn()
	if len(entries) > recentSubmenuLimit {
		entries = entries[:recentSubmenuLimit]
	}
	if len(entries) == 0 {
		empty := parent.AddSubMenuItem("No recent downloads", "")
		empty.Disable()
		return
	}
	for _, e := range entries {
		label := e.Title
		if label == "" {
			label = e.VideoID
		}
		item := parent.AddSubMenuItem(label, e.VideoID)
		applyItemIcon(item, icons.MenuDot)
		item.Disable()
	}
}

func (c *Controller) toggleServer(item *systray.MenuItem) {
	holder := c.stateHolder()
	running := false
	if holder != nil {
		running = holder.Snapshot().Server == bridgestate.ServerRunning
	} else {
		running = item.Checked()
	}
	if running {
		if fn := c.stopServerCallback(); fn != nil {
			if err := fn(); err != nil {
				slog.Warn("tray stop server failed", "err", err)
			}
		}
		return
	}
	if fn := c.startServerCallback(); fn != nil {
		if err := fn(); err != nil {
			slog.Warn("tray start server failed", "err", err)
		}
	}
}

// handleUpdateMenuClick branches on whether an update is currently stashed.
// When one is pending the window opens so the in-window UpdateBanner can take
// over the install flow; otherwise the registered OnCheckForUpdates callback
// runs a fresh manifest fetch. A discovered update flips bridgestate, which
// in turn relabels this same menu item on the next OnChange tick.
func (c *Controller) handleUpdateMenuClick() {
	if holder := c.stateHolder(); holder != nil && holder.Snapshot().UpdatePending {
		c.showWindow()
		return
	}
	fn := c.checkForUpdatesCallback()
	if fn == nil {
		return
	}
	if err := fn(); err != nil {
		slog.Warn("tray check for updates failed", "err", err)
	}
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

// pickTrayIcon maps a bridgestate snapshot to the matching tray-bar variant.
// Stopped/Starting/Stopping all resolve to the dimmed variant regardless of
// download state. While running, an active download wins over a sticky
// LastError so the badge reflects what's happening NOW. UpdatePending sits
// below those: a pending release is sticky-but-not-urgent and must never
// mask an active download or fresh error. Returns the mac template bytes
// plus the colored default bytes; callers pick which to push based on
// platform.
func pickTrayIcon(s bridgestate.State, isMac bool) (template, regular []byte) {
	var tmpl, reg []byte
	switch {
	case s.Server != bridgestate.ServerRunning:
		tmpl, reg = icons.MacStopped, icons.DefaultStopped
	case s.Download == bridgestate.DownloadActive:
		tmpl, reg = icons.MacDownloading, icons.DefaultDownloading
	case s.LastError != "" && s.Download == bridgestate.DownloadIdle:
		tmpl, reg = icons.MacError, icons.DefaultError
	case s.UpdatePending:
		tmpl, reg = icons.MacUpdate, icons.DefaultUpdate
	default:
		tmpl, reg = icons.MacIdle, icons.DefaultIdle
	}
	if isMac {
		return tmpl, tmpl
	}
	return nil, reg
}

// applyTrayIcon pushes the picked variant into systray using the correct API
// for the platform: template mode on macOS for system tint, plain SetIcon
// everywhere else.
func applyTrayIcon(s bridgestate.State, isMac bool) {
	tmpl, reg := pickTrayIcon(s, isMac)
	if isMac {
		systray.SetTemplateIcon(tmpl, tmpl)
		return
	}
	systray.SetIcon(reg)
}

// maybeStartPulse kicks the downloading-state pulse goroutine if the snapshot
// shows an active download and no pulse is already running. The goroutine
// self-exits when the download leaves DownloadActive, so callers do not need
// to track or signal it on the way out.
func (c *Controller) maybeStartPulse(s bridgestate.State, isMac bool) {
	if !isPulseState(s) {
		return
	}
	if !c.pulseRunning.CompareAndSwap(false, true) {
		return
	}
	go c.runPulse(isMac)
}

// runPulse alternates between the full and dim downloading variants every
// pulseInterval. It re-reads the live state on every tick instead of trusting
// the snapshot it was launched with, so a state change that races with the
// ticker cleanly stops the pulse and pushes the correct non-pulsing icon
// before the goroutine exits.
func (c *Controller) runPulse(isMac bool) {
	defer c.pulseRunning.Store(false)
	tick := time.NewTicker(pulseInterval)
	defer tick.Stop()
	dim := true
	for range tick.C {
		holder := c.stateHolder()
		if holder == nil {
			return
		}
		s := holder.Snapshot()
		if !isPulseState(s) {
			applyTrayIcon(s, isMac)
			return
		}
		setDownloadingFrame(dim, isMac)
		dim = !dim
	}
}

func isPulseState(s bridgestate.State) bool {
	return s.Server == bridgestate.ServerRunning && s.Download == bridgestate.DownloadActive
}

func setDownloadingFrame(dim bool, isMac bool) {
	var tmpl, reg []byte
	if dim {
		tmpl, reg = icons.MacDownloadingDim, icons.DefaultDownloadingDim
	} else {
		tmpl, reg = icons.MacDownloading, icons.DefaultDownloading
	}
	if isMac {
		systray.SetTemplateIcon(tmpl, tmpl)
		return
	}
	systray.SetIcon(reg)
}

// applyItemIcon picks SetTemplateIcon on macOS so the OS tints menu icons
// per appearance, and SetIcon elsewhere so other platforms render the
// fixed-color PNG directly.
func applyItemIcon(item *systray.MenuItem, data []byte) {
	if len(data) == 0 {
		return
	}
	if runtime.GOOS == "darwin" {
		item.SetTemplateIcon(data, data)
		return
	}
	item.SetIcon(data)
}

// applyState pushes the latest bridgestate snapshot into the menu: refreshes
// the state title and syncs the server-toggle's checkbox.
func applyState(stateItem, serverItem *systray.MenuItem, s bridgestate.State) {
	stateItem.SetTitle(renderStateTitle(s))
	if s.Server == bridgestate.ServerRunning {
		serverItem.Check()
	} else {
		serverItem.Uncheck()
	}
}

// applyUpdateRow swaps the update row between "Check for updates..." (idle)
// and "Install update..." (a release is stashed). The row is always visible;
// the label tells the user which click action they are about to trigger.
func applyUpdateRow(item *systray.MenuItem, s bridgestate.State) {
	if s.UpdatePending {
		item.SetTitle(updateRowInstallLabel)
		return
	}
	item.SetTitle(updateRowDefaultLabel)
}

func renderStateTitle(s bridgestate.State) string {
	if s.Server == bridgestate.ServerStopped {
		return "Server stopped"
	}
	if s.Server == bridgestate.ServerStarting {
		return "Starting..."
	}
	if s.Server == bridgestate.ServerStopping {
		return "Stopping..."
	}
	if s.Download == bridgestate.DownloadActive && s.DownloadVideoID != "" {
		return fmt.Sprintf("Downloading %s", s.DownloadVideoID)
	}
	return "Idle"
}
