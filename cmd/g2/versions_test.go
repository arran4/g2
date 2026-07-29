package main

import (
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

func TestGentooToSemantic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3_alpha1", "1.2.3-alpha1"},
		{"1.2.3_beta2-r1", "1.2.3-r1-beta2"},
		{"1.2.3-alpha1", "1.2.3-alpha1"},
		{"1.2.3_alpha1-r1", "1.2.3-r1-alpha1"},
		{"1.2.3_beta", "1.2.3-beta"},
		{"1.2.3", "1.2.3"},
		{"1.2.3-r1", "1.2.3-r1"},
		{"1.2.3-r1_alpha1", "1.2.3-r1-alpha1"},
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

func TestSemanticToGentoo_PreAndTrailing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
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
