package g2

import (
	"bytes"
	"reflect"
	"testing"
)

func TestParsePackageDeprecatedReader(t *testing.T) {
	input := `# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

# This file specifies packages that are considered deprecated (but not
# masked yet).

# Michał Górny <mgorny@gentoo.org> (2026-03-25)
# Backwards compatibility package for pkg_resources that have been
# removed from >=dev-python/setuptools-82.  Please migrate to
# importlib.{metadata,resources} and/or dev-python/packaging.
dev-python/pkg-resources

# Michał Górny <mgorny@gentoo.org> (2025-11-25)
# The package has turned into complete AI slop.  Every subsequent
# release introduces serious quality issues, and potential security
# concerns.  Please ask upstreams to move away from it.
dev-python/autobahn
`

	expected := []PackageDeprecated{
		{
			Package:     "dev-python/pkg-resources",
			Reason:      "Backwards compatibility package for pkg_resources that have been removed from >=dev-python/setuptools-82. Please migrate to importlib.{metadata,resources} and/or dev-python/packaging.",
			Date:        "2026-03-25",
			Author:      "Michał Górny",
			AuthorEmail: "mgorny@gentoo.org",
		},
		{
			Package:     "dev-python/autobahn",
			Reason:      "The package has turned into complete AI slop. Every subsequent release introduces serious quality issues, and potential security concerns. Please ask upstreams to move away from it.",
			Date:        "2025-11-25",
			Author:      "Michał Górny",
			AuthorEmail: "mgorny@gentoo.org",
		},
	}

	res, err := parsePackageDeprecatedReader(bytes.NewBufferString(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(res))
	}

	for i := range expected {
		if !reflect.DeepEqual(res[i], expected[i]) {
			t.Errorf("mismatch at index %d:\nExpected: %+v\nGot: %+v", i, expected[i], res[i])
		}
	}
}

func TestSerializePackageDeprecated(t *testing.T) {
	data := []PackageDeprecated{
		{
			Package:     "dev-python/pkg-resources",
			Reason:      "Backwards compatibility package for pkg_resources that have been removed from >=dev-python/setuptools-82. Please migrate to importlib.{metadata,resources} and/or dev-python/packaging.",
			Date:        "2026-03-25",
			Author:      "Michał Górny",
			AuthorEmail: "mgorny@gentoo.org",
		},
		{
			Package:     "dev-python/autobahn",
			Reason:      "The package has turned into complete AI slop. Every subsequent release introduces serious quality issues, and potential security concerns. Please ask upstreams to move away from it.",
			Date:        "2025-11-25",
			Author:      "Michał Górny",
			AuthorEmail: "mgorny@gentoo.org",
		},
	}

	var buf bytes.Buffer
	err := SerializePackageDeprecated(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error during serialization: %v", err)
	}

	// Now parse it back
	parsed, err := parsePackageDeprecatedReader(&buf)
	if err != nil {
		t.Fatalf("unexpected error during re-parsing: %v", err)
	}

	if len(parsed) != len(data) {
		t.Fatalf("expected %d entries, got %d", len(data), len(parsed))
	}

	for i := range data {
		if !reflect.DeepEqual(parsed[i], data[i]) {
			t.Errorf("mismatch after serialization at index %d:\nExpected: %+v\nGot:      %+v", i, data[i], parsed[i])
		}
	}
}
