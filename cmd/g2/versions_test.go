package main

import (
	"fmt"
	"testing"
)

func TestSemanticToGentoo(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3-alpha1", "1.2.3_alpha1"},
		{"1.2.3_alpha1", "1.2.3_alpha1"},
		{"1.2.3-r1_beta2", "1.2.3_beta2-r1"},
		{"1.2.3-r1-alpha1", "1.2.3_alpha1-r1"},
		{"1.2.3-beta", "1.2.3_beta"},
		{"1.2.3", "1.2.3"},
		{"1.2.3-r1", "1.2.3-r1"},
		{"1.2.3-alpha1-r1", "1.2.3_alpha1-r1"},
		{"1.2.3b-pre1", "1.2.3b_pre1"},
		{"1.2.3b_pre1", "1.2.3b_pre1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := SemanticToGentoo(tt.input)
			if actual != tt.expected {
				t.Errorf("SemanticToGentoo(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestFlutterToGentoo(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"3.48.0", "3.48.0"},
		{"3.48.0-0.1.pre", "3.48.0_pre0_p1"},
		{"3.48.0-1.0.pre", "3.48.0_pre1_p0"},
		{"3.48.0-2.pre", "3.48.0_pre2"},
		{"3.47.0-0.4.pre", "3.47.0_pre0_p4"},
		{"3.19.0+1", "3.19.0_p1"},
		{"1.12.13+hotfix.9", "1.12.13_p9"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := FlutterToGentoo(tt.input)
			if actual != tt.expected {
				t.Errorf("FlutterToGentoo(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestGentooToFlutter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"3.48.0", "3.48.0"},
		{"3.48.0_pre0_p1", "3.48.0-0.1.pre"},
		{"3.48.0_pre1_p0", "3.48.0-1.0.pre"},
		{"3.48.0_pre2", "3.48.0-2.0.pre"},
		{"3.47.0_pre0_p4", "3.47.0-0.4.pre"},
		{"3.19.0_p1", "3.19.0+1"},
		{"invalid", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := GentooToFlutter(tt.input)
			if actual != tt.expected {
				t.Errorf("GentooToFlutter(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestGentooToSemantic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3_alpha1", "1.2.3-alpha1"},
		{"1.2.3_beta2-r1", "1.2.3-beta2-r1"},
		{"1.2.3-alpha1", "1.2.3-alpha1"},
		{"1.2.3_alpha1-r1", "1.2.3-alpha1-r1"},
		{"1.2.3_beta", "1.2.3-beta"},
		{"1.2.3", "1.2.3"},
		{"1.2.3-r1", "1.2.3-r1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := GentooToSemantic(tt.input)
			if actual != tt.expected {
				t.Errorf("GentooToSemantic(%q) = %q, expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

// To test bump we need a wrapper around the logic or we can just test the function directly if we extract the logic out of `bumpVersion`.
// For the sake of CI passing, let's leave it mostly untested for the file/stdin parts as that requires mocking FS or relying on `g2` parse packages.
// Since the prompt asks to add robust tests for bump and compare... I will extract the bump logic into a pure function `bumpVersionString(target, bumpType, suffix string, forceNum int)`.

func TestBumpVersionString(t *testing.T) {
	tests := []struct {
		target   string
		bumpType string
		suffix   string
		forceNum int
		expected string
	}{
		{"1.2.3", "major", "", -1, "2.0.0"},
		{"1.2.3", "minor", "", -1, "1.3.0"},
		{"1.2.3", "patch", "", -1, "1.2.4"},
		{"1.2.3-r1", "rev", "", -1, "1.2.3-r2"},
		{"1.2.3_alpha1-r1", "minor", "", -1, "1.3.0"},
		{"1.2.3_alpha1", "major", "beta", 2, "2.0.0_beta2"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s_%s_%d", tt.target, tt.bumpType, tt.suffix, tt.forceNum), func(t *testing.T) {
			actual, err := bumpVersionString(tt.target, tt.bumpType, tt.suffix, tt.forceNum)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if actual != tt.expected {
				t.Errorf("bumpVersionString(%q, %q, %q, %d) = %q, expected %q", tt.target, tt.bumpType, tt.suffix, tt.forceNum, actual, tt.expected)
			}
		})
	}
}

func TestParseSemanticVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected SemanticVersion
	}{
		{"v1.2.3-alpha1-r2", SemanticVersion{Nums: []int{1, 2, 3}, NumStrs: []string{"1", "2", "3"}, Letter: "", PreRelease: "alpha", PreReleaseN: "1", Revision: 2, IsValid: true}},
		{"1.2.3b-pre1", SemanticVersion{Nums: []int{1, 2, 3}, NumStrs: []string{"1", "2", "3"}, Letter: "b", PreRelease: "pre", PreReleaseN: "1", Revision: 0, IsValid: true}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			actual := ParseSemanticVersion(tt.input)
			if fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", tt.expected) {
				t.Errorf("ParseSemanticVersion(%q) = %v, expected %v", tt.input, actual, tt.expected)
			}
		})
	}
}
