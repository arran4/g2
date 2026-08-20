package ebuild

import (
	"testing"

	"github.com/arran4/g2"
	"github.com/stretchr/testify/assert"
)

func TestCharacterSetLintRule(t *testing.T) {
	rule := &CharacterSetLintRule{}

	t.Run("Valid UTF-8", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "good-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						RawText: "# Copyright 1999-2024 Gentoo Authors\n" +
							"# Distributed under the terms of the GNU General Public License v2\n\n" +
							"EAPI=8\n\n" +
							"DESCRIPTION=\"A valid package\"\n" +
							"HOMEPAGE=\"https://example.com\"\n" +
							"SRC_URI=\"\"\n\n" +
							"LICENSE=\"GPL-2\"\n" +
							"SLOT=\"0\"\n" +
							"KEYWORDS=\"~amd64\"\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Empty(t, results)
	})

	t.Run("Invalid UTF-8", func(t *testing.T) {
		pkg := &g2.PackageData{
			Category: "app-test",
			Name:     "invalid-pkg",
			Versions: []g2.VersionData{
				{
					Version: "1.0",
					Ebuild: &g2.Ebuild{
						// Contains invalid UTF-8 sequence \xff\xfe
						RawText: "DESCRIPTION=\"Invalid char \xff\xfe\"\n",
					},
				},
			},
		}

		results := rule.Lint("/tmp/dummyrepo", pkg, nil)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Message, "is not valid UTF-8")
	})
}
