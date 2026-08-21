package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
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
