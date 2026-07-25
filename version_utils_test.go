package g2

import (
	"testing"
)

func TestPadVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.0.0", "0000000001.0000000000.0000000000"},
		{"1.2.3-r1", "0000000001.0000000002.0000000003+r0000000001"},
		{"10.0-r99", "0000000010.0000000000+r0000000099"},
		{"", ""},
		{"v1", "v0000000001"},
	}

	for _, tt := range tests {
		actual := PadVersion(tt.input)
		if actual != tt.expected {
			t.Errorf("PadVersion(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestSplitVersionOp(t *testing.T) {
	tests := []struct {
		input       string
		expectedVer string
		expectedOp  string
	}{
		{">=1.0", "1.0", ">="},
		{"<2.0-r1", "2.0-r1", "<"},
		{"~3.1", "3.1", "~"},
		{"=4.0", "4.0", "="},
		{"5.0", "5.0", "=="},
	}

	for _, tt := range tests {
		ver, op := SplitVersionOp(tt.input)
		if ver != tt.expectedVer || op != tt.expectedOp {
			t.Errorf("SplitVersionOp(%q) = (%q, %q), expected (%q, %q)", tt.input, ver, op, tt.expectedVer, tt.expectedOp)
		}
	}
}
