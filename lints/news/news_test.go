package news

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestNewsValidityLintRule(t *testing.T) {
	rule := &NewsValidityLintRule{
		checkedRepos: make(map[string]bool),
	}

	tempDir, err := os.MkdirTemp("", "news-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	newsDir := filepath.Join(tempDir, "metadata", "news")
	if err := os.MkdirAll(newsDir, 0755); err != nil {
		t.Fatal(err)
	}

	validNewsDir := filepath.Join(newsDir, "2024-01-01-valid-news")
	if err := os.MkdirAll(validNewsDir, 0755); err != nil {
		t.Fatal(err)
	}

	validContent := `Title: Valid News
Author: John Doe <john@example.com>
Translator: Jane Doe <jane@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: app-misc/foo

This is the body.
`
	if err := os.WriteFile(filepath.Join(validNewsDir, "2024-01-01-valid-news.en.txt"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	invalidNewsDir := filepath.Join(newsDir, "2024-01-02-invalid-news")
	if err := os.MkdirAll(invalidNewsDir, 0755); err != nil {
		t.Fatal(err)
	}

	invalidContent := `Title: Invalid News
Author: Invalid Author
Posted: 2024-01-02
Revision: 1
News-Item-Format: 1.0
Display-If-Installed: invalid_format

Body
`
	if err := os.WriteFile(filepath.Join(invalidNewsDir, "2024-01-02-invalid-news.en.txt"), []byte(invalidContent), 0644); err != nil {
		t.Fatal(err)
	}

	pkg := &g2.PackageData{Category: "app-misc", Name: "foo"}

	results := rule.Lint(tempDir, pkg)

	if len(results) == 0 {
		t.Fatalf("expected lint results, got none")
	}

	var hasAuthorErr, hasDisplayErr bool
	for _, res := range results {
		if strings.Contains(res.Message, "Invalid Author format") {
			hasAuthorErr = true
		}
		if strings.Contains(res.Message, "Invalid Display-If-Installed format") {
			hasDisplayErr = true
		}
	}

	if !hasAuthorErr {
		t.Errorf("expected Invalid Author format error")
	}
	if !hasDisplayErr {
		t.Errorf("expected Invalid Display-If-Installed format error")
	}
}
