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
			if err := resolver.addMaster(masterName, policy); err != nil {
				_, explicitlyConfigured := policy.ExplicitRepos[masterName]
				if policy.Mode == CacheModeStrict || policy.ReposConfPath != "" || explicitlyConfigured {
					return nil, err
				}
				resolver.missingMasters = append(resolver.missingMasters, masterName)
			}
		}
	}

	return resolver, nil
}

func (r *EclassResolver) addMaster(masterName string, policy *CachePolicy) error {
	if location, explicit := policy.ExplicitRepos[masterName]; explicit {
		if _, err := os.Stat(location); err != nil {
			return fmt.Errorf("accessing explicitly configured master repository %q at %q: %w", masterName, location, err)
		}
		r.repos = append(r.repos, MasterRepo{Name: masterName, Path: location, FS: os.DirFS(location)})
		return nil
	}

	reposConfPath := policy.ReposConfPath
	if reposConfPath == "" {
		if policy.Mode == CacheModeCI {
			return fmt.Errorf("master repository %q was not explicitly supplied in CI mode", masterName)
		}
		reposConfPath = "/etc/portage/repos.conf"
	}
	repoInfo, err := ResolveRepo(masterName, reposConfPath)
	if err != nil {
		return fmt.Errorf("resolving master repository %q from %q: %w", masterName, reposConfPath, err)
	}
	if repoInfo == nil || repoInfo.Location == "" {
		return fmt.Errorf("configured master repository %q from %q has no location", masterName, reposConfPath)
	}
	if _, err := os.Stat(repoInfo.Location); err != nil {
		return fmt.Errorf("accessing master repository %q at %q: %w", masterName, repoInfo.Location, err)
	}
	r.repos = append(r.repos, MasterRepo{Name: masterName, Path: repoInfo.Location, FS: os.DirFS(repoInfo.Location)})
	return nil
}
