package lints

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/arran4/g2"
)

func TestRuleMetadataReferencesSerialization(t *testing.T) {
	meta := RuleMetadata{
		ID:          "TestRule",
		Title:       "Test Rule Title",
		Description: "Test rule description",
		References: []RuleReference{
			{URL: "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0206", Label: "Gentoo QA Policy Guide PG0206"},
			{URL: "https://devmanual.gentoo.org/", Label: "Gentoo Devmanual"},
		},
		Severity: SeverityWarning,
		Source:   SourceQA,
		Tags:     []string{"test", "policy"},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"references"`) {
		t.Errorf("expected serialized JSON to contain 'references', got %s", jsonStr)
	}
	if strings.Contains(jsonStr, `"url":`) && !strings.Contains(jsonStr, `"references":[{"url":`) {
		t.Errorf("expected url to only appear inside references, got %s", jsonStr)
	}

	// Unmarshal back
	var unmarshaled RuleMetadata
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(unmarshaled.References, meta.References) {
		t.Errorf("expected references %v, got %v", meta.References, unmarshaled.References)
	}
}

func TestRuleMetadataLegacyURLCompatibility(t *testing.T) {
	// Test legacy single "url" field unmarshaling
	legacyJSON := `{
		"id": "LegacyRule",
		"title": "Legacy Title",
		"description": "Legacy Description",
		"url": "https://devmanual.gentoo.org/legacy",
		"severity": "Warning",
		"source": "g2"
	}`

	var meta RuleMetadata
	if err := json.Unmarshal([]byte(legacyJSON), &meta); err != nil {
		t.Fatalf("failed to unmarshal legacy JSON: %v", err)
	}

	expected := []RuleReference{
		{URL: "https://devmanual.gentoo.org/legacy"},
	}
	if !reflect.DeepEqual(meta.References, expected) {
		t.Errorf("expected references %v, got %v", expected, meta.References)
	}

	// Test legacy "urls" array field unmarshaling
	legacyURLsJSON := `{
		"id": "LegacyURLsRule",
		"title": "Legacy URLs Title",
		"description": "Legacy URLs Description",
		"urls": ["https://example.com/1", "https://example.com/2"],
		"severity": "Error",
		"source": "qa"
	}`

	var metaURLs RuleMetadata
	if err := json.Unmarshal([]byte(legacyURLsJSON), &metaURLs); err != nil {
		t.Fatalf("failed to unmarshal legacy URLs JSON: %v", err)
	}

	expectedURLs := []RuleReference{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
	}
	if !reflect.DeepEqual(metaURLs.References, expectedURLs) {
		t.Errorf("expected references %v, got %v", expectedURLs, metaURLs.References)
	}

	// Test that explicit "references" is preserved when legacy "url"/"urls" are also present
	bothJSON := `{
		"id": "BothRule",
		"title": "Title",
		"description": "Desc",
		"references": [{"url": "https://new.example", "label": "New"}],
		"url": "https://old.example",
		"urls": ["https://old-array.example"]
	}`

	var metaBoth RuleMetadata
	if err := json.Unmarshal([]byte(bothJSON), &metaBoth); err != nil {
		t.Fatalf("failed to unmarshal JSON with both fields: %v", err)
	}

	expectedBoth := []RuleReference{
		{URL: "https://new.example", Label: "New"},
	}
	if !reflect.DeepEqual(metaBoth.References, expectedBoth) {
		t.Errorf("expected references to be preserved as %v, got %v", expectedBoth, metaBoth.References)
	}
}

func TestRegisterAndGetAllRules(t *testing.T) {
	oldMetadata := registeredMetadata
	defer func() {
		registeredMetadata = oldMetadata
	}()

	testMeta := RuleMetadata{
		ID:          "CustomRegisteredRule",
		Title:       "Custom Registered Title",
		Description: "Custom Registered Description",
		References: []RuleReference{
			{URL: "https://example.com/rule", Label: "Example Rule"},
		},
		Severity: SeverityWarning,
		Source:   SourceG2,
	}
	RegisterRuleMetadata(testMeta)

	rules := GetAllRules()
	found := false
	for _, r := range rules {
		if r.ID == "CustomRegisteredRule" {
			found = true
			if !reflect.DeepEqual(r.References, testMeta.References) {
				t.Errorf("expected references %v, got %v", testMeta.References, r.References)
			}
			break
		}
	}
	if !found {
		t.Error("expected registered rule to be returned by GetAllRules()")
	}
}

type mockQALintRule struct {
	ruleMeta RuleMetadata
}

func (m *mockQALintRule) Lint(repoDir string, pkg *g2.PackageData) []LintResult {
	return []LintResult{{RuleMetadata: m.ruleMeta, Message: "basic rule"}}
}

func (m *mockQALintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []LintResult {
	if qa != nil {
		if val, ok := qa.Policies["PG123"]; ok {
			return []LintResult{{RuleMetadata: m.ruleMeta, Message: "qa rule " + val}}
		}
		return []LintResult{{RuleMetadata: m.ruleMeta, Message: "qa rule not found"}}
	}
	return []LintResult{{RuleMetadata: m.ruleMeta, Message: "qa rule no qa"}}
}

func TestPerformLintingWithQA(t *testing.T) {
	oldRules := lintRules
	defer func() {
		lintRules = oldRules
	}()

	tmpDir := t.TempDir()
	metaDir := filepath.Join(tmpDir, "metadata")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}

	if err := os.WriteFile(filepath.Join(metaDir, "qa-policy.conf"), []byte("[policy]\nPG123 = yes\n"), 0644); err != nil {
		t.Fatalf("write qa-policy.conf: %v", err)
	}

	mockMeta := RuleMetadata{
		ID:          "MockQARule",
		Title:       "Mock QA Rule",
		Description: "Mock QA Description",
		Severity:    SeverityWarning,
		Source:      SourceQA,
	}
	lintRules = []LintRule{&mockQALintRule{ruleMeta: mockMeta}}

	warnings := PerformLinting(tmpDir, &g2.PackageData{})
	if len(warnings) != 1 || warnings[0] != "qa rule yes" {
		t.Errorf("Expected ['qa rule yes'], got %v", warnings)
	}

	results := PerformLintingResults(tmpDir, &g2.PackageData{})
	if len(results) != 1 || results[0].Message != "qa rule yes" {
		t.Errorf("Expected result message 'qa rule yes', got %v", results)
	}
}
