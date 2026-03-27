package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

func TestGenerateSearchIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "g2-search-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	sites := []*SiteData{
		{
			Title:    "Test Repo",
			RepoName: "test-repo",
			Categories: []CategoryData{
				{
					Name: "app-test",
					Packages: []PackageData{
						{
							Name: "test-pkg",
							Versions: []VersionData{
								{
									Version: "1.0",
									Ebuild: &g2.Ebuild{
										Vars: map[string]string{
											"DESCRIPTION": "A test package for testing",
											"HOMEPAGE":    "https://example.com",
											"LICENSE":     "MIT",
											"KEYWORDS":    "amd64 ~x86",
											"IUSE":        "test-flag +enabled-flag",
											"DEPEND":      "dev-lang/go",
										},
									},
								},
							},
							PkgUseFlags: []PkgUseFlag{
								{
									Name: "test-flag",
									Desc: "Enables testing",
									Versions: map[string]string{
										"1.0": "test-flag",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := generateSearchIndex(tmpDir, sites); err != nil {
		t.Fatalf("generateSearchIndex failed: %v", err)
	}

	searchDir := filepath.Join(tmpDir, "search")
	dataDir := filepath.Join(searchDir, "data")

	// Verify JS files exist
	jsFiles := []string{"search.js", "search_parser.js", "search_ui.js"}
	for _, js := range jsFiles {
		if _, err := os.Stat(filepath.Join(searchDir, js)); err != nil {
			t.Errorf("Expected file %s not found: %v", js, err)
		}
	}

	// Verify manifest
	manifestPath := filepath.Join(dataDir, "manifest.json")
	mBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	var manifest SearchManifest
	if err := json.Unmarshal(mBytes, &manifest); err != nil {
		t.Fatalf("Failed to unmarshal manifest: %v", err)
	}

	if manifest.DocumentCount != 1 {
		t.Errorf("Expected 1 document, got %d", manifest.DocumentCount)
	}

	if len(manifest.DataFiles) != 1 {
		t.Fatalf("Expected 1 data file, got %d", len(manifest.DataFiles))
	}

	// Verify docs
	docsPath := filepath.Join(dataDir, manifest.DataFiles[0])
	dBytes, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("Failed to read docs: %v", err)
	}

	var docs []SearchDocument
	if err := json.Unmarshal(dBytes, &docs); err != nil {
		t.Fatalf("Failed to unmarshal docs: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("Expected 1 doc in file, got %d", len(docs))
	}

	doc := docs[0]
	if doc.FullName != "app-test/test-pkg" {
		t.Errorf("Expected FullName app-test/test-pkg, got %s", doc.FullName)
	}
	if doc.Description != "A test package for testing" {
		t.Errorf("Expected Description 'A test package for testing', got %s", doc.Description)
	}
	if len(doc.Licenses) == 0 || doc.Licenses[0] != "MIT" {
		t.Errorf("Expected license MIT, got %v", doc.Licenses)
	}
	if len(doc.Depends) == 0 || doc.Depends[0] != "dev-lang/go" {
		t.Errorf("Expected depend dev-lang/go, got %v", doc.Depends)
	}
	if len(doc.UseDescriptions) == 0 || doc.UseDescriptions[0] != "Enables testing" {
		t.Errorf("Expected use description 'Enables testing', got %v", doc.UseDescriptions)
	}
	if doc.VersionSortKey == "" {
		t.Errorf("Expected VersionSortKey to be populated, got empty string")
	}
}
