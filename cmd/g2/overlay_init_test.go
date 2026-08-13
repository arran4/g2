package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInitOverlay(t *testing.T) {
	fs := NewMockFS()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "masters = gentoo\n")
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
	fileInfo, ok := fs.MapFS["profiles/repo_name"]
	if !ok {
		t.Fatalf("Failed to find profiles/repo_name")
	}
	if string(fileInfo.Data) != "test-overlay\n" {
		t.Errorf("Unexpected content in profiles/repo_name: %s", fileInfo.Data)
	}

	// check profiles/eapi
	fileInfo, ok = fs.MapFS["profiles/eapi"]
	if !ok {
		t.Fatalf("Failed to find profiles/eapi")
	}
	if string(fileInfo.Data) != "8\n" {
		t.Errorf("Unexpected content in profiles/eapi: %s", fileInfo.Data)
	}

	// check metadata/layout.conf
	fileInfo, ok = fs.MapFS["metadata/layout.conf"]
	if !ok {
		t.Fatalf("Failed to find metadata/layout.conf")
	}
	if !bytes.Contains(fileInfo.Data, []byte("masters = gentoo\n")) {
		t.Errorf("Unexpected content in metadata/layout.conf: %s", fileInfo.Data)
	}
}
