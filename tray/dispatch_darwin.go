//go:build darwin

package tray

/*
#cgo LDFLAGS: -framework Foundation
#include <dispatch/dispatch.h>

extern void trayStartBridge(void);
extern void trayDrainMain(void);

static void _tray_start_helper(void *ctx) {
	trayStartBridge();
}

static void _tray_drain_helper(void *ctx) {
	trayDrainMain();
}

static void trayDispatchStartToMain(void) {
	dispatch_async_f(dispatch_get_main_queue(), NULL, _tray_start_helper);
}

static void trayDispatchDrainToMain(void) {
	dispatch_async_f(dispatch_get_main_queue(), NULL, _tray_drain_helper);
}
*/
import "C"

import "sync"

// pendingStart holds the function dispatchStart wants the main-queue helper to
// run. We use a package-level slot because the cgo callback bridge cannot carry
// a Go closure across the boundary.
var pendingStart struct {
	sync.Mutex
	fn func()
}

// pendingMain is a FIFO queue of closures dispatchMain wants executed on the
// macOS main thread. Each dispatchMain call enqueues one closure and posts
// one drain to the main queue, so closures run in submission order.
var pendingMain struct {
	sync.Mutex
	queue []func()
}

//export trayStartBridge
func trayStartBridge() {
	pendingStart.Lock()
	fn := pendingStart.fn
	pendingStart.fn = nil
	pendingStart.Unlock()
	if fn != nil {
		fn()
	}
}

//export trayDrainMain
func trayDrainMain() {
	pendingMain.Lock()
	if len(pendingMain.queue) == 0 {
		pendingMain.Unlock()
		return
	}
	fn := pendingMain.queue[0]
	pendingMain.queue = pendingMain.queue[1:]
	pendingMain.Unlock()
	if fn != nil {
		fn()
	}
}

// dispatchStart posts fn onto the macOS main dispatch queue. Required because
// energye/systray's nativeStart calls AppKit (NSStatusBar, NSWindow) which
// must run on thread 0; Wails fires OnStartup on a goroutine so a direct call
// would crash with "NSWindow should only be instantiated on the main thread".
func dispatchStart(fn func()) {
	pendingStart.Lock()
	pendingStart.fn = fn
	pendingStart.Unlock()
	C.trayDispatchStartToMain()
}

// dispatchMain posts fn onto the macOS main dispatch queue. The tray menu
// click handlers fire on systray's own goroutine; AppKit calls (SetTitle,
// SetTemplateIcon, Check/Uncheck) must run on thread 0 or crash, so any
// menu/icon mutation triggered from a tray click or from a holder OnChange
// callback dispatched off the main thread must go through this helper.
func dispatchMain(fn func()) {
	pendingMain.Lock()
	pendingMain.queue = append(pendingMain.queue, fn)
	pendingMain.Unlock()
	C.trayDispatchDrainToMain()
}
