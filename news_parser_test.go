package g2

import (
	"strings"
	"testing"
)

func TestParseNewsItem_Format2(t *testing.T) {
	raw := `Title: Update defaults for various mail packages
Author: John Doe <john@example.com>
Translator: Jane Doe <jane@example.com>
Posted: 2024-03-15
Revision: 1
News-Item-Format: 2.0
Display-If-Installed: mail-mta/postfix
Display-If-Keyword: x86

This is an important update.

Note things like:

 * mail-mta/postfix-3.11.0: March 2026. Both lmdb and berkdb USE flags are on
   by default. BerkeleyDB is still supported but the default database and cache
   type changes to lmdb.
 * another point: this point continues
   here.

And code:

  emerge --ask mail-mta/postfix
`

	item := ParseNewsItem(raw)

	if item.Title != "Update defaults for various mail packages" {
		t.Errorf("expected title 'Update defaults for various mail packages', got '%s'", item.Title)
	}
	if item.Author != "John Doe" {
		t.Errorf("expected author 'John Doe', got '%s'", item.Author)
	}
	if len(item.Translator) != 1 || item.Translator[0] != "Jane Doe" {
		t.Errorf("expected translator 'Jane Doe', got '%v'", item.Translator)
	}
	if item.NewsItemFormat != "2.0" {
		t.Errorf("expected format '2.0', got '%s'", item.NewsItemFormat)
	}
	if len(item.DisplayIfInstalled) != 1 || item.DisplayIfInstalled[0] != "mail-mta/postfix" {
		t.Errorf("expected DisplayIfInstalled ['mail-mta/postfix'], got %v", item.DisplayIfInstalled)
	}
	if len(item.DisplayIfKeyword) != 1 || item.DisplayIfKeyword[0] != "x86" {
		t.Errorf("expected DisplayIfKeyword ['x86'], got %v", item.DisplayIfKeyword)
	}

	htmlOut := string(item.ToHTMLTemplate())
	if !strings.Contains(htmlOut, "<ul>") {
		t.Errorf("expected HTML to contain <ul>, got: %s", htmlOut)
	}
	if !strings.Contains(htmlOut, "<li>mail-mta/postfix-3.11.0: March 2026.") {
		t.Errorf("expected HTML to contain list item, got: %s", htmlOut)
	}
	if !strings.Contains(htmlOut, "<pre><code>\nemerge --ask mail-mta/postfix\n</code></pre>") {
		t.Errorf("expected HTML to contain code block, got: %s", htmlOut)
	}
}

func TestParseNewsItem_Format1(t *testing.T) {
	raw := `Title: Old news
Author: Old Guy <old@example.com>
Posted: 2010-01-01
Revision: 1
News-Item-Format: 1.0

This is old text.
It has newlines
that should be preserved.
`
	item := ParseNewsItem(raw)

	if item.Author != "Old Guy" {
		t.Errorf("expected author 'Old Guy', got '%s'", item.Author)
	}
	htmlOut := string(item.ToHTMLTemplate())
	if !strings.Contains(htmlOut, "This is old text.<br>It has newlines<br>that should be preserved.") {
		t.Errorf("expected old formatting to have <br>, got: %s", htmlOut)
	}
}

func TestStripEmail(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"John Doe <john@example.com>", "John Doe"},
		{"<only@email.com>", ""},
		{"No Email Here", "No Email Here"},
		{"Jane Doe <jane@example.com> (Translator)", "Jane Doe (Translator)"},
	}
	for _, tc := range tests {
		got := StripEmail(tc.in)
		if got != tc.out {
			t.Errorf("StripEmail(%q) = %q, expected %q", tc.in, got, tc.out)
		}
	}
}
