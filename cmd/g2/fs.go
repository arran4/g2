package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// WritableFS is an extension of fs.FS that supports write and delete operations
type WritableFS interface {
	fs.FS
	fs.ReadDirFS
	MkdirAll(path string, perm os.FileMode) error
	Create(name string) (io.WriteCloser, error)
	Remove(name string) error
	RemoveAll(name string) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

// OSFS is a WritableFS implementation that interacts with the real OS filesystem
type OSFS struct {
	base string
	fs.FS
}

// NewOSFS creates a new OSFS rooted at base. If base is empty, it uses the root directory (/).
func NewOSFS(base string) *OSFS {
	if base == "" {
		base = "/"
	}
	return &OSFS{
		base: base,
		FS:   os.DirFS(base),
	}
}

func (o *OSFS) join(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(o.base, name)
}

// ReadDir implements fs.ReadDirFS
func (o *OSFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(o.join(name))
}

func (o *OSFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(o.join(path), perm)
}

func (o *OSFS) Create(name string) (io.WriteCloser, error) {
	return os.Create(o.join(name))
}

func (o *OSFS) Remove(name string) error {
	return os.Remove(o.join(name))
}

func (o *OSFS) RemoveAll(name string) error {
	return os.RemoveAll(o.join(name))
}

func (o *OSFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(o.join(name), data, perm)
}
