//go:build windows

package main

func setupResizeHandler(resizeChan chan<- struct{}) func() {
	// No SIGWINCH on Windows. The terminal resize events are usually
	// handled differently, but for now we just do nothing on Windows.
	return func() {}
}
