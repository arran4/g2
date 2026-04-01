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

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
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

func FetchRepoMemory(ctx context.Context, gitUrl string, retries int) (*git.Repository, billy.Filesystem, error) {
	var err error
	var repo *git.Repository
	var fs billy.Filesystem

	for i := 0; i <= retries; i++ {
		if i > 0 {
			log.Printf("Retrying fetch for %s in memory (attempt %d/%d)...", gitUrl, i, retries)
			time.Sleep(1 * time.Second)
		}

		storer := memory.NewStorage()
		fs = memfs.New()

		repo, err = git.CloneContext(ctx, storer, fs, &git.CloneOptions{
			URL:          gitUrl,
			Depth:        1,
			SingleBranch: true,
			Progress:     os.Stdout,
		})

		if err == nil {
			return repo, fs, nil
		}
		log.Printf("Fetch attempt %d failed for %s in memory: %v", i+1, gitUrl, err)
	}
	return nil, nil, err
}

func FetchRepo(ctx context.Context, gitUrl string, destDir string, useZip bool, retries int) error {
	var err error
	for i := 0; i <= retries; i++ {
		if i > 0 {
			log.Printf("Retrying fetch for %s (attempt %d/%d)...", gitUrl, i, retries)
			_ = os.RemoveAll(destDir)
			time.Sleep(1 * time.Second)
		}
		err = fetchRepoAttempt(ctx, gitUrl, destDir, useZip)
		if err == nil {
			return nil
		}
		log.Printf("Fetch attempt %d failed for %s: %v", i+1, gitUrl, err)
	}
	return err
}

func fetchRepoAttempt(ctx context.Context, gitUrl string, destDir string, useZip bool) error {
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
					if err := downloadAndExtractZip(ctx, zipUrl, destDir); err == nil {
						return nil
					}
					// If zip fails, we fallback to git clone
					_ = os.RemoveAll(destDir) // Wipe out any partial extraction
				}
			}
		}
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", gitUrl, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func downloadAndExtractZip(ctx context.Context, zipUrl string, destDir string) error {
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

	if err := os.MkdirAll(destDir, 0755); err != nil {
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

		targetPath := filepath.Clean(filepath.Join(destDir, relPath))
		if !strings.HasPrefix(targetPath, filepath.Clean(destDir)+string(os.PathSeparator)) && targetPath != filepath.Clean(destDir) {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		out, err := os.Create(targetPath)
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
