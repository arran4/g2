//go:build windows

package main

func setupResizeHandler(resizeChan chan<- struct{}) {
	// No SIGWINCH on Windows. The terminal resize events are usually
	// handled differently, but for now we just do nothing on Windows.
}
