// platformInjectMiddleware on the Go side stamps the served index.html with
// `<script>window.__platform="darwin|windows|linux";</script>` before any
// other JS runs. The value is Go's runtime.GOOS, baked into the binary at
// `wails build` time, so it is the ground truth for the host OS.
//
// In tests / browser previews where the script is absent, window.__platform
// is undefined and isMacOS resolves to false. No test asserts the macOS
// branch, so this is a safe default.

declare global {
  interface Window {
    __platform?: "darwin" | "windows" | "linux";
  }
}

export const isMacOS: boolean =
  typeof window !== "undefined" && window.__platform === "darwin";
