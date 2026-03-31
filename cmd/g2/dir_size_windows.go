//go:build windows
// +build windows

package main

import (
	"golang.org/x/sys/windows"
)

func getFreeSpace(path string) (uint64, error) {
	var freeBytesAvailable uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64

	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	err = windows.GetDiskFreeSpaceEx(ptr, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes)
	if err != nil {
		return 0, err
	}

	return freeBytesAvailable, nil
}
