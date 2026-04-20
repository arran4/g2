package main

type GenerationInfo struct {
	Profiler        *Profiler
	Args            []string
	RepositoriesXML string
	FastGit         bool
	RecentDuration  string
	// Placeholders for later:
	TimeTaken      string
	MemoryConsumed string
	DiskConsumed   string
}
