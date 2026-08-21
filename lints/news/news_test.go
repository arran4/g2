package news

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func errorFinding(msg string) Finding {
	return Finding{Severity: lints.SeverityError, Message: msg}
}

func warningFinding(msg string) Finding {
	return Finding{Severity: lints.SeverityWarning, Message: msg}
}

func testHeader(t *testing.T, key string, values []string, expectedFindings ...Finding) {
	t.Helper()
	var actualFindings []Finding
	if validators, ok := headerValidators[key]; ok {
		for _, v := range validators {
			actualFindings = append(actualFindings, v(key, values)...)
		}
	} else {
		t.Fatalf("No validators registered for header: %s", key)
	}

	if len(expectedFindings) == 0 {
		expectedFindings = nil // match actualFindings nil value
	}
	if len(actualFindings) == 0 {
		actualFindings = nil
	}

	if !reflect.DeepEqual(actualFindings, expectedFindings) {
		t.Errorf("Header %q with values %v\nExpected findings: %#v\nActual findings:   %#v", key, values, expectedFindings, actualFindings)
	}
}

func testValidator(t *testing.T, name string, validator Validator, headers Headers, expectedFindings ...Finding) {
	t.Helper()
	actualFindings := validator(headers)

	if len(expectedFindings) == 0 {
		expectedFindings = nil
	}
	if len(actualFindings) == 0 {
		actualFindings = nil
	}

	if !reflect.DeepEqual(actualFindings, expectedFindings) {
		t.Errorf("Validator %q with headers %v\nExpected findings: %#v\nActual findings:   %#v", name, headers, expectedFindings, actualFindings)
	}
}

func TestHeaderValidators(t *testing.T) {
	t.Run("Title", func(t *testing.T) {
		testHeader(t, "Title", nil, errorFinding("Missing required header: Title"))
		testHeader(t, "Title", []string{""}, errorFinding("Title cannot be empty"))
		testHeader(t, "Title", []string{"Valid Title"})
		testHeader(t, "Title", []string{"Title 1", "Title 2"}, errorFinding("Header Title must appear at most once"))
		testHeader(t, "Title", []string{"This Title Is Much Longer Than Fifty Characters So It Should Fail"}, errorFinding("Title exceeds maximum length of 50 characters"))
	})

	t.Run("Author", func(t *testing.T) {
		testHeader(t, "Author", nil, errorFinding("Missing required header: Author"))
		testHeader(t, "Author", []string{"Invalid Author"}, errorFinding("Invalid Author format, expected 'Name <email@domain.com>': 'Invalid Author'"))
		testHeader(t, "Author", []string{"John Doe <john@example.com>"})
		testHeader(t, "Author", []string{"John Doe <john@example.com>", "Jane Doe <jane@example.com>"}) // Multiple valid authors is OK
	})

	t.Run("Translator", func(t *testing.T) {
		testHeader(t, "Translator", nil) // Not required
		testHeader(t, "Translator", []string{"Invalid Translator"}, errorFinding("Invalid Translator format, expected 'Name <email@domain.com>': 'Invalid Translator'"))
		testHeader(t, "Translator", []string{"John Doe <john@example.com>", "Jane Doe <jane@example.com>"}) // Multiple valid translators is OK
	})

	t.Run("Posted", func(t *testing.T) {
		testHeader(t, "Posted", nil, errorFinding("Missing required header: Posted"))
		testHeader(t, "Posted", []string{"2024/01/01"}, errorFinding("Invalid Posted date format, expected YYYY-MM-DD: '2024/01/01'"))
		testHeader(t, "Posted", []string{"2024-01-01"})
		testHeader(t, "Posted", []string{"2024-01-01", "2024-01-02"}, errorFinding("Header Posted must appear at most once"))
	})

	t.Run("Revision", func(t *testing.T) {
		testHeader(t, "Revision", nil, errorFinding("Missing required header: Revision"))
		testHeader(t, "Revision", []string{""}, errorFinding("Revision cannot be empty"))
		testHeader(t, "Revision", []string{"1a"}, errorFinding("Revision must be an integer: '1a'"))
		testHeader(t, "Revision", []string{"1"})
		testHeader(t, "Revision", []string{"1", "2"}, errorFinding("Header Revision must appear at most once"))
	})

	t.Run("Content-Type", func(t *testing.T) {
		testHeader(t, "Content-Type", []string{"text/html"}, errorFinding("Content-Type must be 'text/plain', got: 'text/html'"))
		testHeader(t, "Content-Type", []string{"text/plain"})
		testHeader(t, "Content-Type", []string{"text/plain", "text/plain"}, errorFinding("Header Content-Type must appear at most once"))
	})

	t.Run("News-Item-Format", func(t *testing.T) {
		testHeader(t, "News-Item-Format", nil, errorFinding("Missing required header: News-Item-Format"))
		testHeader(t, "News-Item-Format", []string{"3.0"}, warningFinding("Unsupported News-Item-Format: '3.0'"))
		testHeader(t, "News-Item-Format", []string{"1.0"})
		testHeader(t, "News-Item-Format", []string{"2.0"})
		testHeader(t, "News-Item-Format", []string{"2.0", "2.0"}, errorFinding("Header News-Item-Format must appear at most once"))
	})

	t.Run("Display-If-Installed", func(t *testing.T) {
		testHeader(t, "Display-If-Installed", nil) // Not required
		testHeader(t, "Display-If-Installed", []string{"app-misc/foo app-misc/bar"}, errorFinding("Invalid Display-If-Installed format, should not contain multiple values or spaces: 'app-misc/foo app-misc/bar'"))
		testHeader(t, "Display-If-Installed", []string{"app-misc/foo\tapp-misc/bar"}, errorFinding("Invalid Display-If-Installed format, should not contain multiple values or spaces: 'app-misc/foo\tapp-misc/bar'"))
		testHeader(t, "Display-If-Installed", []string{"foo"}, errorFinding("Invalid Display-If-Installed format, expected category/package: 'foo'"))
		testHeader(t, "Display-If-Installed", []string{"app-misc/foo", "app-misc/bar"}) // Multiple valid headers is OK
	})

	t.Run("Display-If-Keyword", func(t *testing.T) {
		testHeader(t, "Display-If-Keyword", nil) // Not required
		testHeader(t, "Display-If-Keyword", []string{"amd64 x86"}, errorFinding("Invalid Display-If-Keyword format, should not contain multiple values or spaces: 'amd64 x86'"))
		testHeader(t, "Display-If-Keyword", []string{"amd64", "x86"}) // Multiple valid headers is OK
	})

	t.Run("Display-If-Profile", func(t *testing.T) {
		testHeader(t, "Display-If-Profile", nil) // Not required
		testHeader(t, "Display-If-Profile", []string{"default/linux/amd64/17.0 default/linux/x86/17.0"}, errorFinding("Invalid Display-If-Profile format, should not contain multiple values or spaces: 'default/linux/amd64/17.0 default/linux/x86/17.0'"))
		testHeader(t, "Display-If-Profile", []string{"default/linux/amd64/17.0", "default/linux/x86/17.0"}) // Multiple valid headers is OK
	})
}

