package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestHeadTailLintRule(t *testing.T) {
	rule := &HeadTailLintRule{}

	tests := []struct {
		name     string
		ebuild   string
		expected int
	}{
		{
			name:     "clean ebuild",
			ebuild:   `echo "hello" | grep "h"`,
			expected: 0,
		},
		{
			name:     "head -N deprecated syntax",
			ebuild:   `head -5 input.txt`,
			expected: 1,
		},
		{
			name:     "tail -N deprecated syntax",
			ebuild:   `tail -5 input.txt`,
			expected: 1,
		},
		{
			name:     "tail clumsy computing",
			ebuild:   `tail -n $(($(wc -l in.txt | awk '{print $1}') - 5)) in.txt > out.txt`,
			expected: 1,
		},
		{
			name:     "head | sed chaining",
			ebuild:   `head -n 5 input.txt | sed -e 's/foo/bar/g' > output.txt`,
			expected: 1,
		},
		{
			name:     "sed | head chaining",
			ebuild:   `foo=$(sed -n -e '/somestring/p' input.txt | head -n 1)`,
			expected: 1,
		},
		{
			name:     "sed | tail chaining",
			ebuild:   `sed -n -e '/somestring/p' input.txt | tail -n 1`,
			expected: 1,
		},
		{
			name:     "valid head syntax",
			ebuild:   `head -n 5 input.txt`,
			expected: 0,
		},
		{
			name:     "valid tail syntax",
			ebuild:   `tail -n 5 input.txt`,
			expected: 0,
		},
		{
			name:     "valid sed early exit",
			ebuild:   `foo=$(sed -n -e '/somestring/{ p ; q }' input.txt )`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Category: "sys-apps",
				Name:     "testpkg",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: tt.ebuild,
						},
					},
				},
			}

			results := rule.Lint("", pkg)

			if len(results) != tt.expected {
				t.Errorf("expected %d results, got %d", tt.expected, len(results))
				for _, r := range results {
					t.Logf("Result: %s", r.Message)
				}
			}
		})
	}
}
