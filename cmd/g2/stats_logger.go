package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

func logStats(action string, repoName string, duration time.Duration, repoPath string, nodeCount int, hasNodeCount bool) {
	freeSpace, err := getFreeSpace(repoPath)
	appFreeSpace, appErr := getFreeSpace(".")
	tmpFreeSpace, tmpErr := getFreeSpace(os.TempDir())
	freeMem, memErr := getFreeMemory()
	procMem := getProcessMemUsage()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	var parts []string
	var vals []interface{}

	parts = append(parts, fmt.Sprintf("[DONE] Finished %s repository %%s in %%s.", action))
	vals = append(vals, repoName, duration)

	if err == nil {
		if appErr == nil {
			if tmpErr == nil {
				parts = append(parts, "Free space (Repo/App/Tmp): %.2f/%.2f/%.2f MB.")
				vals = append(vals, float64(freeSpace)/(1024*1024), float64(appFreeSpace)/(1024*1024), float64(tmpFreeSpace)/(1024*1024))
			} else {
				parts = append(parts, "Free space (Repo/App): %.2f/%.2f MB.")
				vals = append(vals, float64(freeSpace)/(1024*1024), float64(appFreeSpace)/(1024*1024))
			}
		} else {
			parts = append(parts, "Free space: %.2f MB.")
			vals = append(vals, float64(freeSpace)/(1024*1024))
		}
	}

	if memErr == nil {
		parts = append(parts, "Free memory: %.2f MB.")
		vals = append(vals, float64(freeMem)/(1024*1024))
	}

	parts = append(parts, "Process Memory: %.2f MB. Go Alloc: %.2f MB.")
	vals = append(vals, float64(procMem)/(1024*1024), float64(memStats.Alloc)/(1024*1024))

	if hasNodeCount {
		parts = append(parts, "Nodes: %d")
		vals = append(vals, nodeCount)
	}

	log.Printf(strings.Join(parts, " "), vals...)
}

func logFetchStats(repoName string, checkoutTime time.Duration, repoPath string) {
	logStats("fetching", repoName, checkoutTime, repoPath, 0, false)
}

func logParseStats(repoName string, processTime time.Duration, repoPath string, nodeCount int) {
	logStats("parsing", repoName, processTime, repoPath, nodeCount, true)
}
