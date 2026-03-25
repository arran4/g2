package parser

import (
	"context"
	"strings"
	"testing"
)

func TestParseEbuild(t *testing.T) {
	ebuildStr := `
EAPI=8
DESCRIPTION="Test ebuild"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/${P}.tar.gz"

myarray=(
    one
    # comment
    two
)

src_test() {
	epytest \
		test/test_encryption.py::test_symmetric_encrypt[clean-encrypt_exists-bad_phrase] # hangs in sandbox
}

src_install() {
    emake DESTDIR="${D}" install
}
`

	parser := NewEbuildParser(context.Background(), strings.NewReader(ebuildStr))
	ebuild, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if ebuild.Variables["EAPI"] != "8" {
		t.Errorf("Expected EAPI=8, got %q", ebuild.Variables["EAPI"])
	}
	if ebuild.Variables["DESCRIPTION"] != "Test ebuild" {
		t.Errorf("Expected DESCRIPTION='Test ebuild', got %q", ebuild.Variables["DESCRIPTION"])
	}

	if !strings.Contains(ebuild.Variables["myarray"], "one") || !strings.Contains(ebuild.Variables["myarray"], "two") {
		t.Errorf("Expected myarray to contain 'one' and 'two', got %q", ebuild.Variables["myarray"])
	}
}

func TestParseEdgeCases(t *testing.T) {
	ebuildStr := `
    my_var = "spaced assignment"
    array_with_comment=(
       "string" # inline comment
       bareword
    )
    unquoted_brackets[args]=value
    `

	parser := NewEbuildParser(context.Background(), strings.NewReader(ebuildStr))
	ebuild, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if ebuild.Variables["my_var"] != "spaced assignment" {
		t.Errorf("Expected 'spaced assignment', got %q", ebuild.Variables["my_var"])
	}
    if !strings.Contains(ebuild.Variables["array_with_comment"], "bareword") {
        t.Errorf("Expected bareword to be in array: %v", ebuild.Variables["array_with_comment"])
    }
}
