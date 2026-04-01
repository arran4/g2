1. **Goal**: Add `-include-gentoo` and `-include-guru` flags to `overlay site generate`.
2. **Changes**:
   - In `cmd/g2/site.go` around line 230 in `cmdOverlay`:
     - Add `includeGentoo := fs.Bool("include-gentoo", false, "Include the main gentoo repository in site generation")`
     - Add `includeGuru := fs.Bool("include-guru", false, "Include the guru repository in site generation")`
   - Currently, `cmdOverlay` only handles one repo synchronously. We need to introduce an `errgroup` (like in `cmdSiteRemote`) to fetch/parse the main overlay + gentoo (if flag) + guru (if flag) concurrently.
   - Refactor `cmdOverlay` to use `errgroup` and a slice of `SiteData` appended with a `sync.Mutex` (as mentioned in memory).
   - URLs: Gentoo: `https://github.com/gentoo-mirror/gentoo.git`, Guru: `https://github.com/gentoo-mirror/guru.git`.
   - Ensure temporary directories are cleaned up immediately inside the worker goroutine using `defer func() { _ = os.RemoveAll(repoPath) }()`, unless `-persistent-dir` is set.
   - Sort the resulting slice alphabetically by `RepoName` for deterministic ordering before `generateSite`.
   - Add a pre-commit step using `pre_commit_instructions`.
