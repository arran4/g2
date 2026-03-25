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

func TestParseUseLocalDesc(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected *UseLocalDesc
	}{
		{
			name: "Basic usage",
			content: `# Copyright 1999-2024 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2
app-admin/conky:X - Enable X11 support
app-admin/conky:apcupsd - Enable apcupsd support
app-editors/vim:X - Enable X11 support
app-misc/foo:bar - Enable bar
`,
			expected: &UseLocalDesc{
				Flags: map[string]map[string]string{
					"app-admin/conky": {
						"X":       "Enable X11 support",
						"apcupsd": "Enable apcupsd support",
					},
					"app-editors/vim": {
						"X": "Enable X11 support",
					},
					"app-misc/foo": {
						"bar": "Enable bar",
					},
				},
				HeaderLines: []string{
					"# Copyright 1999-2024 Gentoo Authors",
					"# Distributed under the terms of the GNU General Public License v2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ud, err := ParseUseLocalDesc(strings.NewReader(tt.content))
			if err != nil {
				t.Fatalf("ParseUseLocalDesc() error = %v", err)
			}

			if len(ud.Flags) != len(tt.expected.Flags) {
				t.Fatalf("Packages len = %v, want %v", len(ud.Flags), len(tt.expected.Flags))
			}

			for pkg, flags := range tt.expected.Flags {
				if len(ud.Flags[pkg]) != len(flags) {
					t.Fatalf("Flags[%q] len = %v, want %v", pkg, len(ud.Flags[pkg]), len(flags))
				}
				for k, v := range flags {
					if ud.Flags[pkg][k] != v {
						t.Errorf("Flags[%q][%q] = %v, want %v", pkg, k, ud.Flags[pkg][k], v)
					}
				}
			}
		})
	}
}

func TestWriteUseLocalDesc(t *testing.T) {
	ud := &UseLocalDesc{
		Flags: map[string]map[string]string{
			"cat/pkg-c": {
				"z-flag": "description z",
				"a-flag": "description a",
			},
			"cat/pkg-a": {
				"flag1": "description 1",
			},
		},
		HeaderLines: []string{
			"# Header line 1",
		},
	}

	var buf bytes.Buffer
	if err := ud.Write(&buf); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	expected := `# Header line 1
cat/pkg-a:flag1 - description 1
cat/pkg-c:a-flag - description a
cat/pkg-c:z-flag - description z
`

	if buf.String() != expected {
		t.Errorf("Write() = \n%v\nwant \n%v", buf.String(), expected)
	}
}

func TestParseUseLocalDesc_Complex(t *testing.T) {
	// A complex test representing real-world quirks found in Gentoo's repository.
	// For example, extra spacing around the hyphen, colons in descriptions, etc.
	content := `# Header
sys-apps/systemd:pkcs11 - Enable PKCS#11 smartcard support
dev-lang/python:tk - Build tkinter (Tcl/Tk wrapper)
net-misc/curl:http2 - Enable HTTP/2 support (via net-libs/nghttp2)
media-video/ffmpeg:x265 - Enable HEVC encoding with media-libs/x265
`
	ud, err := ParseUseLocalDesc(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseUseLocalDesc() error = %v", err)
	}

	if ud.Flags["sys-apps/systemd"]["pkcs11"] != "Enable PKCS#11 smartcard support" {
		t.Errorf("Mismatch in complex flag parsing")
	}

	if ud.Flags["dev-lang/python"]["tk"] != "Build tkinter (Tcl/Tk wrapper)" {
		t.Errorf("Mismatch in complex flag parsing")
	}

	if ud.Flags["net-misc/curl"]["http2"] != "Enable HTTP/2 support (via net-libs/nghttp2)" {
		t.Errorf("Mismatch in complex flag parsing")
	}
}

func TestParseUseLocalDesc_Unsupported(t *testing.T) {
	t.Skip("Unsupported format: multi-line descriptions or other extreme oddities are not supported by the simple parser yet.")
	// Placeholder for if we ever find things that break the "pkg:flag - desc" strictly one-line assumption
}
