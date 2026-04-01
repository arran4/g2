package g2

import (
	"bytes"
	_ "embed"
	"reflect"
	"testing"
)

//go:embed testdata/package.mask
var packageMaskTestInput string

func TestParsePackageMaskedReader(t *testing.T) {
	expected := []PackageMasked{
		{
			Reason:      "Backwards compatibility package for pkg_resources that have been removed from >=dev-python/setuptools-82. Please migrate to importlib.{metadata,resources} and/or dev-python/packaging.",
			Date:        "2026-03-25",
			Author:      "Jane Doe",
			AuthorEmail: "jane.doe@example.com",
			Entries: []PackageMaskedEntry{
				{Package: "dev-python/pkg-resources"},
			},
		},
		{
			Reason:      "The package has turned into complete AI slop. Every subsequent release introduces serious quality issues, and potential security concerns. Please ask upstreams to move away from it.",
			Date:        "2025-11-25",
			Author:      "John Smith",
			AuthorEmail: "john.smith@example.com",
			Entries: []PackageMaskedEntry{
				{Package: "dev-python/autobahn"},
			},
		},
	}

	res, err := parsePackageMaskedReader(bytes.NewBufferString(packageMaskTestInput))
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

func TestSerializePackageMasked(t *testing.T) {
	data := []PackageMasked{
		{
			Reason:      "Backwards compatibility package for pkg_resources that have been removed from >=dev-python/setuptools-82. Please migrate to importlib.{metadata,resources} and/or dev-python/packaging.",
			Date:        "2026-03-25",
			Author:      "Jane Doe",
			AuthorEmail: "jane.doe@example.com",
			Entries: []PackageMaskedEntry{
				{Package: "dev-python/pkg-resources"},
			},
		},
		{
			Reason:      "The package has turned into complete AI slop. Every subsequent release introduces serious quality issues, and potential security concerns. Please ask upstreams to move away from it.",
			Date:        "2025-11-25",
			Author:      "John Smith",
			AuthorEmail: "john.smith@example.com",
			Entries: []PackageMaskedEntry{
				{Package: "dev-python/autobahn"},
			},
		},
	}

	var buf bytes.Buffer
	err := SerializePackageMasked(&buf, data)
	if err != nil {
		t.Fatalf("unexpected error during serialization: %v", err)
	}

	// Now parse it back
	parsed, err := parsePackageMaskedReader(&buf)
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
