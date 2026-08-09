package g2

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"io/fs"
)

func TestParseArchList(t *testing.T) {
	r := strings.NewReader("amd64\n# a comment\n\n  x86  \narm64")
	al, err := ParseArchList(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"amd64", "x86", "arm64"}
	if !reflect.DeepEqual(al.Arches, expected) {
		t.Errorf("expected %v, got %v", expected, al.Arches)
	}
}

type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("mock read error")
}

func TestParseArchesDesc(t *testing.T) {
	tests := []struct {
		name        string
		input       io.Reader
		wantHeaders []string
		wantArches  map[string]string
		wantErr     bool
	}{
		{
			name: "basic",
			input: strings.NewReader(`# Copyright 1999-2024 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

# This file contains descriptions of architectures.

amd64           stable
x86             stable
~mips           testing
`),
			wantHeaders: []string{
				"# Copyright 1999-2024 Gentoo Authors",
				"# Distributed under the terms of the GNU General Public License v2",
				"",
				"# This file contains descriptions of architectures.",
				"",
			},
			wantArches: map[string]string{
				"amd64": "stable",
				"x86":   "stable",
				"~mips": "testing",
			},
			wantErr: false,
		},
		{
			name: "no header",
			input: strings.NewReader(`amd64 stable
x86 stable`),
			wantHeaders: []string{},
			wantArches: map[string]string{
				"amd64": "stable",
				"x86":   "stable",
			},
			wantErr: false,
		},
		{
			name:        "empty",
			input:       strings.NewReader(``),
			wantHeaders: []string{},
			wantArches:  map[string]string{},
			wantErr:     false,
		},
		{
			name: "malformed arch line",
			input: strings.NewReader(`amd64
x86 stable
`),
			wantHeaders: []string{},
			wantArches: map[string]string{
				"x86": "stable",
			},
			wantErr: false,
		},
		{
			name: "trailing comments and blank lines",
			input: strings.NewReader(`# Header
amd64 stable

# This is a trailing comment
x86 stable
`),
			wantHeaders: []string{"# Header"},
			wantArches: map[string]string{
				"amd64": "stable",
				"x86":   "stable",
			},
			wantErr: false,
		},
		{
			name:    "read error",
			input:   &errorReader{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ad, err := ParseArchesDesc(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseArchesDesc() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				return
			}

			if tt.wantHeaders == nil {
				tt.wantHeaders = []string{}
			}
			if len(ad.HeaderLines) == 0 && len(tt.wantHeaders) == 0 {
				// both empty, DeepEqual handles this but sometimes []string{} and nil are different
			} else if !reflect.DeepEqual(ad.HeaderLines, tt.wantHeaders) {
				t.Errorf("HeaderLines = %v, want %v", ad.HeaderLines, tt.wantHeaders)
			}

			if tt.wantArches == nil {
				tt.wantArches = map[string]string{}
			}
			if len(ad.Arches) == 0 && len(tt.wantArches) == 0 {
				// both empty
			} else if !reflect.DeepEqual(ad.Arches, tt.wantArches) {
				t.Errorf("Arches = %v, want %v", ad.Arches, tt.wantArches)
			}
		})
	}
}

func TestParseArchListFile(t *testing.T) {
	// Test success using os.DevNull to avoid disk changes
	al, err := ParseArchListFile(os.DevNull)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(al.Arches) != 0 {
		t.Errorf("expected 0 arches, got %d", len(al.Arches))
	}

	// Test non-existent file
	_, err = ParseArchListFile("nonexistent.list")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
  }
}

type errorFS struct{}

func (e errorFS) Open(name string) (fs.File, error) {
	return nil, errors.New("mock read error")
}

func TestParseArchesDescFile(t *testing.T) {
	// Test Case 1: File exists
	validContent := `# Header
amd64 stable
x86 testing
`
	fsys := fstest.MapFS{
		"arches.desc": &fstest.MapFile{
			Data: []byte(validContent),
		},
	}

	ad, err := ParseArchesDescFS(fsys, "arches.desc")
	if err != nil {
		t.Fatalf("unexpected error for valid file: %v", err)
	}
	expectedHeaders := []string{"# Header"}
	expectedArches := map[string]string{
		"amd64": "stable",
		"x86":   "testing",
	}
	if !reflect.DeepEqual(ad.HeaderLines, expectedHeaders) {
		t.Errorf("expected headers %v, got %v", expectedHeaders, ad.HeaderLines)
	}
	if !reflect.DeepEqual(ad.Arches, expectedArches) {
		t.Errorf("expected arches %v, got %v", expectedArches, ad.Arches)
	}

	// Test Case 2: File does not exist
	adMissing, err := ParseArchesDescFS(fsys, "missing.desc")
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	expectedMissingHeaders := []string{
		"# Copyright 1999-2024 Gentoo Authors",
		"# Distributed under the terms of the GNU General Public License v2",
	}
	if !reflect.DeepEqual(adMissing.HeaderLines, expectedMissingHeaders) {
		t.Errorf("expected missing headers %v, got %v", expectedMissingHeaders, adMissing.HeaderLines)
	}
	if len(adMissing.Arches) != 0 {
		t.Errorf("expected empty arches, got %v", adMissing.Arches)
	}

	// Test Case 3: Error case
	_, err = ParseArchesDescFS(errorFS{}, "error.desc")
	if err == nil {
		t.Fatalf("expected error when passing invalid file path")
	}
}
