package main

import (
	"testing"

)

func TestParseDependsLinkFunc(t *testing.T) {
	tests := []struct {
		name     string
		deps     []string
		expected string
	}{
		{
			name:     "Basic versioned dependency",
			deps:     []string{"<sys-libs/zlib-1.2.3"},
			expected: `<a href="/packages/sys-libs/zlib/">&lt;sys-libs/zlib-1.2.3</a>`,
		},
		{
			name:     "Basic unversioned dependency with number",
			deps:     []string{">=app-arch/7zip-21.0-r1"},
			expected: `<a href="/packages/app-arch/7zip/">&gt;=app-arch/7zip-21.0-r1</a>`,
		},
		{
			name:     "XSS test",
			deps:     []string{"<script>alert(1)</script>"},
			expected: `<a href="/packages/script&gt;alert(1)&lt;/script&gt;/">&lt;script&gt;alert(1)&lt;/script&gt;</a>`,
		},
		{
			name:     "Multiple deps",
			deps:     []string{"<sys-libs/zlib-1.2.3", ">=app-arch/7zip"},
			expected: `<a href="/packages/sys-libs/zlib/">&lt;sys-libs/zlib-1.2.3</a><br>
<a href="/packages/app-arch/7zip/">&gt;=app-arch/7zip</a>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(parseDependsLinkFunc(tt.deps))
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}
