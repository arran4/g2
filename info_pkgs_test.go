package g2

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseInfoPkgsReader(t *testing.T) {
	input := `
# Copyright 2004-2025 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2
##
## These ATOMS are printed with a standard 'emerge info' in
## portage as of 2.0.51-r5. Do not overcrowd the output please.
##
app-shells/bash:0
dev-build/autoconf

dev-build/automake
`

	expected := []InfoPkg{
		{PackageAtom: "app-shells/bash:0"},
		{PackageAtom: "dev-build/autoconf"},
		{PackageAtom: "dev-build/automake"},
	}

	results, err := parseInfoPkgsReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(results))
	}

	for i, r := range results {
		if r.PackageAtom != expected[i].PackageAtom {
			t.Errorf("result %d: expected %q, got %q", i, expected[i].PackageAtom, r.PackageAtom)
		}
	}
}

func TestSerializeInfoPkgs(t *testing.T) {
	pkgs := []InfoPkg{
		{PackageAtom: "app-shells/bash:0"},
		{PackageAtom: "dev-build/autoconf"},
		{PackageAtom: "dev-build/automake"},
	}

	var buf bytes.Buffer
	if err := SerializeInfoPkgs(&buf, pkgs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "app-shells/bash:0\ndev-build/autoconf\ndev-build/automake"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestParseInfoPkgsFS(t *testing.T) {
	fsys := fstest.MapFS{
		"info_pkgs": &fstest.MapFile{
			Data: []byte("app-shells/bash:0\ndev-build/autoconf\n"),
		},
	}

	tests := []struct {
		name     string
		path     string
		expected []InfoPkg
		wantErr  bool
	}{
		{
			name: "existing file",
			path: "info_pkgs",
			expected: []InfoPkg{
				{PackageAtom: "app-shells/bash:0"},
				{PackageAtom: "dev-build/autoconf"},
			},
			wantErr: false,
		},
		{
			name:     "missing file",
			path:     "non_existent",
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := ParseInfoPkgsFS(fsys, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInfoPkgsFS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(results, tt.expected) {
				t.Errorf("ParseInfoPkgsFS() = %v, want %v", results, tt.expected)
			}
		})
	}
}

func TestParseInfoPkgs(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		fsys := fstest.MapFS{
			"info_pkgs": &fstest.MapFile{
				Data: []byte("app-shells/bash:0\ndev-build/autoconf\n"),
			},
		}

		expected := []InfoPkg{
			{PackageAtom: "app-shells/bash:0"},
			{PackageAtom: "dev-build/autoconf"},
		}

		results, err := ParseInfoPkgsFS(fsys, "info_pkgs")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("ParseInfoPkgs() = %v, want %v", results, expected)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		fsys := fstest.MapFS{}

		results, err := ParseInfoPkgsFS(fsys, "non_existent")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if results != nil {
			t.Errorf("ParseInfoPkgs() = %v, want nil", results)
		}
	})
}
