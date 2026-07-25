package main

import (
	"testing"
	"github.com/arran4/g2"
)

// Add basic test to cover search_engine.go logic.
// Note that most of this file is already tested in search_test.go, but we want
// to be sure we are covering the newly modified mathVersion logic.
func TestMatchVersion(t *testing.T) {
	e := NewSearchEngine()

	doc := SearchDocument{
		Version:        "1.0.0",
		VersionSortKey: g2.PadVersion("1.0.0"),
	}

	tests := []struct {
		queryVersion string
		expected     bool
	}{
		{"==1.0.0", true},
		{"=1.0.0", true},
		{">=1.0.0", true},
		{"<=1.0.0", true},
		{"<1.1.0", true},
		{">1.1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.queryVersion, func(t *testing.T) {
			if actual := e.matchVersion(doc, tt.queryVersion); actual != tt.expected {
				t.Errorf("matchVersion(%q) = %v, expected %v", tt.queryVersion, actual, tt.expected)
			}
		})
	}
}

// Add a test to prove that matchSequence in search_engine.go doesn't break
func TestMatchSequence(t *testing.T) {
	e := NewSearchEngine()

	doc := SearchDocument{
		SearchText: "some test sequence string",
	}

	if !e.matchSequence(doc, "test sequence") {
		t.Errorf("expected sequence to match")
	}

	if e.matchSequence(doc, "sequence test") {
		t.Errorf("expected reversed sequence not to match")
	}
}
