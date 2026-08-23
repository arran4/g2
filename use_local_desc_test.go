package g2

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestParseUseLocalDescFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFileName := filepath.Join(tmpDir, "use.local.desc")

	content := `# Copyright 1999-2024 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2
app-admin/conky:X - Enable X11 support
`
	if err := os.WriteFile(tmpFileName, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	// Test successful read
	ud, err := ParseUseLocalDescFile(tmpFileName)
	if err != nil {
		t.Fatalf("ParseUseLocalDescFile() error = %v", err)
	}

	if ud.Flags["app-admin/conky"]["X"] != "Enable X11 support" {
		t.Errorf("Mismatch in flag parsing from file")
	}

	// Test file not found
	nonExistentPath := filepath.Join(tmpDir, "non_existent_file.desc")
	udNotFound, err := ParseUseLocalDescFile(nonExistentPath)
	if err != nil {
		t.Fatalf("ParseUseLocalDescFile() expected nil error for not exist, got %v", err)
	}

	if len(udNotFound.Flags) != 0 {
		t.Errorf("Expected 0 flags for non-existent file, got %d", len(udNotFound.Flags))
	}
	if len(udNotFound.HeaderLines) != 2 {
		t.Errorf("Expected 2 header lines for non-existent file, got %d", len(udNotFound.HeaderLines))
	}
	if udNotFound.HeaderLines[0] != "# Copyright 1999-2024 Gentoo Authors" {
		t.Errorf("Expected first header line to match default, got %q", udNotFound.HeaderLines[0])
	}
	if udNotFound.HeaderLines[1] != "# Distributed under the terms of the GNU General Public License v2" {
		t.Errorf("Expected second header line to match default, got %q", udNotFound.HeaderLines[1])
	}
}

func TestWriteFileUseLocalDesc(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_write_local_desc_*.desc")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFileName := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpFileName) }()

	ud := &UseLocalDesc{
		Flags: map[string]map[string]string{
			"app-misc/foo": {
				"bar": "Enable bar",
			},
		},
		HeaderLines: []string{
			"# Header line",
		},
	}

	if err := ud.WriteFile(tmpFileName); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	content, err := os.ReadFile(tmpFileName)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "# Header line\napp-misc/foo:bar - Enable bar\n"
	if string(content) != expected {
		t.Errorf("File content = %q, want %q", string(content), expected)
	}

	// test write file error
	err = ud.WriteFile("/invalid/path/that/does/not/exist/test.desc")
	if err == nil {
		t.Errorf("Expected WriteFile() to return an error, got nil")
	}
}

type errReaderUseLocalDesc struct{}

func (errReaderUseLocalDesc) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("mock reader error")
}

// To test scanner errors, we need a reader that fails *after* successful reads, or just an immediate failure
// during scan. bufio.Scanner fails if Read returns an error other than io.EOF.
func TestParseUseLocalDesc_Error(t *testing.T) {
	_, err := ParseUseLocalDesc(errReaderUseLocalDesc{})
	if err == nil {
		t.Errorf("Expected ParseUseLocalDesc() to return an error with a faulty reader")
	}
}

type errWriterUseLocalDesc struct {
	failOnWrite int
	writeCount  int
}

func (w *errWriterUseLocalDesc) Write(p []byte) (n int, err error) {
	w.writeCount++
	if w.writeCount == w.failOnWrite {
		return 0, fmt.Errorf("mock writer error on write %d", w.failOnWrite)
	}
	return len(p), nil
}

func TestWriteUseLocalDesc_Error(t *testing.T) {
	ud := &UseLocalDesc{
		Flags: map[string]map[string]string{
			"app-misc/foo": {
				"bar": "Enable bar",
			},
		},
		HeaderLines: []string{
			"# Header line",
		},
	}

	// Test failure on header write
	errW1 := &errWriterUseLocalDesc{failOnWrite: 1}
	if err := ud.Write(errW1); err == nil {
		t.Errorf("Expected Write() to return an error on header write, got nil")
	}

	// Test failure on package/flag write
	errW2 := &errWriterUseLocalDesc{failOnWrite: 2}
	if err := ud.Write(errW2); err == nil {
		t.Errorf("Expected Write() to return an error on flag write, got nil")
	}
}
