// Wails uses a platform-native webview on every host (WKWebView on macOS,
// WebView2 on Windows, WebKitGTK on Linux), so navigator.userAgent always
// reflects the host OS. We evaluate this at module load: zero IPC, zero
// layout flash on first paint, and tree-shakeable when bundled.
//
// Why not Wails' Environment() runtime call? It returns a Promise, which
// forces an async effect and a render with the wrong inset on the first
// frame. The host detection is fundamental enough that we want it sync.

export const isMacOS: boolean =
  typeof navigator !== "undefined" && /Macintosh|Mac OS X/.test(navigator.userAgent);
