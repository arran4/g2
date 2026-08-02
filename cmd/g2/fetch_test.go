package main

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
	"io"
)

type memWriteFS struct {
	files map[string]*bytes.Buffer
}

func (m *memWriteFS) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func (m *memWriteFS) Create(name string) (io.WriteCloser, error) {
	if m.files == nil {
		m.files = make(map[string]*bytes.Buffer)
	}
	buf := new(bytes.Buffer)
	m.files[name] = buf
	return nopCloser{Writer: buf}, nil
}

func TestDownloadAndExtractZipRootPrefix(t *testing.T) {
	// We use txtar to define the structure and content of our archive clearly.
	// Since txtar only stores files (not empty directories), we also add explicit directory
	// entries to simulate real ZIP archives where directories appear before or after files.
	fixture := `
-- myrepo-HEAD/README.md --
readme content
-- myrepo-HEAD/src/main.go --
package main
`
	ar := txtar.Parse([]byte(fixture))

	// Create a zip file in memory
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Explicitly create directory entries first, in a mixed order
	dirs := []string{
		"myrepo-HEAD/src/",
		"myrepo-HEAD/",
	}
	for _, d := range dirs {
		if _, err := zw.Create(d); err != nil {
			t.Fatal(err)
		}
	}

	// Add the actual files from the txtar
	for _, f := range ar.Files {
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(f.Data); err != nil {
			t.Fatal(err)
		}
	}

	err := zw.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Create a mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer ts.Close()

	destDir := "/memfs"

	mfs := &memWriteFS{}

	err = downloadAndExtractZip(context.Background(), ts.URL, destDir, mfs)
	if err != nil {
		t.Fatalf("downloadAndExtractZip failed: %v", err)
	}

	// Verify the root prefix "myrepo-HEAD/" was successfully stripped
	if _, ok := mfs.files[filepath.Join(destDir, "README.md")]; !ok {
		t.Errorf("README.md not found in destDir")
	}

	if _, ok := mfs.files[filepath.Join(destDir, "src", "main.go")]; !ok {
		t.Errorf("src/main.go not found in destDir")
	}

	for k := range mfs.files {
		if strings.Contains(k, "myrepo-HEAD") {
			t.Errorf("myrepo-HEAD directory was not stripped: found file %q", k)
		}
	}
}

func TestDownloadAndExtractZip_ZipSlip(t *testing.T) {
	fixture := `
-- normal.txt --
normal
-- ../../etc/passwd --
malicious
`
	ar := txtar.Parse([]byte(fixture))

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	for _, f := range ar.Files {
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(f.Data); err != nil {
			t.Fatal(err)
		}
	}

	err := zw.Close()
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer ts.Close()

	destDir := "/memfs"

	mfs := &memWriteFS{}

	err = downloadAndExtractZip(context.Background(), ts.URL, destDir, mfs)
	if err == nil {
		t.Fatal("expected error due to zip slip, got nil")
	}

	if !strings.Contains(err.Error(), "zip slip vulnerability detected") {
		t.Errorf("expected zip slip error message, got: %v", err)
	}
}

func TestGitUrlValidation(t *testing.T) {
	ctx := context.Background()
	destDir := "/tmp/test-dest"

	tests := []struct {
		url      string
		wantErr  bool
		errMatch string
	}{
		{"--upload-pack=sleep 10", true, "invalid git url: cannot start with '-'"},
		{"-foo", true, "invalid git url: cannot start with '-'"},
		{"invalid://foo", true, "invalid git url protocol: invalid://foo"},
		{"http://github.com/foo/bar", false, ""},
		{"https://github.com/foo/bar", false, ""},
		{"git://github.com/foo/bar", false, ""},
		{"ssh://git@github.com/foo/bar", false, ""},
		{"git@github.com:foo/bar", false, ""},
		{"file:///path/to/repo", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			// We only want to test validation, not actual cloning
			// fetchRepoAttempt with valid url will try to run 'git clone' and fail since the url is fake/unreachable
			err := fetchRepoAttempt(ctx, tt.url, destDir, false, "")
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("expected error containing %q, got: %v", tt.errMatch, err)
				}
			} else {
				if err != nil {
					// Since it's a dummy valid URL, git clone will likely fail, but it shouldn't fail due to our validation
					if strings.Contains(err.Error(), "invalid git url") {
						t.Errorf("did not expect validation error, got: %v", err)
					}
				}
			}
		})
	}
}
