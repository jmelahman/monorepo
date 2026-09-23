//go:build !windows

package server

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyResize returns a channel that receives a tick whenever the
// controlling terminal changes size (SIGWINCH). Ticks coalesce: a burst of
// resizes while the attach loop is busy collapses into one. The returned
// stop function releases the signal handler.
func notifyResize() (<-chan struct{}, func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	ch := make(chan struct{}, 1)
	go func() {
		for range sig {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}()
	return ch, func() {
		signal.Stop(sig)
		close(sig)
	}
}
