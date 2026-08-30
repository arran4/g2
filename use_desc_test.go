package g2

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseUseDesc(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected *UseDesc
	}{
		{
			name: "Basic usage",
			content: `# Copyright 1999-2024 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2
# Keep them sorted
X - Add support for X11
Xaw3d - Add support for the 3d athena widget set
a52 - Enable support for decoding ATSC A/52 streams used in DVD
`,
			expected: &UseDesc{
				Flags: map[string]string{
					"X":     "Add support for X11",
					"Xaw3d": "Add support for the 3d athena widget set",
					"a52":   "Enable support for decoding ATSC A/52 streams used in DVD",
				},
				HeaderLines: []string{
					"# Copyright 1999-2024 Gentoo Authors",
					"# Distributed under the terms of the GNU General Public License v2",
					"# Keep them sorted",
				},
			},
		},
		{
			name: "With blank lines in header",
			content: `# Header line 1

# Header line 2

flag1 - description 1
flag2 - description 2
`,
			expected: &UseDesc{
				Flags: map[string]string{
					"flag1": "description 1",
					"flag2": "description 2",
				},
				HeaderLines: []string{
					"# Header line 1",
					"",
					"# Header line 2",
					"",
				},
			},
		},
		{
			name: "Unsupported or poorly formatted flag line",
			content: `# Header
flag1 - desc1
invalid_line_without_dash
flag2 - desc2
`,
			expected: &UseDesc{
				Flags: map[string]string{
					"flag1": "desc1",
					"flag2": "desc2",
				},
				HeaderLines: []string{
					"# Header",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ud, err := ParseUseDesc(strings.NewReader(tt.content))
			if err != nil {
				t.Fatalf("ParseUseDesc() error = %v", err)
			}

			if len(ud.Flags) != len(tt.expected.Flags) {
				t.Fatalf("Flags len = %v, want %v", len(ud.Flags), len(tt.expected.Flags))
			}

			for k, v := range tt.expected.Flags {
				if ud.Flags[k] != v {
					t.Errorf("Flags[%q] = %v, want %v", k, ud.Flags[k], v)
				}
			}

			if len(ud.HeaderLines) != len(tt.expected.HeaderLines) {
				t.Fatalf("HeaderLines len = %v, want %v", len(ud.HeaderLines), len(tt.expected.HeaderLines))
			}

			for i, v := range tt.expected.HeaderLines {
				if ud.HeaderLines[i] != v {
					t.Errorf("HeaderLines[%d] = %v, want %v", i, ud.HeaderLines[i], v)
				}
			}
		})
	}
}

func TestWriteUseDesc(t *testing.T) {
	ud := &UseDesc{
		Flags: map[string]string{
			"zebra": "Zebra description",
			"apple": "Apple description",
			"cat":   "Cat description",
		},
		HeaderLines: []string{
			"# Header line 1",
			"# Header line 2",
		},
	}

	var buf bytes.Buffer
	if err := ud.Write(&buf); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	expected := `# Header line 1
# Header line 2
apple - Apple description
cat - Cat description
zebra - Zebra description
`

	if buf.String() != expected {
		t.Errorf("Write() = \n%v\nwant \n%v", buf.String(), expected)
	}
}
