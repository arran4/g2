package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/arran4/g2/lints"
)

func TestParseLintQuery(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedQuery LintQuery
	}{
		{
			name:  "basic package",
			query: "app-misc/foo",
			expectedQuery: LintQuery{
				RepoPath: ".",
				Category: "app-misc",
				Package:  "foo",
			},
		},
		{
			name:  "version operator",
			query: ">=app-misc/foo-1.2.3",
			expectedQuery: LintQuery{
				RepoPath:  ".",
				Category:  "app-misc",
				Package:   "foo",
				VersionOp: ">=",
				Version:   "1.2.3",
			},
		},
		{
			name:  "wildcard version",
			query: "app-misc/foo-v3",
			expectedQuery: LintQuery{
				RepoPath:  ".",
				Category:  "app-misc",
				Package:   "foo",
				VWildcard: "3",
			},
		},
		{
			name:  "repo suffix with repo fallback",
			query: "app-misc/foo::guru",
			expectedQuery: LintQuery{
				RepoPath: "/var/db/repos/guru",
				Category: "app-misc",
				Package:  "foo",
			},
		},
		{
			name:  "eclass query fallback",
			query: "foo.eclass",
			expectedQuery: LintQuery{
				RepoPath: ".",
				Package:  "foo.eclass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parseLintQuery(tt.query, ".")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(*q, tt.expectedQuery) {
				t.Errorf("expected %+v, got %+v", tt.expectedQuery, *q)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create os.Pipe: %v", err)
	}
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	runErr := fn()
	_ = w.Close()
	os.Stdout = oldStdout
	output := <-outChan
	_ = r.Close()
	return output, runErr
}

func TestCmdLintList(t *testing.T) {
	cfg := &MainArgConfig{}

	// Test text format
	textOut, err := captureStdout(t, func() error {
		return cfg.cmdLintList([]string{"--format", "text"})
	})
	if err != nil {
		t.Fatalf("cmdLintList text format failed: %v", err)
	}
	if !strings.Contains(textOut, "Available Lint Rules:") {
		t.Errorf("expected text output to contain header 'Available Lint Rules:', got:\n%s", textOut)
	}
	// Verify that at least one known rule reference is rendered with its label
	if !strings.Contains(textOut, "Reference (Gentoo QA Policy Guide PG0001): https://projects.gentoo.org/qa/policy-guide/dependencies.html#pg0001") {
		t.Errorf("expected text output to contain labeled PG0001 reference, got:\n%s", textOut)
	}

	// Test json format
	jsonOut, err := captureStdout(t, func() error {
		return cfg.cmdLintList([]string{"--format", "json"})
	})
	if err != nil {
		t.Fatalf("cmdLintList json format failed: %v", err)
	}

	var rawEntries []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonOut), &rawEntries); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\nOutput was:\n%s", err, jsonOut)
	}
	if len(rawEntries) == 0 {
		t.Fatal("expected non-empty list of rules in JSON output")
	}

	foundLabelledRef := false
	for _, entry := range rawEntries {
		id, _ := entry["id"].(string)

		// Assert no top-level legacy url or urls field is emitted
		if _, hasURL := entry["url"]; hasURL {
			t.Errorf("rule %q emitted legacy top-level 'url' field in JSON output", id)
		}
		if _, hasURLs := entry["urls"]; hasURLs {
			t.Errorf("rule %q emitted legacy top-level 'urls' field in JSON output", id)
		}

		if rawRefs, ok := entry["references"]; ok && rawRefs != nil {
			refs, ok := rawRefs.([]interface{})
			if !ok {
				t.Errorf("rule %q has non-array 'references' field", id)
				continue
			}
			for _, refItem := range refs {
				refMap, ok := refItem.(map[string]interface{})
				if !ok {
					t.Errorf("rule %q reference item is not an object", id)
					continue
				}
				u, ok := refMap["url"].(string)
				if !ok || u == "" {
					t.Errorf("rule %q has empty or invalid reference url", id)
				}
				if lbl, ok := refMap["label"].(string); ok && lbl != "" {
					foundLabelledRef = true
				}
			}
		}
	}

	if !foundLabelledRef {
		t.Error("expected at least one rule with a labelled reference in JSON output")
	}
}

