package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/go-git/go-billy/v5/memfs"
)

func TestInitOverlay(t *testing.T) {
	fs := memfs.New()

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
	f, err := fs.Open("profiles/repo_name")
	if err != nil {
		t.Fatalf("Failed to open profiles/repo_name: %v", err)
	}
	content, _ := io.ReadAll(f)
	if string(content) != "test-overlay\n" {
		t.Errorf("Unexpected content in profiles/repo_name: %s", content)
	}
	f.Close()

	// check profiles/eapi
	f, err = fs.Open("profiles/eapi")
	if err != nil {
		t.Fatalf("Failed to open profiles/eapi: %v", err)
	}
	content, _ = io.ReadAll(f)
	if string(content) != "8\n" {
		t.Errorf("Unexpected content in profiles/eapi: %s", content)
	}
	f.Close()

	// check metadata/layout.conf
	f, err = fs.Open("metadata/layout.conf")
	if err != nil {
		t.Fatalf("Failed to open metadata/layout.conf: %v", err)
	}
	content, _ = io.ReadAll(f)
	if !bytes.Contains(content, []byte("masters = gentoo\n")) {
		t.Errorf("Unexpected content in metadata/layout.conf: %s", content)
	}
	f.Close()
}
