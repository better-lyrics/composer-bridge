// Package bridge wraps the HTTP server lifecycle so it can be started and
// stopped at runtime (Settings toggle, tray menu, OnShutdown). State
// transitions are written to a bridgestate.Holder so the frontend, tray,
// and any other observer see them in one place.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
)

var shutdownTimeout = 5 * time.Second

// handlerDrainTimeout bounds how long Stop waits for force-closed handlers to
// clean up (kill yt-dlp, remove a half-written auto-download) before returning.
const handlerDrainTimeout = 2 * time.Second

// Bridge owns the listener and the http.Server bound to it for a single
// run cycle. Because *http.Server cannot be reused after Shutdown, each
// Start builds a fresh server via the build constructor supplied to New.
// Start and Stop are safe to call concurrently.
type Bridge struct {
	holder   *bridgestate.Holder
	build    func() *http.Server
	mu       sync.Mutex
	srv      *http.Server
	ln       net.Listener
	port     int
	stopping bool
	handlers sync.WaitGroup
}

// New returns a Bridge that will broadcast lifecycle transitions through
// holder. build is invoked once per Start to construct a fresh
// *http.Server; it must return a server with handler and timeouts already
// configured.
func New(holder *bridgestate.Holder, build func() *http.Server) *Bridge {
	return &Bridge{holder: holder, build: build}
}

// Start binds 127.0.0.1:preferred (0 = ephemeral) and begins serving in a
// background goroutine. Returns an error if the bridge is already running,
// a Stop is in progress, or the port is in use; in the port-in-use case
// the server status flips back to stopped.
func (b *Bridge) Start(preferred int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stopping {
		return errors.New("bridge: stop in progress")
	}
	if b.ln != nil {
		return errors.New("bridge: already started")
	}
	b.holder.SetServer(bridgestate.ServerStarting)
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred))
	if err != nil {
		b.holder.SetServer(bridgestate.ServerStopped)
		return fmt.Errorf("bridge listen: %w", err)
	}
	srv := b.build()
	inner := srv.Handler
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.handlers.Add(1)
		defer b.handlers.Done()
		inner.ServeHTTP(w, r)
	})
	b.srv = srv
	b.ln = ln
	b.port = ln.Addr().(*net.TCPAddr).Port
	go func() {
		err := srv.Serve(ln)
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return
		}
		slog.Error("bridge serve", "err", err)
		// Spontaneous Serve failure: clear internal fields so the next Start
		// can succeed. Guard against racing Stop, which nulls these fields
		// itself and will hold b.mu while doing so.
		b.mu.Lock()
		if b.srv == srv {
			b.srv = nil
			b.ln = nil
			b.port = 0
		}
		b.mu.Unlock()
		b.holder.SetServer(bridgestate.ServerStopped)
	}()
	b.holder.SetServer(bridgestate.ServerRunning)
	return nil
}

// Stop gracefully shuts down the listener. Returns nil if the bridge is
// not currently running.
func (b *Bridge) Stop() error {
	b.mu.Lock()
	if b.ln == nil {
		b.mu.Unlock()
		return nil
	}
	srv := b.srv
	b.ln = nil
	b.srv = nil
	b.stopping = true
	b.mu.Unlock()
	b.holder.SetServer(bridgestate.ServerStopping)

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err := srv.Shutdown(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		// Long-lived /audio streams outlast the grace period; close them so the
		// server really stops and their request contexts cancel yt-dlp.
		err = srv.Close()
		b.waitForHandlers(handlerDrainTimeout)
	}

	b.mu.Lock()
	b.port = 0
	b.stopping = false
	b.mu.Unlock()
	b.holder.SetServer(bridgestate.ServerStopped)
	return err
}

// Port returns the port currently bound, or 0 when the bridge is stopped.
func (b *Bridge) Port() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.port
}

func (b *Bridge) waitForHandlers(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		b.handlers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("bridge stop: handlers still running after close", "timeout", timeout)
	}
}
