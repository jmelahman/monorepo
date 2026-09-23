//go:build windows

package server

// notifyResize has no SIGWINCH equivalent on Windows; the attach loop
// sends the initial size only. The nil channel never fires.
func notifyResize() (<-chan struct{}, func()) {
	return nil, func() {}
}
