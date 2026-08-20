package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"github.com/stretchr/testify/assert"
)

func TestEbuildHeaderLintRule(t *testing.T) {
	rule := &EbuildHeaderLintRule{}

	tests := []struct {
		name     string
		pkg      *g2.PackageData
		expected int // expected number of errors
	}{
		{
			name: "Valid header",
			pkg: &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "# Copyright 1999-2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8",
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "Missing header",
			pkg: &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "EAPI=8",
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "Valid header single year",
			pkg: &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "# Copyright 2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8",
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "Valid header alternative contributor",
			pkg: &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "# Copyright 2024 Main Contributor\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8",
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "Valid header range alternative contributor",
			pkg: &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "# Copyright 1999-2024 Main Contributor\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8",
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "Malformed copyright string",
			pkg: &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "# Copyright 200X Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8",
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "CVS header included",
			pkg: &g2.PackageData{
				Category: "app-misc",
				Name:     "foo",
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							RawText: "# Copyright 1999-2024 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n# $Header$\n\nEAPI=8",
						},
					},
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := rule.Lint("", tt.pkg)
			assert.Len(t, results, tt.expected)
			if tt.expected > 0 {
				assert.Equal(t, lints.SeverityNotice, results[0].RuleMetadata.Severity)
			}
		})
	}
}
