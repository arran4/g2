package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func getFileModTime(repoDir string, relPath string, fast bool, preOpenedRepo *git.Repository) time.Time {
	var repo *git.Repository
	var err error
	if preOpenedRepo != nil {
		repo = preOpenedRepo
	} else {
		repo, err = git.PlainOpen(repoDir)
	}
	if err == nil && repo != nil {
		head, err := repo.Head()
		if err == nil {
			if fast {
				// Fast method: uses FileName filter.
				// Note: this may occasionally miss the exact commit in go-git if there are renames
				// or complex histories, but it runs significantly faster (O(1) lookup effectively).
				commitIter, err := repo.Log(&git.LogOptions{
					From:     head.Hash(),
					FileName: &relPath,
				})
				if err == nil {
					commit, err := commitIter.Next()
					if err == nil && commit != nil {
						return commit.Author.When
					}
				}
			} else {
				// Reliable method: Walks the commit tree and compares file hashes with parents.
				// Note: Reliable over speed, but performs an O(N) traversal which can be slow
				// on very large repositories. Switch to fast=true if performance is critical.
				commitIter, err := repo.Log(&git.LogOptions{From: head.Hash()})
				if err == nil {
					var modTime time.Time
					err = commitIter.ForEach(func(c *object.Commit) error {
						file, err := c.File(relPath)
						if err != nil {
							return nil // file not found in this commit
						}

						if c.NumParents() == 0 {
							modTime = c.Author.When
							return fmt.Errorf("found")
						}

						foundSame := false
						for i := 0; i < c.NumParents(); i++ {
							parent, err := c.Parent(i)
							if err != nil {
								continue
							}
							parentFile, err := parent.File(relPath)
							if err == nil && parentFile.Hash == file.Hash {
								foundSame = true
								break
							}
						}

						if !foundSame {
							modTime = c.Author.When
							return fmt.Errorf("found")
						}

						return nil
					})

					if err != nil && err.Error() == "found" {
						return modTime
					}
				}
			}
		}
	}

	info, err := os.Stat(filepath.Join(repoDir, relPath))
	if err == nil {
		return info.ModTime()
	}
	return time.Now()
}

func getGitOriginURL(repoDir string, preOpenedRepo *git.Repository) (string, error) {
	if preOpenedRepo != nil {
		remote, err := preOpenedRepo.Remote("origin")
		if err == nil && remote != nil {
			if len(remote.Config().URLs) > 0 {
				return remote.Config().URLs[0], nil
			}
		}
		return "", fmt.Errorf("remote.origin.url not found in pre-opened repository")
	}

	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting remote origin url: %w", err)
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("remote.origin.url is empty")
	}
	return url, nil
}

func getFileCommit(repoDir string, filepath string, preOpenedRepo *git.Repository) (string, error) {
	if preOpenedRepo != nil {
		head, err := preOpenedRepo.Head()
		if err == nil {
			commitIter, err := preOpenedRepo.Log(&git.LogOptions{
				From:     head.Hash(),
				FileName: &filepath,
			})
			if err == nil {
				commit, err := commitIter.Next()
				if err == nil && commit != nil {
					return commit.Hash.String(), nil
				}
			}
		}
		return "", fmt.Errorf("file commit not found in pre-opened repository")
	}

	cmd := exec.Command("git", "log", "-n", "1", "--pretty=format:%H", "--", filepath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting file commit: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func generateGitHubRawURL(remoteURL, commitHash, relativeFilepath string) string {
	// e.g. https://github.com/arran4/arrans_overlay.git
	// e.g. git@github.com:arran4/arrans_overlay.git
	// Output: https://github.com/arran4/arrans_overlay/blob/<commitHash>/<relativeFilepath>

	url := strings.TrimSuffix(remoteURL, ".git")

	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	} else if strings.HasPrefix(url, "git@gitlab.com:") {
		url = strings.Replace(url, "git@gitlab.com:", "https://gitlab.com/", 1)
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// Just a fallback if it doesn't match standard prefixes
		return ""
	}

	return fmt.Sprintf("%s/blob/%s/%s", url, commitHash, relativeFilepath)
}
