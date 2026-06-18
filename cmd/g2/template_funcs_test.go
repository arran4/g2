package main

import (
	"github.com/arran4/g2"
	"html/template"
	"testing"
)

func TestFormatKeywordsFunc(t *testing.T) {
	tests := []struct {
		name     string
		keywords string
		baseURL  string
		expected template.HTML
	}{
		{
			name:     "empty keywords",
			keywords: "",
			baseURL:  "/",
			expected: template.HTML(""),
		},
		{
			name:     "simple keywords",
			keywords: "amd64 x86",
			baseURL:  "/",
			expected: template.HTML("<a href=\"/arches/amd64/\" class=\"text-decoration-none\">amd64</a> <a href=\"/arches/x86/\" class=\"text-decoration-none\">x86</a>"),
		},
		{
			name:     "keywords with tilde",
			keywords: "~amd64 x86",
			baseURL:  "/",
			expected: template.HTML("~<a href=\"/arches/amd64/\" class=\"text-decoration-none\">amd64</a> <a href=\"/arches/x86/\" class=\"text-decoration-none\">x86</a>"),
		},
		{
			name:     "keywords with minus",
			keywords: "-* amd64",
			baseURL:  "/",
			expected: template.HTML("-* <a href=\"/arches/amd64/\" class=\"text-decoration-none\">amd64</a>"),
		},
		{
			name:     "xss in baseURL",
			keywords: "amd64",
			baseURL:  "\"><script>alert(1)</script>",
			expected: template.HTML("<a href=\"&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;arches/amd64/\" class=\"text-decoration-none\">amd64</a>"),
		},
		{
			name:     "xss in arch",
			keywords: "<script>alert(1)</script>",
			baseURL:  "/",
			expected: template.HTML("<a href=\"/arches/%3Cscript%3Ealert%281%29%3C%2Fscript%3E/\" class=\"text-decoration-none\">&lt;script&gt;alert(1)&lt;/script&gt;</a>"),
		},
		{
			name:     "xss in arch with tilde",
			keywords: "~<script>alert(1)</script>",
			baseURL:  "/",
			expected: template.HTML("~<a href=\"/arches/%3Cscript%3Ealert%281%29%3C%2Fscript%3E/\" class=\"text-decoration-none\">&lt;script&gt;alert(1)&lt;/script&gt;</a>"),
		},
		{
			name:     "xss in minus keyword",
			keywords: "-<script>alert(1)</script>",
			baseURL:  "/",
			expected: template.HTML("-&lt;script&gt;alert(1)&lt;/script&gt;"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatKeywordsFunc(tt.keywords, tt.baseURL)
			if result != tt.expected {
				t.Errorf("formatKeywordsFunc(%q, %q) = %q, want %q", tt.keywords, tt.baseURL, result, tt.expected)
			}
		})
	}
}

func TestFormatDependencyFunc(t *testing.T) {
	tests := []struct {
		name     string
		dep      string
		baseURL  string
		expected template.HTML
	}{
		{
			name:     "empty",
			dep:      "",
			baseURL:  "/",
			expected: template.HTML(""),
		},
		{
			name:     "simple dep",
			dep:      "dev-libs/kirigami",
			baseURL:  "/",
			expected: template.HTML("<a href=\"/packages/dev-libs/kirigami/\">dev-libs/kirigami</a>"),
		},
		{
			name:     "dep with version",
			dep:      ">=dev-qt/qtbase-6.10.1:6[gui,network,widgets]",
			baseURL:  "/",
			expected: template.HTML("<a href=\"/packages/dev-qt/qtbase/\">&gt;=dev-qt/qtbase-6.10.1:6[gui,network,widgets]</a>"),
		},
		{
			name:     "multiple deps and newlines",
			dep:      "dev-libs/kirigami\n>=dev-qt/qtbase-6",
			baseURL:  "/",
			expected: template.HTML("<a href=\"/packages/dev-libs/kirigami/\">dev-libs/kirigami</a>\n<a href=\"/packages/dev-qt/qtbase/\">&gt;=dev-qt/qtbase-6</a>"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDependencyFunc(tt.baseURL, tt.dep)
			if result != tt.expected {
				t.Errorf("formatDependencyFunc() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResolveBreadcrumbsFunc(t *testing.T) {
	tests := []struct {
		name        string
		currentPath string
		crumbs      []g2.Breadcrumb
		expected    []g2.Breadcrumb
	}{
		{
			name:        "root to root",
			currentPath: "/",
			crumbs: []g2.Breadcrumb{
				{Name: "Home", Path: "/"},
			},
			expected: []g2.Breadcrumb{
				{Name: "Home", Path: "/", URL: ""},
			},
		},
		{
			name:        "sub to root",
			currentPath: "/categories/app-admin/foo",
			crumbs: []g2.Breadcrumb{
				{Name: "Home", Path: "/"},
				{Name: "Categories", Path: "/categories"},
				{Name: "app-admin", Path: "/categories/app-admin"},
				{Name: "foo", Path: "/categories/app-admin/foo"},
			},
			expected: []g2.Breadcrumb{
				{Name: "Home", Path: "/", URL: "../../../"},
				{Name: "Categories", Path: "/categories", URL: "../../"},
				{Name: "app-admin", Path: "/categories/app-admin", URL: "../"},
				{Name: "foo", Path: "/categories/app-admin/foo", URL: ""},
			},
		},
		{
			name:        "sibling to sibling",
			currentPath: "/categories/app-admin/foo",
			crumbs: []g2.Breadcrumb{
				{Name: "bar", Path: "/categories/app-admin/bar"},
			},
			expected: []g2.Breadcrumb{
				{Name: "bar", Path: "/categories/app-admin/bar", URL: "../bar/"},
			},
		},
		{
			name:        "fallback to URL if Path empty",
			currentPath: "/categories/app-admin/foo",
			crumbs: []g2.Breadcrumb{
				{Name: "Home", URL: "../../../"},
			},
			expected: []g2.Breadcrumb{
				{Name: "Home", Path: "", URL: "../../../"},
			},
		},
		{
			name:        "trailing slash bug",
			currentPath: "/categories/app-admin/foo/",
			crumbs: []g2.Breadcrumb{
				{Name: "foo", Path: "/categories/app-admin/foo"},
			},
			expected: []g2.Breadcrumb{
				{Name: "foo", Path: "/categories/app-admin/foo", URL: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveBreadcrumbsFunc(tt.currentPath, tt.crumbs)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected len %d, got %d", len(tt.expected), len(result))
			}
			for i, r := range result {
				if r.Name != tt.expected[i].Name || r.Path != tt.expected[i].Path || r.URL != tt.expected[i].URL {
					t.Errorf("expected crumb[%d] = %+v, got %+v", i, tt.expected[i], r)
				}
			}
		})
	}
}
