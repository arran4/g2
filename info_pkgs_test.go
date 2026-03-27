package g2

import (
	"bytes"
	"strings"
	"testing"
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