func TestSeverityThresholdLogic(t *testing.T) {
	makeResult := func(sev lints.Severity) lints.LintResult {
		return lints.LintResult{
			RuleMetadata: lints.RuleMetadata{
				Severity: sev,
			},
		}
	}

	testCases := []struct {
		desc       string
		results    []lints.LintResult
		failSev    string
		expectFail bool
	}{
		{
			desc:       "Info only + warning threshold => pass",
			results:    []lints.LintResult{makeResult(lints.SeverityInfo)},
			failSev:    "warning",
			expectFail: false,
		},
		{
			desc:       "Info only + info threshold => fail",
			results:    []lints.LintResult{makeResult(lints.SeverityInfo)},
			failSev:    "info",
			expectFail: true,
		},
		{
			desc:       "Error only + warning threshold => fail",
			results:    []lints.LintResult{makeResult(lints.SeverityError)},
			failSev:    "warning",
			expectFail: true,
		},
		{
			desc:       "Error only + error threshold => fail",
			results:    []lints.LintResult{makeResult(lints.SeverityError)},
			failSev:    "error",
			expectFail: true,
		},
		{
			desc:       "Notice + Error + warning threshold => fail",
			results:    []lints.LintResult{makeResult(lints.SeverityNotice), makeResult(lints.SeverityError)},
			failSev:    "warning",
			expectFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := hasFailingLintResults(tc.results, tc.failSev)
			if got != tc.expectFail {
				t.Errorf("hasFailingLintResults() = %v, want %v for %s", got, tc.expectFail, tc.desc)
			}
		})
	}
}
func TestSeverityLevel(t *testing.T) {
	if severityLevel("error") != 3 {
		t.Errorf("expected 3")
	}
	if severityLevel("warning") != 2 {
		t.Errorf("expected 2")
	}
	if severityLevel("notice") != 1 {
		t.Errorf("expected 1")
	}
	if severityLevel("info") != 0 {
		t.Errorf("expected 0")
	}

	if severityLevel("error") <= severityLevel("warning") {
		t.Errorf("error should be > warning")
	}
	if severityLevel("warning") <= severityLevel("notice") {
		t.Errorf("warning should be > notice")
	}
	if severityLevel("notice") <= severityLevel("info") {
		t.Errorf("notice should be > info")
	}
}

