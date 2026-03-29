package lints

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arran4/g2"
)

type MockLintRule struct{}

func (m *MockLintRule) Lint(repoDir string, pkg *g2.PackageData) []LintResult {
	return []LintResult{{Message: "basic rule"}}
}

func (m *MockLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []LintResult {
	if qa != nil {
		if val, ok := qa.Policies["PG123"]; ok {
			return []LintResult{{Message: "qa rule " + val}}
		}
		return []LintResult{{Message: "qa rule " + "not found"}}
	}
	return []LintResult{{Message: "qa rule no qa"}}
}

func TestPerformLintingWithQA(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "qa-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	metaDir := filepath.Join(tmpDir, "metadata")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(metaDir, "qa-policy.conf"), []byte("[policy]\nPG123 = yes"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	lintRules = []LintRule{&MockLintRule{}}

	warnings := PerformLinting(tmpDir, &g2.PackageData{})
	if len(warnings) != 1 || warnings[0] != "qa rule yes" {
		t.Errorf("Expected 'qa rule yes', got %v", warnings)
	}
}
