package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"
)

type MockFS struct {
	fstest.MapFS
}

func NewMockFS() *MockFS {
	return &MockFS{
		MapFS: make(fstest.MapFS),
	}
}

func (m *MockFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." {
		name = "."
	}

	// fstest.MapFS doesn't implement ReadDir natively in a way we can just call it if we add directories dynamically without files.
	// But actually MapFS has ReadDir
	return m.MapFS.ReadDir(name)
}

func (m *MockFS) MkdirAll(path string, perm os.FileMode) error {
	path = filepath.ToSlash(path)
	path = strings.TrimPrefix(path, "/")

	if m.MapFS[path] == nil {
		m.MapFS[path] = &fstest.MapFile{Mode: fs.ModeDir | perm}
	}
	return nil
}

type nopWriteCloser struct {
	io.Writer
}

func (n nopWriteCloser) Close() error {
	return nil
}

func (m *MockFS) Create(name string) (io.WriteCloser, error) {
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "/")
	buf := new(bytes.Buffer)

	m.MapFS[name] = &fstest.MapFile{Data: nil, Mode: 0644}

	// We'd need a custom write closer that updates MapFS on close or write, but for tests a simple buffer + update works
	// For full mock, WriteFile is usually easier.
	return nopWriteCloser{buf}, nil // simplistic, doesn't actually save to MapFS right away
}

func (m *MockFS) Remove(name string) error {
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "/")
	if _, ok := m.MapFS[name]; !ok {
		return fs.ErrNotExist
	}
	delete(m.MapFS, name)
	return nil
}

func (m *MockFS) RemoveAll(name string) error {
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "/")

	for k := range m.MapFS {
		if k == name || strings.HasPrefix(k, name+"/") {
			delete(m.MapFS, k)
		}
	}
	return nil
}

func (m *MockFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	name = filepath.ToSlash(name)
	name = strings.TrimPrefix(name, "/")

	// Ensure parent dir exists conceptually
	dir := filepath.Dir(name)
	if dir != "." {
		_ = m.MkdirAll(dir, 0755)
	}

	m.MapFS[name] = &fstest.MapFile{
		Data: append([]byte(nil), data...),
		Mode: perm,
	}
	return nil
}
