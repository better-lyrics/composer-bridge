package main

import (
	"bytes"
	"net/http"
	goruntime "runtime"
	"strconv"
)

// htmlBufferingWriter captures the response from the inner asset handler so
// platformInjectMiddleware can rewrite the body before flushing to the real
// ResponseWriter. We don't use httptest.ResponseRecorder because that package
// is conceptually test-only and importing it in main feels off.
type htmlBufferingWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *htmlBufferingWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *htmlBufferingWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(b)
}

func (w *htmlBufferingWriter) WriteHeader(status int) {
	w.status = status
}

// platformInjectMiddleware stamps the served index.html with `window.__platform`
// set to Go's `runtime.GOOS`. The React app reads this synchronously, before
// any other JS runs, to render correct window-chrome insets on the first
// paint (the macOS title-bar inset is wasted space on Windows and Linux).
//
// Why this and not navigator.userAgent: UA strings can change across webview
// versions. runtime.GOOS is a Go compile-time constant baked into the binary
// by `wails build -platform <os>/<arch>`, so it is the ground truth and is
// never wrong for the host the binary is running on.
//
// Why this and not Environment() from the Wails JS runtime: Environment is a
// Promise (IPC roundtrip). Reading it from React forces a first-frame layout
// flash or an awaited delay before mount. Injecting at HTML-serve time means
// the variable is set before any JS in the document runs.
//
// We only buffer the response when the path is the root index.html. Static
// asset requests (JS bundles, CSS, images) pass straight through.
func platformInjectMiddleware(next http.Handler) http.Handler {
	script := []byte(`<script>window.__platform=` + strconv.Quote(goruntime.GOOS) + `;</script>`)
	closingHead := []byte("</head>")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			next.ServeHTTP(w, r)
			return
		}

		buf := &htmlBufferingWriter{}
		next.ServeHTTP(buf, r)

		body := bytes.Replace(buf.body.Bytes(), closingHead, append(script, closingHead...), 1)
		for k, vv := range buf.Header() {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		status := buf.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	})
}
