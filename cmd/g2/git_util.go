package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// repoGitInfo caches repository-level git metadata so per-file lookups do not
// repeatedly open the git repository or spawn git subprocesses.
type repoGitInfo struct {
	sysFS        fs.FS
	repoDir      string
	localRepoDir string
	headCommit   string
	headTime     time.Time
	fileModTimes map[string]time.Time
}

func newRepoGitInfo(sysFS fs.FS, repoDir string, localRepoDir string, fast bool) *repoGitInfo {
	info := &repoGitInfo{
		sysFS:        sysFS,
		repoDir:      repoDir,
		localRepoDir: localRepoDir,
	}
	if localRepoDir == "" {
		return info
	}
	if st, err := os.Stat(filepath.Join(localRepoDir, ".git")); err != nil || !st.IsDir() {
		return info
	}

	repo, err := git.PlainOpen(localRepoDir)
	if err != nil {
		return info
	}
	head, err := repo.Head()
	if err != nil {
		return info
	}
	info.headCommit = head.Hash().String()
	if headCommitObj, err := repo.CommitObject(head.Hash()); err == nil && headCommitObj != nil {
		info.headTime = headCommitObj.Author.When
		// If this is a shallow clone (single commit) or fast mode, avoid walking history per file.
		if !fast && headCommitObj.NumParents() > 0 {
			info.fileModTimes = buildRepoFileModTimes(repo, headCommitObj, 500)
		}
	}
	return info
}

// buildRepoFileModTimes walks at most maxCommits from head in a single pass to record
// recent file modification times without performing O(files * commits) traversals.
func buildRepoFileModTimes(repo *git.Repository, headCommit *object.Commit, maxCommits int) map[string]time.Time {
	modTimes := make(map[string]time.Time)
	commitIter, err := repo.Log(&git.LogOptions{From: headCommit.Hash})
	if err != nil {
		return modTimes
	}
	defer commitIter.Close()

	count := 0
	for count < maxCommits {
		c, err := commitIter.Next()
		if err != nil || c == nil {
			break
		}
		count++
		if c.NumParents() == 0 {
			if tree, err := c.Tree(); err == nil {
				_ = tree.Files().ForEach(func(f *object.File) error {
					if _, exists := modTimes[f.Name]; !exists {
						modTimes[f.Name] = c.Author.When
					}
					return nil
				})
			}
			break
		}
		parent, err := c.Parent(0)
		if err != nil {
			continue
		}
		currTree, err := c.Tree()
		if err != nil {
			continue
		}
		parentTree, err := parent.Tree()
		if err != nil {
			continue
		}
		changes, err := object.DiffTree(parentTree, currTree)
		if err != nil {
			continue
		}
		for _, ch := range changes {
			name := ch.To.Name
			if name == "" {
				name = ch.From.Name
			}
			if name != "" {
				if _, exists := modTimes[name]; !exists {
					modTimes[name] = c.Author.When
				}
			}
		}
	}
	return modTimes
}

func (g *repoGitInfo) fileCommit(relPath string) string {
	if g == nil {
		return ""
	}
	return g.headCommit
}

func (g *repoGitInfo) fileModTime(relPath string) time.Time {
	if g != nil {
		slashRel := filepath.ToSlash(relPath)
		if g.fileModTimes != nil {
			if t, ok := g.fileModTimes[slashRel]; ok {
				return t
			}
		}
		if g.sysFS != nil {
			fsPath := filepath.ToSlash(filepath.Join(g.repoDir, relPath))
			if st, err := fs.Stat(g.sysFS, fsPath); err == nil {
				if !st.ModTime().IsZero() {
					return st.ModTime()
				}
			}
		}
		if g.localRepoDir != "" {
			if st, err := os.Stat(filepath.Join(g.localRepoDir, relPath)); err == nil {
				return st.ModTime()
			}
		}
		if !g.headTime.IsZero() {
			return g.headTime
		}
	}
	return time.Now()
}

func getFileModTime(repoDir string, relPath string, fast bool) time.Time {
	repo, err := git.PlainOpen(repoDir)
	if err == nil {
		head, err := repo.Head()
		if err == nil {
			if fast {
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

func getGitOriginURL(repoDir string) (string, error) {
	if repoDir == "" {
		return "", fmt.Errorf("empty repoDir")
	}
	if st, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil || !st.IsDir() {
		return "", fmt.Errorf("not a git repository: %s", repoDir)
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

func getFileCommit(repoDir string, filepath string) (string, error) {
	cmd := exec.Command("git", "log", "-n", "1", "--pretty=format:%H", "--", filepath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting file commit: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func generateGitHubRawURL(remoteURL, commitHash, relativeFilepath string) string {
	url := strings.TrimSuffix(remoteURL, ".git")

	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	} else if strings.HasPrefix(url, "git@gitlab.com:") {
		url = strings.Replace(url, "git@gitlab.com:", "https://gitlab.com/", 1)
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return ""
	}

	return fmt.Sprintf("%s/blob/%s/%s", url, commitHash, relativeFilepath)
}