func TestCrossValidators(t *testing.T) {
	t.Run("Format 1.0 requires text/plain Content-Type", func(t *testing.T) {
		testValidator(t, "validateFormatContentTypeCrossCheck", validateFormatContentTypeCrossCheck, Headers{
			"News-Item-Format": []string{"1.0"},
		}, errorFinding("Content-Type: text/plain is mandatory for News-Item-Format 1.0"))

		testValidator(t, "validateFormatContentTypeCrossCheck", validateFormatContentTypeCrossCheck, Headers{
			"News-Item-Format": []string{"1.0"},
			"Content-Type":     []string{"text/html"},
		}, errorFinding("Content-Type: text/plain is mandatory for News-Item-Format 1.0"))

		testValidator(t, "validateFormatContentTypeCrossCheck", validateFormatContentTypeCrossCheck, Headers{
			"News-Item-Format": []string{"1.0"},
			"Content-Type":     []string{"text/plain"},
		})

		testValidator(t, "validateFormatContentTypeCrossCheck", validateFormatContentTypeCrossCheck, Headers{
			"News-Item-Format": []string{"2.0"},
		})
	})
}

func TestLintNewsItemEndToEnd(t *testing.T) {
	fsys := fstest.MapFS{
		"metadata/news/2024-01-01-valid-news/2024-01-01-valid-news.en.txt": &fstest.MapFile{Data: []byte(`Title: Valid News
Author: John Doe <john@example.com>
Translator: Jane Doe <jane@example.com>
Posted: 2024-01-01
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: app-misc/foo
Display-If-Installed: app-misc/bar

This is the body.
`)},
		"metadata/news/2024-01-02-invalid-news/2024-01-02-invalid-news.en.txt": &fstest.MapFile{Data: []byte(`Title: Invalid News
Author: Invalid Author
Posted: 2024-01-02
Revision: 1
News-Item-Format: 1.0
Unknown-Header: value
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

	var hasAuthorErr, hasDisplayErr, hasFormatOneErr, hasUnknownHeaderWarn bool
	for _, res := range results {
		if strings.Contains(res.Message, "Invalid Author format") {
			hasAuthorErr = true
		}
		if strings.Contains(res.Message, "Invalid Display-If-Installed format, expected category/package") {
			hasDisplayErr = true
		}
		if strings.Contains(res.Message, "Unknown header: 'Unknown-Header'") {
			hasUnknownHeaderWarn = true
		}
		if strings.Contains(res.Message, "Content-Type: text/plain is mandatory for News-Item-Format 1.0") {
			hasFormatOneErr = true
		}
	}

	if !hasAuthorErr {
		t.Errorf("expected Invalid Author format error")
	}
	if !hasDisplayErr {
		t.Errorf("expected Invalid Display-If-Installed format error")
	}
	if !hasUnknownHeaderWarn {
		t.Errorf("expected Unknown header warning")
	}
	if !hasFormatOneErr {
		t.Errorf("expected Format 1.0 mandatory Content-Type error")
	}
}
