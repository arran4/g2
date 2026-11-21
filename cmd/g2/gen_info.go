package main

type GenerationInfo struct {
	Profiler        *Profiler
	Args            []string
	RepositoriesXML string
	FastGit         bool
	RecentDuration  string
	StrictLinks     bool
	OutDir          string      `json:"-"`
	Ledger          *LinkLedger `json:"-"`
	LinkReport      *LinkIntegrityReport
	// Placeholders for later:
	TimeTaken      string
	MemoryConsumed string
	DiskConsumed   string
}
