package main

import "time"

type GenerationInfo struct {
	Profiler        *Profiler
	Args            []string
	RepositoriesXML string
	FastGit         bool
	RecentDuration  string
	FetchParseWall  time.Duration
	CheckoutWorker  time.Duration
	ParseWorker     time.Duration
	DisableStatsJSON bool
	NowFn           func() time.Time
	// Finalized metrics populated at the end of site generation:
	TimeTaken      string
	MemoryConsumed string
	DiskConsumed   string
	SiteStats      *SiteStatsReport
}
