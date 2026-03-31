//go:build !windows
// +build !windows

package main

import (
	"golang.org/x/sys/unix"
)

func getFreeSpace(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
