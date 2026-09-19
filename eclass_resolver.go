package g2

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
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
	missingMasters []string
}

// NewEclassResolver initializes a resolver using the provided repos list.
// The list should be ordered by resolution precedence (e.g. primary overlay first, then its masters in order).
func NewEclassResolver(repos []MasterRepo) *EclassResolver {
	return &EclassResolver{repos: repos}
}

// MissingMasters reports the exact CI capabilities that are unavailable.
func (r *EclassResolver) MissingMasters() []string {
	return append([]string(nil), r.missingMasters...)
}

// Resolve searches for the eclass in the configured repositories.
// Returns the file content, the repo name it was found in, and an error if not found.
func (r *EclassResolver) Resolve(eclassName string) ([]byte, string, error) {
	content, repo, err := r.resolve(eclassName)
	if err != nil {
		return nil, "", err
	}
	return content, repo.Name, nil
}

func (r *EclassResolver) resolve(eclassName string) ([]byte, MasterRepo, error) {
	if eclassName == "" || strings.Contains(eclassName, "/") || eclassName == "." || eclassName == ".." {
		return nil, MasterRepo{}, fmt.Errorf("invalid eclass name %q", eclassName)
	}
	eclassFile := path.Join("eclass", eclassName+".eclass")
	for _, repo := range r.repos {
		content, err := fs.ReadFile(repo.FS, eclassFile)
		if err == nil {
			return content, repo, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, MasterRepo{}, fmt.Errorf("reading eclass %q from repository %q (%s): %w", eclassName, repo.Name, repo.Path, err)
		}
	}
	if len(r.missingMasters) > 0 {
		return nil, MasterRepo{}, fmt.Errorf("eclass %q was not found in available repositories; unavailable masters: %s", eclassName, strings.Join(r.missingMasters, ", "))
	}
	names := make([]string, 0, len(r.repos))
	for _, repo := range r.repos {
		names = append(names, repo.Name)
	}
	return nil, MasterRepo{}, fmt.Errorf("eclass %q was not found in repository search path %s", eclassName, strings.Join(names, ", "))
}

// BuildEclassResolver constructs an EclassResolver from the given repository directory and CachePolicy.
func BuildEclassResolver(cfs CacheFS, repoDir string, policy *CachePolicy) (*EclassResolver, error) {
	if policy == nil {
		policy = NewCachePolicy(CacheModeCI)
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if filepath.IsAbs(repoDir) {
		return nil, fmt.Errorf("repository directory must be relative to CacheFS root: %q", repoDir)
	}
	root := filepath.ToSlash(filepath.Clean(repoDir))
	primaryFS, err := fs.Sub(cfs, root)
	if err != nil {
		return nil, fmt.Errorf("opening primary repository %q: %w", repoDir, err)
	}
	resolver := &EclassResolver{}

	// Primary repo always comes first
	resolver.repos = append(resolver.repos, MasterRepo{
		Name: "primary",
		Path: root,
		FS:   primaryFS,
	})

	var lc *LayoutConf
	if f, err := primaryFS.Open("metadata/layout.conf"); err == nil {
		var parseErr error
		lc, parseErr = ParseLayoutConfFromReader(f)
		closeErr := f.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("parsing layout.conf for repository %q: %w", root, parseErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("closing layout.conf for repository %q: %w", root, closeErr)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("opening layout.conf for repository %q: %w", root, err)
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
				}
				resolver.missingMasters = append(resolver.missingMasters, masterName)
			}
		}
	}

	return resolver, nil
}
