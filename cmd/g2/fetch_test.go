package main

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadAndExtractZipRootPrefix(t *testing.T) {
	// Create a zip file in memory
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	files := []string{
		"myrepo-HEAD/src/",
		"myrepo-HEAD/README.md",
		"myrepo-HEAD/",
		"myrepo-HEAD/src/main.go",
	}

	for _, name := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		switch name {
		case "myrepo-HEAD/README.md":
			_, _ = f.Write([]byte("readme content"))
		case "myrepo-HEAD/src/main.go":
			_, _ = f.Write([]byte("package main"))
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

	destDir, err := os.MkdirTemp("", "g2-fetch-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(destDir) }()

	err = downloadAndExtractZip(context.Background(), ts.URL, destDir)
	if err != nil {
		t.Fatalf("downloadAndExtractZip failed: %v", err)
	}

	// Verify the root prefix "myrepo-HEAD/" was successfully stripped
	_, err = os.Stat(filepath.Join(destDir, "README.md"))
	if err != nil {
		t.Errorf("README.md not found in destDir: %v", err)
	}

	_, err = os.Stat(filepath.Join(destDir, "src", "main.go"))
	if err != nil {
		t.Errorf("src/main.go not found in destDir: %v", err)
	}

	_, err = os.Stat(filepath.Join(destDir, "myrepo-HEAD"))
	if err == nil {
		t.Errorf("myrepo-HEAD directory was not stripped")
	}
}
