package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ZipUrlConverter func(gitUrl string) (string, error)

var ZipUrlRegistry = map[string]ZipUrlConverter{
	"github.com": func(gitUrl string) (string, error) {
		u, err := url.Parse(gitUrl)
		if err != nil {
			return "", err
		}
		path := strings.TrimSuffix(u.Path, ".git")
		return fmt.Sprintf("https://%s%s/archive/HEAD.zip", u.Host, path), nil
	},
	"gitlab.com": gitlabUrlConverter,
	"bitbucket.org": func(gitUrl string) (string, error) {
		u, err := url.Parse(gitUrl)
		if err != nil {
			return "", err
		}
		path := strings.TrimSuffix(u.Path, ".git")
		return fmt.Sprintf("https://%s%s/get/HEAD.zip", u.Host, path), nil
	},
	"codeberg.org": giteaUrlConverter,
	"gitea.com":    giteaUrlConverter,
	"git.sr.ht": func(gitUrl string) (string, error) {
		u, err := url.Parse(gitUrl)
		if err != nil {
			return "", err
		}
		path := strings.TrimSuffix(u.Path, ".git")
		return fmt.Sprintf("https://%s%s/archive/HEAD.tar.gz", u.Host, path), nil // SourceHut defaults to tar.gz
	},
}

func gitlabUrlConverter(gitUrl string) (string, error) {
	u, err := url.Parse(gitUrl)
	if err != nil {
		return "", err
	}
	path := strings.TrimSuffix(u.Path, ".git")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid gitlab url")
	}
	repo := parts[len(parts)-1]
	return fmt.Sprintf("https://%s%s/-/archive/HEAD/%s-HEAD.zip", u.Host, path, repo), nil
}

func giteaUrlConverter(gitUrl string) (string, error) {
	u, err := url.Parse(gitUrl)
	if err != nil {
		return "", err
	}
	path := strings.TrimSuffix(u.Path, ".git")
	return fmt.Sprintf("https://%s%s/archive/HEAD.zip", u.Host, path), nil
}

func FetchRepo(ctx context.Context, gitUrl string, destDir string, useZip bool, workMode string, retries int) error {
	var err error
	for i := 0; i <= retries; i++ {
		if i > 0 {
			log.Printf("Retrying fetch for %s (attempt %d/%d)...", gitUrl, i, retries)
			_ = os.RemoveAll(destDir)
			time.Sleep(1 * time.Second)
		}
		err = fetchRepoAttempt(ctx, gitUrl, destDir, useZip, workMode)
		if err == nil {
			return nil
		}
		log.Printf("Fetch attempt %d failed for %s: %v", i+1, gitUrl, err)
	}
	return err
}

func updatePersistentRepo(ctx context.Context, destDir string) error {
	log.Printf("Persistent repo exists, attempting to fetch and reset: %s", destDir)
	noPromptEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	cmdFetch := exec.CommandContext(ctx, "git", "fetch", "--force", "--depth", "1", "origin", "HEAD")
	cmdFetch.Env = noPromptEnv
	cmdFetch.Dir = destDir
	cmdFetch.Stdout = os.Stdout
	cmdFetch.Stderr = os.Stderr
	if err := cmdFetch.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	cmdReset := exec.CommandContext(ctx, "git", "reset", "--hard", "FETCH_HEAD")
	cmdReset.Env = noPromptEnv
	cmdReset.Dir = destDir
	cmdReset.Stdout = os.Stdout
	cmdReset.Stderr = os.Stderr
	if err := cmdReset.Run(); err != nil {
		return fmt.Errorf("git reset failed: %w", err)
	}

	cmdClean := exec.CommandContext(ctx, "git", "clean", "-fdx")
	cmdClean.Env = noPromptEnv
	cmdClean.Dir = destDir
	cmdClean.Stdout = os.Stdout
	cmdClean.Stderr = os.Stderr
	if err := cmdClean.Run(); err != nil {
		return fmt.Errorf("git clean failed: %w", err)
	}

	return nil
}

type WriteFS interface {
	MkdirAll(path string, perm os.FileMode) error
	Create(name string) (io.WriteCloser, error)
}

type osWriteFS struct{}

func (osWriteFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (osWriteFS) Create(name string) (io.WriteCloser, error)   { return os.Create(name) }

func fetchRepoAttempt(ctx context.Context, gitUrl string, destDir string, useZip bool, workMode string) error {
	if workMode == "persistent" {
		gitDir := filepath.Join(destDir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			err := updatePersistentRepo(ctx, destDir)
			if err == nil {
				return nil
			}
			log.Printf("Persistent update failed: %v, wiping directory %s and doing a fresh clone", err, destDir)
			_ = os.RemoveAll(destDir)
		}
	}

	if useZip {
		u, err := url.Parse(gitUrl)
		if err == nil {
			converter, ok := ZipUrlRegistry[u.Host]

			if !ok {
				// Best-effort generic fallback
				if strings.Contains(u.Host, "gitlab") {
					converter = gitlabUrlConverter
				} else if strings.Contains(u.Host, "gitea") || strings.Contains(u.Host, "codeberg") || strings.Contains(u.Host, "forgejo") {
					converter = giteaUrlConverter
				}
			}

			if converter != nil {
				zipUrl, err := converter(gitUrl)
				if err == nil {
					if err := downloadAndExtractZip(ctx, zipUrl, destDir, osWriteFS{}); err == nil {
						return nil
					}
					// If zip fails, we fallback to git clone
					_ = os.RemoveAll(destDir) // Wipe out any partial extraction
				}
			}
		}
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", gitUrl, destDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadAndExtractZip(ctx context.Context, zipUrl string, destDir string, wfs WriteFS) error {
	req, err := http.NewRequestWithContext(ctx, "GET", zipUrl, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "g2-repo-*.zip")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}

	// Explicitly close the file to flush writes to disk before re-opening for unzip
	_ = tmpFile.Close()

	zr, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	var rootPrefix string
	if len(zr.File) > 0 {
		parts := strings.Split(zr.File[0].Name, "/")
		if len(parts) > 1 {
			potentialRoot := parts[0] + "/"
			allMatch := true
			for _, f := range zr.File {
				if !strings.HasPrefix(f.Name, potentialRoot) {
					allMatch = false
					break
				}
			}
			if allMatch {
				rootPrefix = potentialRoot
			}
		}
	}

	if err := wfs.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		relPath := f.Name
		if rootPrefix != "" && strings.HasPrefix(f.Name, rootPrefix) {
			relPath = strings.TrimPrefix(f.Name, rootPrefix)
		}

		localPath := filepath.FromSlash(relPath)
		if localPath == "" {
			continue // Root directory entry stripped by rootPrefix logic
		}
		if !filepath.IsLocal(localPath) {
			return fmt.Errorf("zip slip vulnerability detected: invalid path %q", relPath)
		}

		targetPath := filepath.Join(destDir, localPath)

		if err := wfs.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := wfs.Create(targetPath)
		if err != nil {
			_ = rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)

		if outErr := out.Close(); outErr != nil && err == nil {
			err = outErr
		}
		if rcErr := rc.Close(); rcErr != nil && err == nil {
			err = rcErr
		}
		if err != nil {
			return err
		}
	}

	return nil
}
