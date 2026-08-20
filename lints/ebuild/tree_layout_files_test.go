package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestTreeLayoutFilesLintRule(t *testing.T) {
	rule := &TreeLayoutFilesLintRule{}

	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "foo",
		Files: []g2.FileData{
			{Name: "valid-name.patch", Path: "app-misc/foo/files/valid-name.patch"},
			{Name: ".hidden", Path: "app-misc/foo/files/.hidden"},
			{Name: "-test", Path: "app-misc/foo/files/-test"},
			{Name: "+test", Path: "app-misc/foo/files/+test"},
			{Name: "te!st", Path: "app-misc/foo/files/te!st"},
			{Name: "a", Path: "app-misc/foo/files/a"},
		},
	}

	results := rule.Lint("", pkg, nil)

	expectedMessages := []string{
		"file '.hidden' starts with a dot, hyphen, or plus sign",
		"file '-test' starts with a dot, hyphen, or plus sign",
		"file '+test' starts with a dot, hyphen, or plus sign",
		"file 'te!st' contains characters outside [A-Za-z0-9._+-]",
	}

	if len(results) != len(expectedMessages) {
		t.Fatalf("expected %d results, got %d", len(expectedMessages), len(results))
	}

	for i, expected := range expectedMessages {
		if results[i].Message != expected {
			t.Errorf("expected message %q, got %q", expected, results[i].Message)
		}
	}
}
