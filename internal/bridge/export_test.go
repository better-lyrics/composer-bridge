package bridge

import "time"

func SetShutdownTimeout(d time.Duration) (restore func()) {
	prev := shutdownTimeout
	shutdownTimeout = d
	return func() { shutdownTimeout = prev }
}
