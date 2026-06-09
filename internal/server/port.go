package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const (
	ephemeralMin = 49152
	ephemeralMax = 65535
)

type PortSelection struct {
	Port     int
	Listener net.Listener
	Fallback bool
}

func SelectPort(preferred int, dataDir string) (*PortSelection, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(preferred))
	if err == nil {
		port := listener.Addr().(*net.TCPAddr).Port
		if writeErr := WritePortFile(dataDir, port); writeErr != nil {
			listener.Close()
			return nil, writeErr
		}
		return &PortSelection{Port: port, Listener: listener, Fallback: false}, nil
	}
	if !isAddrInUse(err) {
		return nil, fmt.Errorf("listen on %d: %w", preferred, err)
	}

	listener, err = listenEphemeral()
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if writeErr := WritePortFile(dataDir, port); writeErr != nil {
		listener.Close()
		return nil, writeErr
	}
	return &PortSelection{Port: port, Listener: listener, Fallback: true}, nil
}

func listenEphemeral() (net.Listener, error) {
	for port := ephemeralMin; port <= ephemeralMax; port++ {
		l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err == nil {
			return l, nil
		}
		if !isAddrInUse(err) {
			return nil, fmt.Errorf("listen on %d: %w", port, err)
		}
	}
	return nil, errors.New("no free port in ephemeral range 49152-65535")
}

// WritePortFile atomically writes port to dataDir/port.txt. Composer's
// discovery hook and the README's recovery instructions both read this file
// when the preferred bridge port is busy and the server falls back to an
// ephemeral one.
func WritePortFile(dataDir string, port int) error {
	path := filepath.Join(dataDir, "port.txt")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(port)), 0o644); err != nil {
		return fmt.Errorf("write port file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename port file: %w", err)
	}
	return nil
}

func isAddrInUse(err error) bool {
	var sysErr *os.SyscallError
	if errors.As(err, &sysErr) {
		return errors.Is(sysErr.Err, syscall.EADDRINUSE)
	}
	return false
}
