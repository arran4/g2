package news

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/g2"
)

func TestNewsValidityLintRule(t *testing.T) {
	fsys := fstest.MapFS{
		"metadata/news/2024-01-01-valid-news/2024-01-01-valid-news.en.txt": &fstest.MapFile{Data: []byte(`Title: Valid News
Author: John Doe <john@example.com>
Translator: Jane Doe <jane@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: app-misc/foo

This is the body.
`)},
		"metadata/news/2024-01-02-invalid-news/2024-01-02-invalid-news.en.txt": &fstest.MapFile{Data: []byte(`Title: Invalid News
Author: Invalid Author
Posted: 2024-01-02
Revision: 1
News-Item-Format: 1.0
Display-If-Installed: invalid_format

Body
`)},
	}

	rule := NewNewsValidityLintRule(WithFS(fsys))

	pkg := &g2.PackageData{Category: "app-misc", Name: "foo"}

	results := rule.Lint(".", pkg)

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
