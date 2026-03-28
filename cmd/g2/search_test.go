package main

import (
	"testing"
)

func TestSearchEngine(t *testing.T) {
	docs := []SearchDocument{
		{
			ID:          1,
			FullName:    "app-admin/ollama",
			Version:     "0.0.1",
			Description: "Run LLMs locally",
			Licenses:    []string{"MIT"},
			Arches:      []string{"amd64", "arm64"},
			Mask:        "none",
			Keywords:    []string{"~amd64", "~arm64"},
			SearchText:  "app-admin/ollama run llms locally",
			Uses:        []string{"cuda", "rocm"},
			Depends:     []string{"dev-lang/go", "sys-libs/zlib"},
		},
		{
			ID:          2,
			FullName:    "dev-lang/go",
			Version:     "1.22.1",
			Description: "The Go Programming Language",
			Licenses:    []string{"BSD"},
			Arches:      []string{"amd64", "arm64", "x86"},
			Mask:        "none",
			Keywords:    []string{"amd64", "arm64"},
			SearchText:  "dev-lang/go the go programming language",
			Uses:        []string{},
			Depends:     []string{},
		},
		{
			ID:          3,
			FullName:    "sys-apps/systemd",
			Version:     "254",
			Description: "System and service manager for Linux",
			Licenses:    []string{"LGPL-2.1"},
			Arches:      []string{"amd64"},
			Mask:        "hard",
			Keywords:    []string{"-amd64"},
			SearchText:  "sys-apps/systemd system and service manager for linux",
			Uses:        []string{"pam", "seccomp"},
			Depends:     []string{"sys-libs/pam"},
		},
	}

	engine := NewSearchEngine()
	engine.LoadDocuments(docs)

	tests := []struct {
		name     string
		query    string
		expected []int // expected doc IDs
	}{
		{
			name:     "Basic term search",
			query:    "ollama",
			expected: []int{1},
		},
		{
			name:     "Field search license",
			query:    "license:BSD",
			expected: []int{2},
		},
		{
			name:     "AND implicit",
			query:    "license:MIT app-admin",
			expected: []int{1},
		},
		{
			name:     "AND explicit",
			query:    "license:MIT AND arch:amd64",
			expected: []int{1},
		},
		{
			name:     "Explicit OR",
			query:    "license:MIT OR license:BSD",
			expected: []int{1, 2},
		},
		{
			name:     "NOT",
			query:    "arch:amd64 NOT mask:hard",
			expected: []int{1, 2},
		},
		{
			name:     "Grouping",
			query:    "(license:MIT OR license:BSD) AND arch:amd64",
			expected: []int{1, 2},
		},
		{
			name:     "Version",
			query:    "version:>1.0.0",
			expected: []int{2, 3},
		},
		{
			name:     "Keyword field",
			query:    "keyword:~amd64",
			expected: []int{1},
		},
		{
			name:     "Depends field",
			query:    "depends:dev-lang/go",
			expected: []int{1},
		},
		{
			name:     "NOT syntax (!)",
			query:    "!mask:hard",
			expected: []int{1, 2},
		},
		{
			name:     "NOT syntax (-)",
			query:    "-mask:hard",
			expected: []int{1, 2},
		},
		{
			name:     "Sequence search",
			query:    "'system manager'",
			expected: []int{3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := engine.Search(tt.query)
			if len(results) != len(tt.expected) {
				t.Fatalf("Query %q: Expected %d results, got %d", tt.query, len(tt.expected), len(results))
			}

			// check if all expected IDs are in results
			resultMap := make(map[int]bool)
			for _, r := range results {
				resultMap[r.ID] = true
			}

			for _, id := range tt.expected {
				if !resultMap[id] {
					t.Errorf("Query %q: Expected doc ID %d to be in results", tt.query, id)
				}
			}
		})
	}
}

func TestSearchEngineArchives(t *testing.T) {
	// The core logic is tested heavily elsewhere.
	// We mainly want to test that the g2 package command correctly instantiates
	// the search engine and filters based on archive paths without panicking.

	docs := []SearchDocument{
		{ID: 1, FullName: "test/one", SearchText: "test/one"},
	}

	engine := NewSearchEngine()
	engine.LoadDocuments(docs)

	results := engine.Search("test")
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}
