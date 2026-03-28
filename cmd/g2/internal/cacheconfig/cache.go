//go:build !root

package cacheconfig

import "os"

func GetCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "/var/cache"
	}
	return cacheDir
}
