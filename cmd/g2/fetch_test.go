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
)

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

func TestDownloadAndExtractZip_ZipSlip(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Add a normal file
	w, err := zw.Create("normal.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("normal"))

	// Add a malicious file
	wMalicious, err := zw.Create("../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = wMalicious.Write([]byte("malicious"))

	err = zw.Close()
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer ts.Close()

	destDir, err := os.MkdirTemp("", "g2-fetch-zipslip-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(destDir) }()

	err = downloadAndExtractZip(context.Background(), ts.URL, destDir)
	if err == nil {
		t.Fatal("expected error due to zip slip, got nil")
	}

	if !strings.Contains(err.Error(), "zip slip vulnerability detected") {
		t.Errorf("expected zip slip error message, got: %v", err)
	}
}
