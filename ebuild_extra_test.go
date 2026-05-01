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

