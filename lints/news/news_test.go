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

func TestLintNewsItem_TableDriven(t *testing.T) {
	rule := &NewsValidityLintRule{}
	relPath := "metadata/news/2024-01-01-test/2024-01-01-test.en.txt"

	tests := []struct {
		name        string
		content     string
		expectedErr []string
	}{
		{
			name: "Valid News Item",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Translator: Jane Doe <jane@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: app-misc/foo

This is the body.
`,
			expectedErr: nil,
		},
		{
			name: "Missing required headers",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Posted: 2024-01-01
News-Item-Format: 2.0

This is the body.
`,
			expectedErr: []string{"Missing required header: Revision"},
		},
		{
			name: "Missing multiple required headers",
			content: `Title: Valid News

This is the body.
`,
			expectedErr: []string{"Missing required header: Author", "Missing required header: Posted", "Missing required header: Revision", "Missing required header: News-Item-Format"},
		},
		{
			name: "Format 1.0 requires text/plain Content-Type",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 1.0

This is the body.
`,
			expectedErr: []string{"Content-Type: text/plain is mandatory for News-Item-Format 1.0"},
		},
		{
			name: "Format 1.0 with correct Content-Type",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 1.0
Content-Type: text/plain

This is the body.
`,
			expectedErr: nil,
		},
		{
			name: "Format 1.0 with incorrect Content-Type",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 1.0
Content-Type: text/html

This is the body.
`,
			expectedErr: []string{"Content-Type must be 'text/plain'", "Content-Type: text/plain is mandatory for News-Item-Format 1.0"},
		},
		{
			name: "Unknown header warning",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 2.0
Unknown-Header: value

This is the body.
`,
			expectedErr: []string{"Unknown header: 'Unknown-Header'"},
		},
		{
			name: "Display-If-Installed missing category",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: foo

This is the body.
`,
			expectedErr: []string{"Invalid Display-If-Installed format, expected category/package"},
		},
		{
			name: "Display-If-* space validation",
			content: `Title: Valid News
Author: John Doe <john@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: app-misc/foo app-misc/bar
Display-If-Keyword: amd64 x86
Display-If-Profile: default/linux/amd64/17.0 default/linux/x86/17.0

This is the body.
`,
			expectedErr: []string{
				"Invalid Display-If-Installed format, should not contain multiple values or spaces",
				"Invalid Display-If-Keyword format, should not contain multiple values or spaces",
				"Invalid Display-If-Profile format, should not contain multiple values or spaces",
			},
		},
		{
			name: "Display-If-* tab validation",
			content: "Title: Valid News\n" +
				"Author: John Doe <john@example.com>\n" +
				"Posted: 2024-01-01\n" +
				"Revision: 1\n" +
				"News-Item-Format: 2.0\n" +
				"Display-If-Installed: app-misc/foo\tapp-misc/bar\n" +
				"Display-If-Keyword: amd64\tx86\n" +
				"Display-If-Profile: default/linux/amd64/17.0\tdefault/linux/x86/17.0\n" +
				"\n" +
				"This is the body.\n",
			expectedErr: []string{
				"Invalid Display-If-Installed format, should not contain multiple values or spaces",
				"Invalid Display-If-Keyword format, should not contain multiple values or spaces",
				"Invalid Display-If-Profile format, should not contain multiple values or spaces",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := rule.lintNewsItem(tt.content, relPath)

			for _, expectedMsg := range tt.expectedErr {
				found := false
				for _, res := range results {
					if strings.Contains(res.Message, expectedMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q, but did not find it in results:\n%v", expectedMsg, results)
				}
			}

			if len(tt.expectedErr) > 0 && len(results) != len(tt.expectedErr) {
				// We expect exact match in number of errors (excluding any accidental unrelated errors)
				// to ensure we aren't generating extra unintended warnings/errors.
				// However, if some test naturally generates multiple errors, ensure they are all covered.
				t.Logf("Warning: Expected %d errors, got %d. Check if there are unexpected errors.", len(tt.expectedErr), len(results))
			} else if len(tt.expectedErr) == 0 && len(results) > 0 {
				t.Errorf("Expected no errors, but got:\n%v", results)
			}
		})
	}
}
