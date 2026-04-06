package main

import (
	"log"
	"os"
	"runtime"
	"time"
)

func logFetchStats(repoName string, checkoutTime time.Duration, repoPath string) {
	freeSpace, err := getFreeSpace(repoPath)
	appFreeSpace, appErr := getFreeSpace(".")
	tmpFreeSpace, tmpErr := getFreeSpace(os.TempDir())
	freeMem, memErr := getFreeMemory()
	procMem := getProcessMemUsage()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	if err == nil && appErr == nil && tmpErr == nil && memErr == nil {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Free space (Repo/App/Tmp): %.2f/%.2f/%.2f MB. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(freeSpace)/(1024*1024),
			float64(appFreeSpace)/(1024*1024),
			float64(tmpFreeSpace)/(1024*1024),
			float64(freeMem)/(1024*1024),
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	} else if err == nil && appErr == nil && memErr == nil {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Free space (Repo/App): %.2f/%.2f MB. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(freeSpace)/(1024*1024),
			float64(appFreeSpace)/(1024*1024),
			float64(freeMem)/(1024*1024),
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	} else if err == nil && memErr == nil {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Free space: %.2f MB. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(freeSpace)/(1024*1024),
			float64(freeMem)/(1024*1024),
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	} else if err == nil && appErr == nil && tmpErr == nil {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Free space (Repo/App/Tmp): %.2f/%.2f/%.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(freeSpace)/(1024*1024),
			float64(appFreeSpace)/(1024*1024),
			float64(tmpFreeSpace)/(1024*1024),
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	} else if err == nil && appErr == nil {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Free space (Repo/App): %.2f/%.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(freeSpace)/(1024*1024),
			float64(appFreeSpace)/(1024*1024),
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	} else if err == nil {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Free space: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(freeSpace)/(1024*1024),
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	} else if memErr == nil {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(freeMem)/(1024*1024),
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	} else {
		log.Printf(
			"[DONE] Finished fetching repository %s in %s. Process Memory: %.2f MB. Go Alloc: %.2f MB",
			repoName,
			checkoutTime,
			float64(procMem)/(1024*1024),
			float64(memStats.Alloc)/(1024*1024),
		)
	}
}

func logParseStats(repoName string, processTime time.Duration, repoPath string, nodeCount int) {
	freeSpaceAfter, err := getFreeSpace(repoPath)
	appFreeSpaceAfter, appErrAfter := getFreeSpace(".")
	tmpFreeSpaceAfter, tmpErrAfter := getFreeSpace(os.TempDir())
	freeMemAfter, memErrAfter := getFreeMemory()
	procMemAfter := getProcessMemUsage()

	var memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsAfter)

	if err == nil && appErrAfter == nil && tmpErrAfter == nil && memErrAfter == nil {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Free space (Repo/App/Tmp): %.2f/%.2f/%.2f MB. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(freeSpaceAfter)/(1024*1024),
			float64(appFreeSpaceAfter)/(1024*1024),
			float64(tmpFreeSpaceAfter)/(1024*1024),
			float64(freeMemAfter)/(1024*1024),
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	} else if err == nil && appErrAfter == nil && memErrAfter == nil {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Free space (Repo/App): %.2f/%.2f MB. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(freeSpaceAfter)/(1024*1024),
			float64(appFreeSpaceAfter)/(1024*1024),
			float64(freeMemAfter)/(1024*1024),
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	} else if err == nil && memErrAfter == nil {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Free space: %.2f MB. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(freeSpaceAfter)/(1024*1024),
			float64(freeMemAfter)/(1024*1024),
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	} else if err == nil && appErrAfter == nil && tmpErrAfter == nil {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Free space (Repo/App/Tmp): %.2f/%.2f/%.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(freeSpaceAfter)/(1024*1024),
			float64(appFreeSpaceAfter)/(1024*1024),
			float64(tmpFreeSpaceAfter)/(1024*1024),
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	} else if err == nil && appErrAfter == nil {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Free space (Repo/App): %.2f/%.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(freeSpaceAfter)/(1024*1024),
			float64(appFreeSpaceAfter)/(1024*1024),
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	} else if err == nil {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Free space: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(freeSpaceAfter)/(1024*1024),
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	} else if memErrAfter == nil {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Free memory: %.2f MB. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(freeMemAfter)/(1024*1024),
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	} else {
		log.Printf(
			"[DONE] Finished parsing repository %s in %s. Process Memory: %.2f MB. Go Alloc: %.2f MB. Nodes: %d",
			repoName,
			processTime,
			float64(procMemAfter)/(1024*1024),
			float64(memStatsAfter.Alloc)/(1024*1024),
			nodeCount,
		)
	}
}
