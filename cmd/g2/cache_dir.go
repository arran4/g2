//go:build !root

package main

import (
	"os"
)

func getCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "/var/cache"
	}
	return cacheDir
}