func TestCmdLintFailSeverity(t *testing.T) {
	cfg := &MainArgConfig{}

	overlayPath := t.TempDir()

	if err := os.MkdirAll(overlayPath+"/app-misc/warning-pkg", 0755); err != nil {
		t.Fatalf("failed to create warning pkg dir: %v", err)
	}
	if err := os.MkdirAll(overlayPath+"/app-misc/notice-pkg", 0755); err != nil {
		t.Fatalf("failed to create notice pkg dir: %v", err)
	}
	if err := os.MkdirAll(overlayPath+"/profiles", 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	_ = os.WriteFile(overlayPath+"/profiles/repo_name", []byte("dummy-repo\n"), 0644)

	warningEbuild := []byte(`
# Copyright 2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="short"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/warning-pkg-1.0.ebuild", warningEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/Manifest", []byte(""), 0644)

	noticeEbuild := []byte(`
# Copyright 2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="A sufficiently descriptive test package"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="~amd64"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/notice-pkg/notice-pkg-1.0.ebuild", noticeEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/notice-pkg/Manifest", []byte(""), 0644)

	metadataContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE pkgmetadata SYSTEM "https://www.gentoo.org/dtd/metadata.dtd">
<pkgmetadata>
	<maintainer type="person">
		<email>test@example.com</email>
	</maintainer>
</pkgmetadata>`)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/metadata.xml", metadataContent, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/notice-pkg/metadata.xml", metadataContent, 0644)

	// Error only package (no metadata.xml)
	if err := os.MkdirAll(overlayPath+"/app-misc/error-pkg", 0755); err != nil {
		t.Fatalf("failed to create error pkg dir: %v", err)
	}
	errorEbuild := []byte(`# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="A sufficiently descriptive test package"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/error-pkg/error-pkg-1.0.ebuild", errorEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/error-pkg/Manifest", []byte(""), 0644)

	// Mixed Notice + Error
	if err := os.MkdirAll(overlayPath+"/app-misc/mixed-pkg", 0755); err != nil {
		t.Fatalf("failed to create mixed pkg dir: %v", err)
	}
	mixedEbuild := []byte(`# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="A sufficiently descriptive test package"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="~amd64"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/mixed-pkg/mixed-pkg-1.0.ebuild", mixedEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/mixed-pkg/Manifest", []byte(""), 0644)

	// Pre-test validations
	outWarning, _ := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/warning-pkg"})
	})
	if strings.Contains(outWarning, `"severity": "Error"`) {
		t.Fatalf("Fixture setup failed: warning-pkg contains Errors!\nOutput: %s", outWarning)
	}
	if !strings.Contains(outWarning, `"severity": "Warning"`) {
		t.Fatalf("Fixture setup failed: warning-pkg does not contain a Warning!\nOutput: %s", outWarning)
	}

	outNotice, _ := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/notice-pkg"})
	})
	if strings.Contains(outNotice, `"severity": "Error"`) || strings.Contains(outNotice, `"severity": "Warning"`) {
		t.Fatalf("Fixture setup failed: notice-pkg contains Errors/Warnings!\nOutput: %s", outNotice)
	}
	if !strings.Contains(outNotice, `"severity": "Notice"`) {
		t.Fatalf("Fixture setup failed: notice-pkg does not contain a Notice!\nOutput: %s", outNotice)
	}

	outError, _ := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/error-pkg"})
	})
	var errorResults []lints.LintResult
	if err := json.Unmarshal([]byte(outError), &errorResults); err != nil {
		t.Fatalf("Fixture setup failed: error-pkg json unmarshal error: %v\nOutput: %s", err, outError)
	}
	if len(errorResults) == 0 {
		t.Fatalf("Fixture setup failed: error-pkg produced no findings!\nOutput: %s", outError)
	}
	for _, res := range errorResults {
		if res.RuleMetadata.Severity != lints.SeverityError {
			t.Fatalf("Fixture setup failed: error-pkg contains non-Error finding: %+v", res)
		}
	}

	outMixed, _ := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/mixed-pkg"})
	})
	var mixedResults []lints.LintResult
	if err := json.Unmarshal([]byte(outMixed), &mixedResults); err != nil {
		t.Fatalf("Fixture setup failed: mixed-pkg json unmarshal error: %v\nOutput: %s", err, outMixed)
	}
	hasNotice := false
	hasError := false
	for _, res := range mixedResults {
		if res.RuleMetadata.Severity == lints.SeverityNotice {
			hasNotice = true
		}
		if res.RuleMetadata.Severity == lints.SeverityError {
			hasError = true
		}
		if res.RuleMetadata.Severity == lints.SeverityWarning {
			t.Fatalf("Fixture setup failed: mixed-pkg contains Warning finding: %+v", res)
		}
	}
	if !hasNotice || !hasError {
		t.Fatalf("Fixture setup failed: mixed-pkg must contain both Notice and Error findings (hasNotice=%v, hasError=%v)\nOutput: %s", hasNotice, hasError, outMixed)
	}

	tests := []struct {
		name         string
		pkgTarget    string
		failSeverity string
		expectFail   bool
	}{
		{"warning pkg, default warning", "app-misc/warning-pkg", "warning", true},
		{"warning pkg, --fail-severity=error", "app-misc/warning-pkg", "error", false},
		{"notice pkg, default warning", "app-misc/notice-pkg", "warning", false},
		{"notice pkg, --fail-severity=notice", "app-misc/notice-pkg", "notice", true},
		{"error-only + default warning => fail", "app-misc/error-pkg", "warning", true},
		{"error-only + --fail-severity=error => fail", "app-misc/error-pkg", "error", true},
		{"mixed Notice + Error + default warning => fail", "app-misc/mixed-pkg", "warning", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := captureStdout(t, func() error {
				args := []string{"--format", "text", "--fail-severity", tt.failSeverity, overlayPath, tt.pkgTarget}
				return cfg.cmdLintPackage(args)
			})

			hasErr := err != nil
			if hasErr != tt.expectFail {
				t.Errorf("expected fail: %v, got: %v (err: %v)", tt.expectFail, hasErr, err)
				t.Logf("Output: %s", out)
			}
		})
	}

	// Verify legacy runOldLint honors --fail-severity as well
	t.Run("legacy notice pkg, default warning -> pass", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return cfg.runOldLint([]string{"--format", "text", "--fail-severity", "warning", overlayPath, "app-misc/notice-pkg"})
		})
		if err != nil {
			t.Errorf("expected legacy lint to pass on notice-pkg with default warning, got: %v (output: %s)", err, out)
		}
	})
	t.Run("legacy notice pkg, --fail-severity=notice -> fail", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return cfg.runOldLint([]string{"--format", "text", "--fail-severity", "notice", overlayPath, "app-misc/notice-pkg"})
		})
		if err == nil {
			t.Errorf("expected legacy lint to fail on notice-pkg with --fail-severity=notice, got output: %s", out)
		}
	})
}
func TestCmdLintFailSeverityOutputFormats(t *testing.T) {
	cfg := &MainArgConfig{}

	overlayPath := t.TempDir()

	if err := os.MkdirAll(overlayPath+"/app-misc/notice-pkg", 0755); err != nil {
		t.Fatalf("failed to create notice pkg dir: %v", err)
	}
	if err := os.MkdirAll(overlayPath+"/profiles", 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	_ = os.WriteFile(overlayPath+"/profiles/repo_name", []byte("dummy-repo\n"), 0644)

	noticeEbuild := []byte(`
# Copyright 2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="A sufficiently descriptive test package"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="~amd64"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/notice-pkg/notice-pkg-1.0.ebuild", noticeEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/notice-pkg/Manifest", []byte(""), 0644)

	metadataContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE pkgmetadata SYSTEM "https://www.gentoo.org/dtd/metadata.dtd">
<pkgmetadata>
	<maintainer type="person">
		<email>test@example.com</email>
	</maintainer>
</pkgmetadata>`)
	_ = os.WriteFile(overlayPath+"/app-misc/notice-pkg/metadata.xml", metadataContent, 0644)

	// Test github-actions format returns 0 with default failSeverity, but still outputs a notice
	outNotice, err := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "github-actions", overlayPath, "app-misc/notice-pkg"})
	})

	if err != nil {
		t.Fatalf("expected github-actions format to exit 0 with notice, got err: %v", err)
	}
	if !strings.Contains(outNotice, "::notice ") {
		t.Errorf("expected github-actions format to still output a notice, got: %s", outNotice)
	}

	// Test json format returns 0 with default failSeverity, but still outputs a notice
	outJson, errJson := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/notice-pkg"})
	})
	if errJson != nil {
		t.Fatalf("expected json format to exit 0 with notice, got err: %v", errJson)
	}
	if !strings.Contains(outJson, `"severity": "Notice"`) {
		t.Errorf("expected json format to still output a notice, got: %s", outJson)
	}

	// Error JSON assertion
	if err := os.MkdirAll(overlayPath+"/app-misc/error-pkg", 0755); err != nil {
		t.Fatalf("failed to create error pkg dir: %v", err)
	}
	errorEbuild := []byte(`# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="A sufficiently descriptive test package"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/error-pkg/error-pkg-1.0.ebuild", errorEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/error-pkg/Manifest", []byte(""), 0644)

	outJsonErr, errJsonErr := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/error-pkg"})
	})
	if errJsonErr == nil {
		t.Fatalf("expected json format to exit non-0 with error")
	}
	if !strings.Contains(outJsonErr, `"severity": "Error"`) {
		t.Errorf("expected json format to still output an error, got: %s", outJsonErr)
	}
}

func TestCmdLintSuppression(t *testing.T) {
	cfg := &MainArgConfig{}
	overlayPath := t.TempDir()

	if err := os.MkdirAll(overlayPath+"/app-misc/warning-pkg", 0755); err != nil {
		t.Fatalf("failed to create warning pkg dir: %v", err)
	}
	if err := os.MkdirAll(overlayPath+"/profiles", 0755); err != nil {
		t.Fatalf("failed to create profiles dir: %v", err)
	}
	_ = os.WriteFile(overlayPath+"/profiles/repo_name", []byte("dummy-repo\n"), 0644)

	warningEbuild := []byte(`
# Copyright 2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="short"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/warning-pkg-1.0.ebuild", warningEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/Manifest", []byte(""), 0644)

	metadataContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE pkgmetadata SYSTEM "https://www.gentoo.org/dtd/metadata.dtd">
<pkgmetadata>
	<maintainer type="person">
		<email>test@example.com</email>
	</maintainer>
</pkgmetadata>`)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/metadata.xml", metadataContent, 0644)

	// Verify unsuppressed baseline fails under default warning threshold and contains MissingDescription
	outBaseline, errBaseline := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/warning-pkg"})
	})
	if errBaseline == nil {
		t.Fatalf("expected unsuppressed warning fixture to fail, but got success. Output: %s", outBaseline)
	}
	var baselineResults []lints.LintResult
	if err := json.Unmarshal([]byte(outBaseline), &baselineResults); err != nil {
		t.Fatalf("failed to parse baseline JSON: %v", err)
	}
	foundMissingDesc := false
	for _, res := range baselineResults {
		if strings.EqualFold(string(res.RuleMetadata.ID), "MissingDescription") {
			foundMissingDesc = true
		}
	}
	if !foundMissingDesc {
		t.Fatalf("expected baseline results to contain MissingDescription finding: %s", outBaseline)
	}

	// 1. --disable-rule MissingDescription suppresses the finding and succeeds
	t.Run("disable-rule MissingDescription", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return cfg.cmdLintPackage([]string{
				"--format", "json",
				"--disable-rule", "MissingDescription",
				overlayPath, "app-misc/warning-pkg",
			})
		})
		if err != nil {
			t.Errorf("expected command to succeed with --disable-rule MissingDescription, got err: %v\nOutput: %s", err, out)
		}
		var results []lints.LintResult
		if err := json.Unmarshal([]byte(out), &results); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v\nOutput: %s", err, out)
		}
		for _, res := range results {
			if strings.EqualFold(string(res.RuleMetadata.ID), "MissingDescription") ||
				strings.Contains(res.Message, "DESCRIPTION is suspiciously short") {
				t.Errorf("expected MissingDescription finding to be absent, but found: %+v", res)
			}
		}
	})

	// 2. --ignore-tag site-quality suppresses the finding and succeeds
	t.Run("ignore-tag site-quality", func(t *testing.T) {
		out, err := captureStdout(t, func() error {
			return cfg.cmdLintPackage([]string{
				"--format", "json",
				"--ignore-tag", "site-quality",
				overlayPath, "app-misc/warning-pkg",
			})
		})
		if err != nil {
			t.Errorf("expected command to succeed with --ignore-tag site-quality, got err: %v\nOutput: %s", err, out)
		}
		var results []lints.LintResult
		if err := json.Unmarshal([]byte(out), &results); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v\nOutput: %s", err, out)
		}
		for _, res := range results {
			if strings.EqualFold(string(res.RuleMetadata.ID), "MissingDescription") ||
				strings.Contains(res.Message, "DESCRIPTION is suspiciously short") {
				t.Errorf("expected MissingDescription finding to be absent, but found: %+v", res)
			}
		}
	})
}

