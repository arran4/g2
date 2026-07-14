//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func setupResizeHandler(resizeChan chan<- struct{}) func() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-sigChan:
				select {
				case resizeChan <- struct{}{}:
				default:
					// If the channel is full (e.g. render is slow), we don't block.
					// A resize event just triggers a re-render.
				}
			case <-done:
				signal.Stop(sigChan)
				return
			}
		}
	}()
	return func() {
		close(done)
	}
}
