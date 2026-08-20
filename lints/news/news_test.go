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
		"metadata/news/invalid_dir_name/2024-01-03-news.en.txt": &fstest.MapFile{Data: []byte(`Title: Invalid Dir
Author: John Doe <john@example.com>
Posted: 2024-01-03
Revision: 1
News-Item-Format: 2.0

Body
`)},
		"metadata/news/2024-01-04-invalid-file/invalid_file_name.txt": &fstest.MapFile{Data: []byte(`Title: Invalid File
Author: John Doe <john@example.com>
Posted: 2024-01-04
Revision: 1
News-Item-Format: 2.0

Body
`)},
		"metadata/news/2024-01-05-mismatch-prefix/2024-01-06-mismatch-prefix.en.txt": &fstest.MapFile{Data: []byte(`Title: Mismatched File
Author: John Doe <john@example.com>
Posted: 2024-01-05
Revision: 1
News-Item-Format: 2.0

Body
`)},
	}

	rule := NewNewsValidityLintRule(WithFS(fsys))

	pkg := &g2.PackageData{Category: "app-misc", Name: "foo"}

	results := rule.Lint(".", pkg, nil)

	if len(results) == 0 {
		t.Fatalf("expected lint results, got none")
	}

	var hasAuthorErr, hasDisplayErr bool
	var hasInvalidDirErr, hasInvalidFileErr, hasMismatchErr bool
	for _, res := range results {
		if strings.Contains(res.Message, "Invalid Author format") {
			hasAuthorErr = true
		}
		if strings.Contains(res.Message, "Invalid Display-If-Installed format") {
			hasDisplayErr = true
		}
		if strings.Contains(res.Message, "Invalid news directory name format") {
			hasInvalidDirErr = true
		}
		if strings.Contains(res.Message, "Invalid news file name format") {
			hasInvalidFileErr = true
		}
		if strings.Contains(res.Message, "does not match directory name") {
			hasMismatchErr = true
		}
	}

	if !hasAuthorErr {
		t.Errorf("expected Invalid Author format error")
	}
	if !hasDisplayErr {
		t.Errorf("expected Invalid Display-If-Installed format error")
	}
	if !hasInvalidDirErr {
		t.Errorf("expected Invalid news directory name format error")
	}
	if !hasInvalidFileErr {
		t.Errorf("expected Invalid news file name format error")
	}
	if !hasMismatchErr {
		t.Errorf("expected prefix mismatch error")
	}
}
