// Package bridge wraps the HTTP server lifecycle so it can be started and
// stopped at runtime (Settings toggle, tray menu, OnShutdown). State
// transitions are written to a bridgestate.Holder so the frontend, tray,
// and any other observer see them in one place.
package bridge

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/better-lyrics/composer-bridge/internal/bridgestate"
)

const shutdownTimeout = 5 * time.Second

// Bridge owns an *http.Server and the listener it is bound to. Start and
// Stop are safe to call concurrently; the underlying server is reused
// across stop/start cycles.
type Bridge struct {
	holder *bridgestate.Holder
	srv    *http.Server
	mu     sync.Mutex
	ln     net.Listener
	port   int
}

// New returns a Bridge that will broadcast lifecycle transitions through
// holder. The srv handler and timeouts are the caller's responsibility.
func New(holder *bridgestate.Holder, srv *http.Server) *Bridge {
	return &Bridge{holder: holder, srv: srv}
}

// Start binds 127.0.0.1:preferred (0 = ephemeral) and begins serving in a
// background goroutine. Returns an error if the bridge is already running
// or the port is in use; in either error case the server status flips back
// to stopped.
func (b *Bridge) Start(preferred int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ln != nil {
		return errors.New("bridge: already started")
	}
	b.holder.SetServer(bridgestate.ServerStarting)
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred))
	if err != nil {
		b.holder.SetServer(bridgestate.ServerStopped)
		return fmt.Errorf("bridge listen: %w", err)
	}
	b.ln = ln
	b.port = ln.Addr().(*net.TCPAddr).Port
	go func() { _ = b.srv.Serve(ln) }()
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
	b.mu.Unlock()
	b.holder.SetServer(bridgestate.ServerStopping)
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err := srv.Shutdown(ctx)
	b.holder.SetServer(bridgestate.ServerStopped)
	return err
}

// Port returns the port currently bound, or 0 when the bridge is stopped.
func (b *Bridge) Port() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.port
}
