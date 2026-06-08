//go:build darwin

package tray

/*
#cgo LDFLAGS: -framework Foundation
#include <dispatch/dispatch.h>

extern void trayStartBridge(void);

static void _tray_dispatch_helper(void *ctx) {
	trayStartBridge();
}

static void trayDispatchToMain(void) {
	dispatch_async_f(dispatch_get_main_queue(), NULL, _tray_dispatch_helper);
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

// dispatchStart posts fn onto the macOS main dispatch queue. Required because
// energye/systray's nativeStart calls AppKit (NSStatusBar, NSWindow) which
// must run on thread 0; Wails fires OnStartup on a goroutine so a direct call
// would crash with "NSWindow should only be instantiated on the main thread".
func dispatchStart(fn func()) {
	pendingStart.Lock()
	pendingStart.fn = fn
	pendingStart.Unlock()
	C.trayDispatchToMain()
}
