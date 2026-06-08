//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// The app launches with LSUIElement=true so it starts as Accessory and has no
// Dock icon. Wails's AppDelegate.m unfortunately calls setActivationPolicy:
// Regular in applicationWillFinishLaunching, so OnStartup must call
// dockShow + WindowShow only after that override has run. The
// Accessory-to-Regular-to-Accessory cycle works reliably; the inverse (when
// the app launches as Regular) does not, which is why all our prior fixes
// failed.

static void dockShow(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
		[NSApp activateIgnoringOtherApps:YES];
	});
}

static void dockHide(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	});
}
*/
import "C"

// DockShow puts the app back in Regular activation mode: Dock icon visible,
// app present in Cmd+Tab, window focusable. Posts to the main queue.
func DockShow() {
	C.dockShow()
}

// DockHide drops the app to Accessory mode: no Dock icon, no Cmd+Tab entry,
// tray icon remains. Posts to the main queue.
func DockHide() {
	C.dockHide()
}
