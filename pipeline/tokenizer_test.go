package pipeline

import (
	"reflect"
	"testing"
)

func TestTokenizePipeline(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
		wantError bool
	}{
		{"quoted pipes", "get('http://foo|bar') | trim", []string{"get('http://foo|bar')", "trim"}, false},
		{"nested/argument parentheses", "replace('a', '(b)') | trim", []string{"replace('a', '(b)')", "trim"}, false},
		{"escaped quotes", "replace('a\\'b', 'c')", []string{"replace('a\\'b', 'c')"}, false},
		{"escaped backslashes", "replace('a\\\\b', 'c')", []string{"replace('a\\\\b', 'c')"}, false},
		{"unterminated single quotes", "replace('a, 'b')", nil, true},
		{"unterminated double quotes", "replace(\"a, \"b\")", nil, true},
		{"unbalanced parentheses 1", "replace('a', 'b'", nil, true},
		{"unbalanced parentheses 2", "replace('a', 'b'))", nil, true},
		{"empty stages 1 leading", "| a", nil, true},
		{"empty stages 2 trailing", "a |", nil, true},
		{"empty stages 3 doubled", "a | | b", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TokenizePipeline(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("expected error: %v, got: %v", tt.wantError, err)
			}
			if !tt.wantError && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("expected: %v, got: %v", tt.expected, got)
			}
		})
	}
}

func TestParseReplaceArgs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  []string
		wantError bool
	}{
		{"malformed argument lists 1", "'a', 'b', 'c'", nil, true},
		{"malformed argument lists 2", "'a'", nil, true},
		{"malformed argument lists 3", "a, b", nil, true}, // unquoted
		{"quoted commas", "'a,b', 'c'", []string{"a,b", "c"}, false},
		{"quoted pipes", "'a|b', 'c'", []string{"a|b", "c"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReplaceArgs(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("expected error: %v, got: %v", tt.wantError, err)
			}
			if !tt.wantError && !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("expected: %v, got: %v", tt.expected, got)
			}
		})
	}
}
