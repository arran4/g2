//go:build !linux && !windows
// +build !linux,!windows

package main

import (
	"errors"
)

func getFreeMemory() (uint64, error) {
	return 0, errors.New("not implemented on this platform")
}
