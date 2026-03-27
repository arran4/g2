package g2

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseLicenseGroups(t *testing.T) {
	content := `
# Copyright 2022-2025 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2
# License groups for the repository.

GPL-COMPATIBLE GPL-2 GPL-3 LGPL-2 LGPL-2.1 LGPL-3
FREE MIT BSD Apache-2.0
@FREE FREE
`

	expected := map[string][]string{
		"GPL-COMPATIBLE": {"GPL-2", "GPL-3", "LGPL-2", "LGPL-2.1", "LGPL-3"},
		"FREE":           {"MIT", "BSD", "Apache-2.0"},
		"@FREE":          {"FREE"},
	}

	result, err := ParseLicenseGroups(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
