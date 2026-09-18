package g2

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MasterRepo represents a repository and its filesystem for eclass resolution.
type MasterRepo struct {
	Name string
	Path string
	FS   fs.FS
}

// EclassResolver resolves eclasses across the current repository and its masters.
type EclassResolver struct {
	repos          []MasterRepo
	MissingContext bool // True if one or more masters could not be resolved in CI mode
}

// NewEclassResolver initializes a resolver using the provided repos list.
// The list should be ordered by resolution precedence (e.g. primary overlay first, then its masters in order).
func NewEclassResolver(repos []MasterRepo) *EclassResolver {
	return &EclassResolver{repos: repos}
}

// Resolve searches for the eclass in the configured repositories.
// Returns the file content, the repo name it was found in, and an error if not found.
func (r *EclassResolver) Resolve(eclassName string) ([]byte, string, error) {
	eclassFile := filepath.ToSlash(filepath.Join("eclass", eclassName+".eclass"))
	for _, repo := range r.repos {
		content, err := fs.ReadFile(repo.FS, eclassFile)
		if err == nil {
			return content, repo.Name, nil
		}
	}
	return nil, "", fmt.Errorf("eclass %s not found in any repository", eclassName)
}

// BuildEclassResolver constructs an EclassResolver from the given repository directory and CachePolicy.
func BuildEclassResolver(cfs CacheFS, repoDir string, policy *CachePolicy) (*EclassResolver, error) {
	resolver := &EclassResolver{}

	// Primary repo always comes first
	resolver.repos = append(resolver.repos, MasterRepo{
		Name: "primary",
		Path: repoDir,
		FS:   cfs,
	})

	var lc *LayoutConf
	if f, err := cfs.Open("metadata/layout.conf"); err == nil {
		lc, _ = ParseLayoutConfFromReader(f)
		_ = f.Close()
	} else if f, err := os.Open(filepath.ToSlash(filepath.Join(repoDir, "metadata", "layout.conf"))); err == nil {
		lc, _ = ParseLayoutConfFromReader(f)
		_ = f.Close()
	}

	if lc != nil {
		masters := lc.Masters()
		for _, masterName := range masters {
			resolved := false
			// First check if it was provided explicitly
			if loc, ok := policy.ExplicitRepos[masterName]; ok {
				if _, statErr := os.Stat(loc); statErr == nil {
					resolver.repos = append(resolver.repos, MasterRepo{
						Name: masterName,
						Path: loc,
						FS:   os.DirFS(loc),
					})
					resolved = true
				}
			}

			if !resolved {
				// Try to resolve using repos.conf (either explicit path or system default)
				reposConfPath := policy.ReposConfPath
				if reposConfPath == "" {
					reposConfPath = "/etc/portage/repos.conf"
				}
				repoInfo, err := ResolveRepo(masterName, reposConfPath)

				if err == nil && repoInfo != nil && repoInfo.Location != "" {
					if _, statErr := os.Stat(repoInfo.Location); statErr == nil {
						resolver.repos = append(resolver.repos, MasterRepo{
							Name: masterName,
							Path: repoInfo.Location,
							FS:   os.DirFS(repoInfo.Location),
						})
						resolved = true
					}
				}
			}

			// If we get here, we failed to resolve the master.
			if !resolved {
				if policy.Mode == CacheModeStrict {
					return nil, fmt.Errorf("strict mode: required master repository %q could not be resolved", masterName)
				} else {
					resolver.MissingContext = true
				}
			}
		}
	}

	return resolver, nil
}
