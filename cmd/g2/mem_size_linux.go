//go:build linux
// +build linux

package main

import (
	"golang.org/x/sys/unix"
)

func getFreeMemory() (uint64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, err
	}
	return uint64(info.Freeram) * uint64(info.Unit), nil
}

func getUsedMemory() (uint64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, err
	}
	return uint64(info.Totalram-info.Freeram) * uint64(info.Unit), nil
}