func TestCmdLintTargetedAccessLimit(t *testing.T) {
	// A package with a noticeable problem (no maintainer -> Error, missing vars -> Warning, etc)
	overlayPath := "../../testdata/test_overlay"

	// Create tracking fs
	tfs := &trackingFS{
		FS:       os.DirFS(overlayPath),
		Accessed: make(map[string]bool),
	}

	targetMap := make(map[string]bool)
	targetMap["app-misc/foo"] = true

	_, err := parseRepo(tfs, ".", "Test Overlay", false, nil, TargetPackages(targetMap))
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}

	for k := range tfs.Accessed {
		if strings.Contains(k, "bad_category") {
			t.Errorf("targeted lint incorrectly accessed unrelated path: %s", k)
		}
	}

	if !tfs.Accessed["app-misc/foo"] {
		t.Errorf("targeted lint failed to access target package 'app-misc/foo'")
	}
}

// trackingFS wraps an fs.FS to track directory and file accesses.
type trackingFS struct {
	fs.FS
	Accessed map[string]bool
}

func (t *trackingFS) Open(name string) (fs.File, error) {
	t.Accessed[name] = true
	return t.FS.Open(name)
}

func (t *trackingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	t.Accessed[name] = true
	return fs.ReadDir(t.FS, name)
}

