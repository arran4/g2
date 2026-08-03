package g2

import (
	"reflect"
	"testing"
)

func TestDeduplicateStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"empty", []string{}, []string{}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"all duplicates", []string{"a", "a", "a"}, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeduplicateStrings(tt.input); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("DeduplicateStrings() = %v, want %v", got, tt.expected)
			}
		})
	}
}
