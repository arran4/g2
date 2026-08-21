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
		"metadata/news/2024-01-02-invalid-multiple-values/2024-01-02-invalid-multiple-values.en.txt": &fstest.MapFile{Data: []byte(`Title: Invalid Multiple Values
Author: John Doe <john@example.com>
Posted: 2024-01-02
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: app-misc/foo app-misc/bar
Display-If-Keyword: amd64 x86
Display-If-Profile: default/linux/amd64/17.0 default/linux/x86/17.0

This is the body.
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
		"metadata/news/2024-01-06-invalid-fields/2024-01-06-invalid-fields.en.txt": &fstest.MapFile{Data: []byte(`Title: This Title Is Much Longer Than Fifty Characters So It Should Fail The Linting Check
Author: John Doe <john@example.com>
Posted: 2024-01-06
Revision: 1a
News-Item-Format: 1.0
Content-Type: text/html

This	body contains a tab character and is definitely way too long, far exceeding the seventy-two character limit that is mandated by the standard we are supposed to be following here.
`)},
	}

	rule := NewNewsValidityLintRule(WithFS(fsys))

	pkg := &g2.PackageData{Category: "app-misc", Name: "foo"}

	results := rule.Lint(".", pkg)

	if len(results) == 0 {
		t.Fatalf("expected lint results, got none")
	}

	var hasAuthorErr, hasDisplayErr bool
	var hasDisplayInstalledSpaceErr, hasDisplayKeywordSpaceErr, hasDisplayProfileSpaceErr bool
	var hasInvalidDirErr, hasInvalidFileErr, hasMismatchErr bool
	var hasTitleLengthErr, hasRevisionIntErr, hasContentTypeErr, hasFormatOneErr bool
	var hasTabErr, hasWrapErr bool
	for _, res := range results {
		if strings.Contains(res.Message, "Invalid Author format") {
			hasAuthorErr = true
		}
		if strings.Contains(res.Message, "Invalid Display-If-Installed format, expected category/package") {
			hasDisplayErr = true
		}
		if strings.Contains(res.Message, "Invalid Display-If-Installed format, should not contain multiple values or spaces") {
			hasDisplayInstalledSpaceErr = true
		}
		if strings.Contains(res.Message, "Invalid Display-If-Keyword format, should not contain multiple values or spaces") {
			hasDisplayKeywordSpaceErr = true
		}
		if strings.Contains(res.Message, "Invalid Display-If-Profile format, should not contain multiple values or spaces") {
			hasDisplayProfileSpaceErr = true
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
		if strings.Contains(res.Message, "Title exceeds maximum length of 50 characters") {
			hasTitleLengthErr = true
		}
		if strings.Contains(res.Message, "Revision must be an integer") {
			hasRevisionIntErr = true
		}
		if strings.Contains(res.Message, "Content-Type must be 'text/plain'") {
			hasContentTypeErr = true
		}
		if strings.Contains(res.Message, "Content-Type: text/plain is mandatory for News-Item-Format 1.0") {
			hasFormatOneErr = true
		}
		if strings.Contains(res.Message, "Body lines should not contain tab characters") {
			hasTabErr = true
		}
		if strings.Contains(res.Message, "Body lines should wrap at 72 characters") {
			hasWrapErr = true
		}
	}

	if !hasAuthorErr {
		t.Errorf("expected Invalid Author format error")
	}
	if !hasDisplayErr {
		t.Errorf("expected Invalid Display-If-Installed format error")
	}
	if !hasDisplayInstalledSpaceErr {
		t.Errorf("expected Invalid Display-If-Installed format error (multiple values or spaces)")
	}
	if !hasDisplayKeywordSpaceErr {
		t.Errorf("expected Invalid Display-If-Keyword format error (multiple values or spaces)")
	}
	if !hasDisplayProfileSpaceErr {
		t.Errorf("expected Invalid Display-If-Profile format error (multiple values or spaces)")
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
	if !hasTitleLengthErr {
		t.Errorf("expected Title length error")
	}
	if !hasRevisionIntErr {
		t.Errorf("expected Revision integer error")
	}
	if !hasContentTypeErr {
		t.Errorf("expected Content-Type error")
	}
	if !hasFormatOneErr {
		t.Errorf("expected Format 1.0 mandatory Content-Type error")
	}
	if !hasTabErr {
		t.Errorf("expected Body tab character error")
	}
	if !hasWrapErr {
		t.Errorf("expected Body line wrap error")
	}
}
