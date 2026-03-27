package g2

import (
	"bytes"
	"reflect"
	"testing"
)

func TestParseInfoVarsReader(t *testing.T) {
	input := `# Copyright 2004-2023 Gentoo Authors
# Distributed under the terms of the GNU General Public License v2

##
## These variables are printed with a standard 'emerge info' in
## portage as of 2.0.51-r5. They are sorted regardless of order here.
##
ACCEPT_KEYWORDS
ACCEPT_LICENSE

ADDR2LINE
AR
`

	expected := []string{
		"ACCEPT_KEYWORDS",
		"ACCEPT_LICENSE",
		"ADDR2LINE",
		"AR",
	}

	r := bytes.NewReader([]byte(input))
	parsed, err := parseInfoVarsReader(r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(parsed, expected) {
		t.Errorf("expected %v, got %v", expected, parsed)
	}
}

func TestSerializeInfoVars(t *testing.T) {
	vars := []string{
		"ACCEPT_KEYWORDS",
		"ACCEPT_LICENSE",
		"ADDR2LINE",
		"AR",
	}

	expected := "ACCEPT_KEYWORDS\nACCEPT_LICENSE\nADDR2LINE\nAR\n"

	var buf bytes.Buffer
	err := SerializeInfoVars(&buf, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}
