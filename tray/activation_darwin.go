//go:build darwin

package tray

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

// We use TransformProcessType instead of -[NSApplication setActivationPolicy:]
// because the Regular -> Accessory transition is unreliable on modern macOS
// (it silently no-ops when called from a foreground app). TransformProcessType
// is the documented Carbon Process Manager API that menubar apps (1Password
// mini, Bartender, etc.) use to flip Dock visibility at runtime.

static void trayBecomeAccessory(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		ProcessSerialNumber psn = { 0, kCurrentProcess };
		TransformProcessType(&psn, kProcessTransformToUIElementApplication);
	});
}

static void trayBecomeRegular(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		ProcessSerialNumber psn = { 0, kCurrentProcess };
		TransformProcessType(&psn, kProcessTransformToForegroundApplication);
		[NSApp activateIgnoringOtherApps:YES];
	});
}
*/
import "C"

// SetBackground switches the macOS app to accessory mode: the Dock icon
// disappears so the app reads as tray-only. Posts to the main queue, safe
// to call from any goroutine.
func SetBackground() {
	C.trayBecomeAccessory()
}

// SetForeground switches the macOS app back to regular mode: the Dock icon
// returns and the app is brought to the front. Pair with WindowShow.
func SetForeground() {
	C.trayBecomeRegular()
}
