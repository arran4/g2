package g2

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseLicenseMapping(t *testing.T) {
	content := `
# Copyright 2022-2025 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2
# Mapping of SPDX-2.0 licenses to Gentoo repository licenses.

[spdx-to-ebuild]
0BSD = 0BSD
AFL-2.1 = AFL-2.1
AGPL-3.0 = AGPL-3
AGPL-3.0+ = AGPL-3+
AGPL-3.0-only = AGPL-3
Apache-2.0 WITH LLVM-exception = Apache-2.0-with-LLVM-exceptions
`

	expected := map[string][]string{
		"0BSD":                            {"0BSD"},
		"AFL-2.1":                         {"AFL-2.1"},
		"AGPL-3":                          {"AGPL-3.0", "AGPL-3.0-only"},
		"AGPL-3+":                         {"AGPL-3.0+"},
		"Apache-2.0-with-LLVM-exceptions": {"Apache-2.0 WITH LLVM-exception"},
	}

	result, err := ParseLicenseMapping(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
