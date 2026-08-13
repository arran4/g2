package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type mockFS struct {
	files map[string][]byte
}

func newMockFS() *mockFS {
	return &mockFS{files: make(map[string][]byte)}
}

func (m *mockFS) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (m *mockFS) Stat(name string) (os.FileInfo, error) {
	if _, ok := m.files[name]; ok {
		return mockFileInfo{name: name}, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	m.files[name] = data
	return nil
}

type mockFileInfo struct {
	name string
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return 0 }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return false }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestInitOverlay(t *testing.T) {
	fs := newMockFS()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "masters = gentoo\n")
	}))
	defer ts.Close()

	args := OverlayInitArgs{
		Name:          "test-overlay",
		EapiVersion:   "8",
		LayoutConfUrl: ts.URL,
	}

	err := InitOverlay(fs, args)
	if err != nil {
		t.Fatalf("InitOverlay failed: %v", err)
	}

	// check profiles/repo_name
	content, ok := fs.files["profiles/repo_name"]
	if !ok {
		t.Fatalf("Failed to find profiles/repo_name")
	}
	if string(content) != "test-overlay\n" {
		t.Errorf("Unexpected content in profiles/repo_name: %s", content)
	}

	// check profiles/eapi
	content, ok = fs.files["profiles/eapi"]
	if !ok {
		t.Fatalf("Failed to find profiles/eapi")
	}
	if string(content) != "8\n" {
		t.Errorf("Unexpected content in profiles/eapi: %s", content)
	}

	// check metadata/layout.conf
	content, ok = fs.files["metadata/layout.conf"]
	if !ok {
		t.Fatalf("Failed to find metadata/layout.conf")
	}
	if !bytes.Contains(content, []byte("masters = gentoo\n")) {
		t.Errorf("Unexpected content in metadata/layout.conf: %s", content)
	}
}
