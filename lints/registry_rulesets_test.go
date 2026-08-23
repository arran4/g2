package lints_test

import (
	"os"
	"testing"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	_ "github.com/arran4/g2/lints/ebuild"
)

func TestBuiltInRuleSetsRetrieval(t *testing.T) {
	for _, id := range []string{"default", "gentoo-main", "guru"} {
		rs, ok := lints.GetRuleSet(id)
		if !ok || rs == nil {
			t.Errorf("expected built-in ruleset %q to exist", id)
			continue
		}
		if rs.ID != id {
			t.Errorf("expected ruleset ID %q, got %q", id, rs.ID)
		}
	}

	_, ok := lints.GetRuleSet("non-existent-ruleset")
	if ok {
		t.Errorf("expected non-existent ruleset to return false")
	}
}

func TestPerformLintingResultsWithRuleSet_UnknownRuleSet(t *testing.T) {
	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "foo",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "# Copyright 2026 Example Authors\nEAPI=8\n",
				},
			},
		},
	}

	_, err := lints.PerformLintingResultsWithRuleSet("", pkg, "unknown-ruleset-xyz")
	if err == nil {
		t.Fatalf("expected error for unknown ruleset, got nil")
	}
}

func TestPerformLintingResultsWithRuleSet_DefaultVsGentooMain(t *testing.T) {
	// A package with a non-Gentoo copyright header.
	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "foo",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "# Copyright 2026 Third-Party Project Authors\n# Distributed under the terms of the Apache License, Version 2.0\n\nEAPI=8\n",
				},
			},
		},
	}

	// 1. Under "default": Generic CopyrightHeader is enabled and should pass. GentooCopyrightHeader is NOT enabled.
	resultsDefault, err := lints.PerformLintingResultsWithRuleSet("", pkg, "default")
	if err != nil {
		t.Fatalf("unexpected error running default ruleset: %v", err)
	}

	for _, res := range resultsDefault {
		if res.RuleMetadata.ID == "CopyrightHeader" {
			t.Errorf("CopyrightHeader unexpectedly failed under default: %v", res.Message)
		}
		if res.RuleMetadata.ID == "GentooCopyrightHeader" {
			t.Errorf("GentooCopyrightHeader should not run under default ruleset")
		}
	}

	// 2. Under "gentoo-main": Generic CopyrightHeader passes, but GentooCopyrightHeader fails because it's not Gentoo Authors.
	resultsGentoo, err := lints.PerformLintingResultsWithRuleSet("", pkg, "gentoo-main")
	if err != nil {
		t.Fatalf("unexpected error running gentoo-main ruleset: %v", err)
	}

	var foundGentooCopyrightWarning bool
	for _, res := range resultsGentoo {
		if res.RuleMetadata.ID == "CopyrightHeader" {
			t.Errorf("CopyrightHeader unexpectedly failed under gentoo-main: %v", res.Message)
		}
		if res.RuleMetadata.ID == "GentooCopyrightHeader" {
			foundGentooCopyrightWarning = true
			if res.RuleMetadata.Severity != lints.SeverityWarning {
				t.Errorf("expected GentooCopyrightHeader severity %q, got %q", lints.SeverityWarning, res.RuleMetadata.Severity)
			}
		}
	}
	if !foundGentooCopyrightWarning {
		t.Errorf("expected GentooCopyrightHeader warning under gentoo-main ruleset")
	}

	// 3. Under "guru": Generic CopyrightHeader passes, GentooCopyrightHeader is not enabled.
	resultsGuru, err := lints.PerformLintingResultsWithRuleSet("", pkg, "guru")
	if err != nil {
		t.Fatalf("unexpected error running guru ruleset: %v", err)
	}
	for _, res := range resultsGuru {
		if res.RuleMetadata.ID == "GentooCopyrightHeader" {
			t.Errorf("GentooCopyrightHeader should not run under guru ruleset")
		}
	}
}