func TestCmdLintTargetedSyntaxes(t *testing.T) {
	cfg := &MainArgConfig{}
	overlayPath := "../../testdata/test_overlay"

	// 1. New package syntax
	outNew, errNew := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/foo"})
	})
	if errNew == nil {
		t.Errorf("expected new syntax to fail due to foo errors, but it passed")
	}

	// 2. Legacy package syntax
	outLegacy, errLegacy := captureStdout(t, func() error {
		return cfg.runOldLint([]string{"--format", "json", overlayPath, "app-misc/foo"})
	})
	if errLegacy == nil {
		t.Errorf("expected legacy syntax to fail due to foo errors, but it passed")
	}

	// 3. Exact Category Target syntax
	outCategory, errCat := captureStdout(t, func() error {
		return cfg.runOldLint([]string{"--format", "json", overlayPath, "app-misc"})
	})
	if errCat == nil {
		t.Errorf("expected category syntax to fail due to foo errors, but it passed")
	}

	// Check that none of these reached bad_category
	for _, out := range []string{outNew, outLegacy, outCategory} {
		if strings.Contains(out, "bad_category") {
			t.Errorf("targeted lint incorrectly leaked into unrelated 'bad_category'. Output snippet: %s", out[:min(len(out), 200)])
		}
	}
}

