package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"log"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
)

func getZipUrl(gitUrl string) string {
	u, err := url.Parse(gitUrl)
	if err != nil {
		return ""
	}

	converter, ok := ZipUrlRegistry[u.Host]
	if !ok {
		if strings.Contains(u.Host, "gitlab") {
			converter = gitlabUrlConverter
		} else if strings.Contains(u.Host, "gitea") || strings.Contains(u.Host, "codeberg") || strings.Contains(u.Host, "forgejo") {
			converter = giteaUrlConverter
		}
	}

	if converter != nil {
		zipUrl, err := converter(gitUrl)
		if err == nil {
			return zipUrl
		}
	}
	return ""
}

func tryFetchZipFS(ctx context.Context, zipUrl string) (fs.FS, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", zipUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

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
				rootPrefix = parts[0]
			}
		}
	}

	if rootPrefix != "" {
		return fs.Sub(zr, rootPrefix)
	}

	return zr, nil
}

type billyFS struct {
	bfs billy.Filesystem
}

type billyFile struct {
	bf billy.File
	fs billy.Filesystem
	name string
}

func (b *billyFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	f, err := b.bfs.Open(name)
	if err != nil {
		return nil, err
	}
	return &billyFile{bf: f, fs: b.bfs, name: name}, nil
}

func (f *billyFile) Stat() (fs.FileInfo, error) {
	return f.fs.Stat(f.name)
}

func (f *billyFile) Read(b []byte) (int, error) {
	return f.bf.Read(b)
}

func (f *billyFile) Close() error {
	return f.bf.Close()
}

type billyDirEntry struct {
	info fs.FileInfo
}
func (b *billyDirEntry) Name() string { return b.info.Name() }
func (b *billyDirEntry) IsDir() bool { return b.info.IsDir() }
func (b *billyDirEntry) Type() fs.FileMode { return b.info.Mode().Type() }
func (b *billyDirEntry) Info() (fs.FileInfo, error) { return b.info, nil }


func (b *billyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	infos, err := b.bfs.ReadDir(name)
	if err != nil {
		return nil, err
	}

	entries := make([]fs.DirEntry, len(infos))
	for i, info := range infos {
		entries[i] = &billyDirEntry{info: info}
	}
	return entries, nil
}


func tryFetchGitFS(ctx context.Context, gitUrl string) (fs.FS, error) {
	st := memory.NewStorage()
	bfs := memfs.New()
	_, err := git.CloneContext(ctx, st, bfs, &git.CloneOptions{
		URL: gitUrl,
		Depth: 1,
	})
	if err != nil {
		return nil, err
	}
	return &billyFS{bfs: bfs}, nil
}

type MemoryManager struct {
	mu sync.Mutex
	cond *sync.Cond
	reserved uint64
	activeTasks int
}

func NewMemoryManager() *MemoryManager {
	m := &MemoryManager{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *MemoryManager) Acquire(baseAlloc uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		free, err := getFreeMemory()
		if err != nil {
			// If we can't get free memory, we should wait and retry.
			log.Printf("Warning: could not get free memory: %v. Retrying...", err)
			m.cond.Wait()
			continue
		}

		if m.activeTasks == 0 || free >= m.reserved+baseAlloc {
			m.activeTasks++
			m.reserved += baseAlloc
			break
		}
		// Not enough memory, wait
		m.cond.Wait()
	}
}

func (m *MemoryManager) Release(baseAlloc uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeTasks--
	if m.reserved >= baseAlloc {
		m.reserved -= baseAlloc
	} else {
		m.reserved = 0
	}
	m.cond.Signal() // Wake up one waiting goroutine
}