func TestPerformLintingResultsWithRuleSet_SeverityOverrideAndDisabled(t *testing.T) {
	// Register a custom RuleSet to test dynamic enablement and severity override.
	customRS := &lints.RuleSet{
		ID:          "custom-test-ruleset",
		Description: "RuleSet for testing severity override and rule disabling.",
		Rules: map[string]lints.RuleSetEntry{
			"CopyrightHeader": {
				Enabled:  true,
				Severity: lints.SeverityError, // Override Notice -> Error
			},
			"GentooCopyrightHeader": {
				Enabled:  false, // Explicitly disabled
				Severity: lints.SeverityWarning,
			},
		},
	}
	lints.RegisterRuleSet(customRS)

	// Missing copyright header should trigger CopyrightHeader with overridden Error severity.
	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "badpkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "EAPI=8\n",
				},
			},
		},
	}

	results, err := lints.PerformLintingResultsWithRuleSet("", pkg, "custom-test-ruleset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var foundCopyrightHeader bool
	for _, res := range results {
		if res.RuleMetadata.ID == "CopyrightHeader" {
			foundCopyrightHeader = true
			if res.RuleMetadata.Severity != lints.SeverityError {
				t.Errorf("expected severity Error overridden from RuleSet, got %s", res.RuleMetadata.Severity)
			}
		}
		if res.RuleMetadata.ID == "GentooCopyrightHeader" {
			t.Errorf("GentooCopyrightHeader was explicitly disabled and should not execute")
		}
	}

	if !foundCopyrightHeader {
		t.Errorf("expected CopyrightHeader result for missing header")
	}
}

func TestLegacyLintRulesRunUnchanged(t *testing.T) {
	// A package with invalid category layout to trigger legacy CategorySanity check.
	pkg := &g2.PackageData{
		Category: "invalid-category",
		Name:     "foo",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "# Copyright 2026 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8\n",
				},
			},
		},
	}

	results, err := lints.PerformLintingResultsWithRuleSet("", pkg, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var foundCategorySanity bool
	for _, res := range results {
		if res.RuleMetadata.ID == "CategorySanity" {
			foundCategorySanity = true
		}
	}
	if !foundCategorySanity {
		t.Errorf("expected legacy CategorySanity rule to execute in default ruleset")
	}
}

func TestAbsentRuleSetManagedRuleDoesNotRun(t *testing.T) {
	emptyRS := &lints.RuleSet{
		ID:          "empty-test-ruleset",
		Description: "RuleSet with no managed rules enabled.",
		Rules:       map[string]lints.RuleSetEntry{},
	}
	lints.RegisterRuleSet(emptyRS)

	// Missing copyright header would trigger CopyrightHeader if enabled.
	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "badpkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "EAPI=8\n",
				},
			},
		},
	}

	results, err := lints.PerformLintingResultsWithRuleSet("", pkg, "empty-test-ruleset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, res := range results {
		if res.RuleMetadata.ID == "CopyrightHeader" || res.RuleMetadata.ID == "GentooCopyrightHeader" {
			t.Errorf("rule %q should not execute when absent from RuleSet", res.RuleMetadata.ID)
		}
	}
}

func TestMetadataAwareNonManagedRuleContinuesRunning(t *testing.T) {
	// UseExpandRule is MetadataAware but not RuleSetManaged.
	// It should continue running under the default RuleSet even though it is not listed in default's Rules map.
	tmpDir := t.TempDir()
	importDir := tmpDir + "/profiles/desc"
	if err := os.MkdirAll(importDir, 0755); err != nil {
		t.Fatalf("failed to create profiles/desc: %v", err)
	}
	descContent := "valid_flag - A valid flag\n"
	if err := os.WriteFile(importDir+"/custom_expand.desc", []byte(descContent), 0644); err != nil {
		t.Fatalf("failed to write desc file: %v", err)
	}

	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "pkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "# Copyright 2026 Gentoo Authors\n# Distributed under the terms of the GNU General Public License v2\n\nEAPI=8\n",
					Vars: map[string]string{
						"IUSE": "custom_expand_unsupported_flag",
					},
				},
			},
		},
	}

	results, err := lints.PerformLintingResultsWithRuleSet(tmpDir, pkg, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var foundUseExpand bool
	for _, res := range results {
		if res.RuleMetadata.ID == "UseExpandUnsupported" {
			foundUseExpand = true
			break
		}
	}
	if !foundUseExpand {
		t.Errorf("expected metadata-aware non-ruleset-managed rule UseExpandUnsupported to run under default ruleset")
	}
}

func TestPerformLintingWithRuleSet_StringWarnings(t *testing.T) {
	pkg := &g2.PackageData{
		Category: "app-misc",
		Name:     "badpkg",
		Versions: []g2.VersionData{
			{
				Version: "1.0",
				Ebuild: &g2.Ebuild{
					RawText: "EAPI=8\n",
				},
			},
		},
	}

	warnings, err := lints.PerformLintingWithRuleSet("", pkg, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) == 0 {
		t.Errorf("expected warnings for missing copyright header under default ruleset")
	}

	_, err = lints.PerformLintingWithRuleSet("", pkg, "invalid-rs-id")
	if err == nil {
		t.Fatalf("expected error for invalid ruleset ID, got nil")
	}
}