func TestCmdLintEffectiveSeverityOverride(t *testing.T) {
	cfg := &MainArgConfig{}
	overlayPath := t.TempDir()

	_ = os.MkdirAll(overlayPath+"/app-misc/warning-pkg", 0755)
	_ = os.MkdirAll(overlayPath+"/profiles", 0755)
	_ = os.MkdirAll(overlayPath+"/metadata", 0755)
	_ = os.WriteFile(overlayPath+"/profiles/repo_name", []byte("dummy-repo\n"), 0644)

	// Provide a qa-policy.conf overriding PG0002 from Warning to Notice
	qaPolicy := []byte(`
[policy]
PG0002 = notice
`)
	_ = os.WriteFile(overlayPath+"/metadata/qa-policy.conf", qaPolicy, 0644)

	// Trigger PG0002: =-dependency with no revision
	warningEbuild := []byte(`# Copyright 1999-2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

EAPI=8
DESCRIPTION="A proper description"
HOMEPAGE="https://example.com"
LICENSE="MIT"
SLOT="0"
KEYWORDS="amd64"
DEPEND="=app-misc/some-dep-1.0"
`)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/warning-pkg-1.0.ebuild", warningEbuild, 0644)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/Manifest", []byte(""), 0644)
	metadataContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE pkgmetadata SYSTEM "https://www.gentoo.org/dtd/metadata.dtd">
<pkgmetadata>
	<maintainer type="person">
		<email>test@example.com</email>
	</maintainer>
</pkgmetadata>`)
	_ = os.WriteFile(overlayPath+"/app-misc/warning-pkg/metadata.xml", metadataContent, 0644)

	// Since we overrode PG0002 to Notice, it should NOT fail default (Warning) threshold
	out, err := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "json", overlayPath, "app-misc/warning-pkg"})
	})

	if err != nil {
		t.Fatalf("expected to PASS because effective severity of PG0002 is Notice. err: %v\nOut: %s", err, out)
	}

	// Parse JSON structurally
	var results []lints.LintResult
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v\nOutput was:\n%s", err, out)
	}

	// Locate the actual PG0002 / dependency-no-revision result
	var pg0002Result *lints.LintResult
	for i := range results {
		if strings.EqualFold(string(results[i].RuleMetadata.ID), "DependencyNoRevision") ||
			strings.Contains(results[i].Message, "PG0002") {
			pg0002Result = &results[i]
			break
		}
	}
	if pg0002Result == nil {
		t.Fatalf("expected to find PG0002 / DependencyNoRevision finding in JSON output: %s", out)
	}

	// Assert that specific result has effective severity Notice
	if pg0002Result.RuleMetadata.Severity != lints.SeverityNotice {
		t.Errorf("expected PG0002 effective severity to be Notice, got: %s", pg0002Result.RuleMetadata.Severity)
	}

	// Ensure no unrelated Warning or Error is present
	for _, res := range results {
		if res.RuleMetadata.Severity == lints.SeverityWarning || res.RuleMetadata.Severity == lints.SeverityError {
			t.Errorf("unexpected finding with severity %s: %s (id: %s)", res.RuleMetadata.Severity, res.Message, res.RuleMetadata.ID)
		}
	}

	// But it should fail if fail-severity is Notice
	outNotice, errNotice := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--format", "text", "--fail-severity", "notice", overlayPath, "app-misc/warning-pkg"})
	})
	if errNotice == nil {
		t.Errorf("expected to FAIL when fail-severity is Notice. Out: %s", outNotice)
	}
}

func TestCmdLintInvalidFailSeverity(t *testing.T) {
	cfg := &MainArgConfig{}

	// Newer path
	_, err := captureStdout(t, func() error {
		return cfg.cmdLintPackage([]string{"--fail-severity", "invalid_sev", ".", "app-misc/foo"})
	})
	if err == nil || !strings.Contains(err.Error(), "invalid fail-severity") {
		t.Errorf("expected invalid fail-severity error from package cmd, got: %v", err)
	}

	// Legacy path
	_, err = captureStdout(t, func() error {
		return cfg.runOldLint([]string{"--fail-severity", "invalid_sev", ".", "app-misc/foo"})
	})
	if err == nil || !strings.Contains(err.Error(), "invalid fail-severity") {
		t.Errorf("expected invalid fail-severity error from legacy cmd, got: %v", err)
	}
}
