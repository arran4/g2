package g2

import (
	"testing"
)

func TestExtractPackageNameFromDep(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{">=dev-lang/python-3.10.4:0/3.10[sqlite,xml]", "dev-lang/python"},
		{"~dev-libs/libxml2-2.9.12-r5", "dev-libs/libxml2"},
		{"dev-lang/python", "dev-lang/python"},
		{"!!sys-fs/udev", "sys-fs/udev"},
		{"!<sys-apps/systemd-216", "sys-apps/systemd"},
		{"=media-libs/libpng-1.6.39-r1", "media-libs/libpng"},
		{"virtual/pkgconfig", "virtual/pkgconfig"},
		{"<x11-base/xorg-server-21.1", "x11-base/xorg-server"},
		{"app-misc/foo-1.0_alpha2-r3", "app-misc/foo"},
	}

	for _, tc := range tests {
		actual := ExtractPackageNameFromDep(tc.input)
		if actual != tc.expected {
			t.Errorf("ExtractPackageNameFromDep(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestParsePackageAtom(t *testing.T) {
	tests := []struct {
		input string
		op    string
		cat   string
		name  string
		ver   string
		slot  string
		use   string
	}{
		{">=dev-lang/python-3.10.4:0/3.10[sqlite,xml]", ">=", "dev-lang", "python", "3.10.4", "0/3.10", "sqlite,xml"},
		{"~dev-libs/libxml2-2.9.12-r5", "~", "dev-libs", "libxml2", "2.9.12-r5", "", ""},
		{"dev-lang/python", "", "dev-lang", "python", "", "", ""},
		{"!!sys-fs/udev", "!!", "sys-fs", "udev", "", "", ""},
		{"!<sys-apps/systemd-216", "!<", "sys-apps", "systemd", "216", "", ""},
		{"=media-libs/libpng-1.6.39-r1", "=", "media-libs", "libpng", "1.6.39-r1", "", ""},
	}

	for _, tc := range tests {
		atom := ParsePackageAtom(tc.input)
		if atom.Operator != tc.op {
			t.Errorf("input %q: expected op %q, got %q", tc.input, tc.op, atom.Operator)
		}
		if atom.Category != tc.cat {
			t.Errorf("input %q: expected cat %q, got %q", tc.input, tc.cat, atom.Category)
		}
		if atom.Name != tc.name {
			t.Errorf("input %q: expected name %q, got %q", tc.input, tc.name, atom.Name)
		}
		if atom.Version != tc.ver {
			t.Errorf("input %q: expected ver %q, got %q", tc.input, tc.ver, atom.Version)
		}
		if atom.Slot != tc.slot {
			t.Errorf("input %q: expected slot %q, got %q", tc.input, tc.slot, atom.Slot)
		}
		if atom.UseFlags != tc.use {
			t.Errorf("input %q: expected use %q, got %q", tc.input, tc.use, atom.UseFlags)
		}
	}
}
