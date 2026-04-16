package g2

import (
	"html/template"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseNewsItem(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected NewsItem
	}{
		{
			name: "Basic Format 1.0",
			content: "Title: Test News\n" +
				"Author: Tester <tester@example.com>\n" +
				"Translator: Trans <trans@example.com>\n" +
				"Posted: 2023-01-01\n" +
				"Revision: 1\n" +
				"News-Item-Format: 1.0\n" +
				"Display-If-Installed: app-misc/foo\n" +
				"Display-If-Keyword: amd64\n" +
				"Display-If-Profile: default/linux/amd64/17.1\n" +
				"\n" +
				"This is the body.\n" +
				"Line 2.",
			expected: NewsItem{
				Title:              "Test News",
				Author:             "Tester",
				Translator:         []string{"Trans"},
				Posted:             time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				Revision:           "1",
				NewsItemFormat:     "1.0",
				DisplayIfInstalled: []string{"app-misc/foo"},
				DisplayIfKeyword:   []string{"amd64"},
				DisplayIfProfile:   []string{"default/linux/amd64/17.1"},
				Body:               "This is the body.\nLine 2.",
			},
		},
		{
			name: "Missing Headers",
			content: "Title: Test News 2\n" +
				"\n" +
				"Body only.",
			expected: NewsItem{
				Title: "Test News 2",
				Body:  "Body only.",
			},
		},
		{
			name: "Multiple Translators and Display Ifs",
			content: "Title: Multi\n" +
				"Translator: Trans 1 <t1@example.com>\n" +
				"Translator: Trans 2 <t2@example.com>\n" +
				"Display-If-Installed: app-misc/foo\n" +
				"Display-If-Installed: app-misc/bar\n" +
				"Display-If-Keyword: amd64\n" +
				"Display-If-Keyword: x86\n" +
				"Display-If-Profile: p1\n" +
				"Display-If-Profile: p2\n" +
				"\n" +
				"Body.",
			expected: NewsItem{
				Title:              "Multi",
				Translator:         []string{"Trans 1", "Trans 2"},
				DisplayIfInstalled: []string{"app-misc/foo", "app-misc/bar"},
				DisplayIfKeyword:   []string{"amd64", "x86"},
				DisplayIfProfile:   []string{"p1", "p2"},
				Body:               "Body.",
			},
		},
		{
			name: "Invalid Date",
			content: "Title: Invalid Date\n" +
				"Posted: invalid-date\n" +
				"\n" +
				"Body.",
			expected: NewsItem{
				Title: "Invalid Date",
				Body:  "Body.",
			},
		},
		{
			name: "Invalid Header Format",
			content: "Title: No Colon Format\n" +
				"InvalidHeaderNoColon\n" +
				"Author: Tester\n" +
				"\n" +
				"Body.",
			expected: NewsItem{
				Title:  "No Colon Format",
				Author: "Tester",
				Body:   "Body.",
			},
		},
		{
			name:    "Empty File",
			content: "",
			expected: NewsItem{
				Body: "",
			},
		},
		{
			name:    "No Body",
			content: "Title: No Body\n",
			expected: NewsItem{
				Title: "No Body",
				Body:  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseNewsItem(tt.content)

			// Verify all fields of the NewsItem struct match the expected values.
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseNewsItem() = %#v, want %#v", got, tt.expected)
			}
		})
	}
}

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

func TestNewsItem_ToHTMLTemplate(t *testing.T) {
	tests := []struct {
		name     string
		item     NewsItem
		expected template.HTML
	}{
		{
			name: "Format 1.0 (Plain text)",
			item: NewsItem{
				NewsItemFormat: "1.0",
				Body:           "Line 1\nLine 2 <tag>\nLine 3",
			},
			expected: "Line 1<br>Line 2 &lt;tag&gt;<br>Line 3",
		},
		{
			name: "Format 2.0 with AST",
			item: NewsItem{
				NewsItemFormat: "2.0",
				BodyAST: []NewsNode{
					{
						Type:  NewsNodeText,
						Lines: []string{"Text <escaped> 1", "", "Text 2"},
					},
					{
						Type:  NewsNodeList,
						Lines: []string{"List <1>", "List 2"},
					},
					{
						Type:  NewsNodeCode,
						Lines: []string{"code <var>", "another"},
					},
				},
			},
			expected: "Text &lt;escaped&gt; 1\n<br><br>\nText 2\n<ul>\n<li>List &lt;1&gt;</li>\n<li>List 2</li>\n</ul>\n<pre><code>\ncode &lt;var&gt;\nanother\n</code></pre>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.item.ToHTMLTemplate()
			if got != tt.expected {
				t.Errorf("NewsItem.ToHTMLTemplate() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewsItem_ToText(t *testing.T) {
	tests := []struct {
		name     string
		item     NewsItem
		expected string
	}{
		{
			name: "Format 1.0",
			item: NewsItem{
				NewsItemFormat: "1.0",
				Body:           "This is a standard body.\nIt has multiple lines.",
			},
			expected: "This is a standard body.\nIt has multiple lines.",
		},
		{
			name: "Format 2.0 with various nodes",
			item: NewsItem{
				NewsItemFormat: "2.0",
				BodyAST: []NewsNode{
					{
						Type:  NewsNodeText,
						Lines: []string{"Intro text line 1.", "Intro text line 2."},
					},
					{
						Type:  NewsNodeList,
						Lines: []string{"Item 1", "Item 2"},
					},
					{
						Type:  NewsNodeCode,
						Lines: []string{"code block line 1", "", "code block line 3"},
					},
					{
						Type:  NewsNodeText,
						Lines: []string{"Outro text."},
					},
				},
			},
			expected: "Intro text line 1.\nIntro text line 2.\n - Item 1\n - Item 2\n  code block line 1\n\n  code block line 3\nOutro text.",
		},
		{
			name: "Empty format",
			item: NewsItem{
				NewsItemFormat: "",
				Body:           "Fallback plain text body.",
			},
			expected: "Fallback plain text body.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.ToText(); got != tt.expected {
				t.Errorf("NewsItem.ToText() = %v, want %v", got, tt.expected)
			}
		})
	}
}
