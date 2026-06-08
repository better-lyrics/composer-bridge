package server

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSelectPort_UsesConfiguredWhenFree(t *testing.T) {
	dir := t.TempDir()
	free := freePort(t)

	got, err := SelectPort(free, dir)
	if err != nil {
		t.Fatalf("SelectPort: %v", err)
	}
	defer got.Listener.Close()

	if got.Port != free {
		t.Errorf("Port: got %d, want %d", got.Port, free)
	}
	if got.Fallback {
		t.Error("Fallback: got true, want false")
	}
	assertPortFileMatches(t, dir, free)
}

func TestSelectPort_FallsBackWhenBusy(t *testing.T) {
	dir := t.TempDir()
	busy := freePort(t)

	hold, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(busy))
	if err != nil {
		t.Fatalf("hold: %v", err)
	}
	defer hold.Close()

	got, err := SelectPort(busy, dir)
	if err != nil {
		t.Fatalf("SelectPort: %v", err)
	}
	defer got.Listener.Close()

	if got.Port == busy {
		t.Errorf("Port: got %d, want different from busy %d", got.Port, busy)
	}
	if got.Port < 49152 || got.Port > 65535 {
		t.Errorf("Port: got %d, want ephemeral range 49152-65535", got.Port)
	}
	if !got.Fallback {
		t.Error("Fallback: got false, want true")
	}
	assertPortFileMatches(t, dir, got.Port)
}

func TestSelectPort_PortFileOverwritten(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "port.txt")
	if err := os.WriteFile(stale, []byte("12345"), 0o644); err != nil {
		t.Fatalf("seed stale port file: %v", err)
	}

	free := freePort(t)
	got, err := SelectPort(free, dir)
	if err != nil {
		t.Fatalf("SelectPort: %v", err)
	}
	defer got.Listener.Close()

	assertPortFileMatches(t, dir, free)
}

func TestSelectPort_CreatesDataDirIfMissing(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "nested", "data")

	free := freePort(t)
	got, err := SelectPort(free, dir)
	if err != nil {
		t.Fatalf("SelectPort: %v", err)
	}
	defer got.Listener.Close()

	assertPortFileMatches(t, dir, free)
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func assertPortFileMatches(t *testing.T, dir string, want int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "port.txt"))
	if err != nil {
		t.Fatalf("read port.txt: %v", err)
	}
	got, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("parse port.txt %q: %v", raw, err)
	}
	if got != want {
		t.Errorf("port.txt: got %d, want %d", got, want)
	}
}
