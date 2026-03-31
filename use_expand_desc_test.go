package g2

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseUseExpandDesc(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		content  string
		expected *UseExpandDesc
	}{
		{
			name:   "Basic usage",
			prefix: "abi_mips",
			content: `# Copyright 1999-2013 Gentoo Foundation.
# Distributed under the terms of the GNU General Public License v2

# This file contains descriptions of ABI_MIPS USE_EXPAND flags.

# Keep it sorted.
n32 - 64-bit (32-bit pointer) libraries
n64 - 64-bit libraries
o32 - 32-bit libraries
`,
			expected: &UseExpandDesc{
				Prefix: "abi_mips",
				Flags: map[string]string{
					"n32": "64-bit (32-bit pointer) libraries",
					"n64": "64-bit libraries",
					"o32": "32-bit libraries",
				},
				HeaderLines: []string{
					"# Copyright 1999-2013 Gentoo Foundation.",
					"# Distributed under the terms of the GNU General Public License v2",
					"",
					"# This file contains descriptions of ABI_MIPS USE_EXPAND flags.",
					"",
					"# Keep it sorted.",
				},
			},
		},
		{
			name:   "Empty content",
			prefix: "empty",
			content: `
`,
			expected: &UseExpandDesc{
				Prefix:      "empty",
				Flags:       map[string]string{},
				HeaderLines: []string{""},
			},
		},
		{
			name:   "Malformed line ignored",
			prefix: "malformed",
			content: `flag1 - Description 1
malformed line without dash
flag2 - Description 2
`,
			expected: &UseExpandDesc{
				Prefix: "malformed",
				Flags: map[string]string{
					"flag1": "Description 1",
					"flag2": "Description 2",
				},
				HeaderLines: []string{},
			},
		},
		{
			name:   "Comments mixed with flags",
			prefix: "mixed",
			content: `# header
flag1 - desc1
# inline comment (ignored as header)
flag2 - desc2
`,
			expected: &UseExpandDesc{
				Prefix: "mixed",
				Flags: map[string]string{
					"flag1": "desc1",
					"flag2": "desc2",
				},
				HeaderLines: []string{
					"# header",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ud, err := ParseUseExpandDesc(tt.prefix, strings.NewReader(tt.content))
			if err != nil {
				t.Fatalf("ParseUseExpandDesc() error = %v", err)
			}

			if ud.Prefix != tt.expected.Prefix {
				t.Errorf("Prefix = %v, expected %v", ud.Prefix, tt.expected.Prefix)
			}

			if len(ud.Flags) != len(tt.expected.Flags) {
				t.Errorf("Flags count = %v, expected %v", len(ud.Flags), len(tt.expected.Flags))
			}
			for k, v := range tt.expected.Flags {
				if ud.Flags[k] != v {
					t.Errorf("Flags[%q] = %v, expected %v", k, ud.Flags[k], v)
				}
			}

			if len(ud.HeaderLines) != len(tt.expected.HeaderLines) {
				t.Errorf("HeaderLines count = %v, expected %v", len(ud.HeaderLines), len(tt.expected.HeaderLines))
			}
			for i, line := range tt.expected.HeaderLines {
				if i < len(ud.HeaderLines) && ud.HeaderLines[i] != line {
					t.Errorf("HeaderLines[%d] = %v, expected %v", i, ud.HeaderLines[i], line)
				}
			}
		})
	}
}

func TestWriteUseExpandDesc(t *testing.T) {
	ud := &UseExpandDesc{
		Prefix: "test",
		Flags: map[string]string{
			"b": "Desc B",
			"a": "Desc A",
			"c": "Desc C",
		},
		HeaderLines: []string{
			"# Header 1",
			"# Header 2",
		},
	}

	var buf bytes.Buffer
	if err := ud.Write(&buf); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	expected := `# Header 1
# Header 2
a - Desc A
b - Desc B
c - Desc C
`
	if buf.String() != expected {
		t.Errorf("Write() generated \n%s\nexpected \n%s", buf.String(), expected)
	}
}
