package g2

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseThirdPartyMirrorsFromReader(t *testing.T) {
	content := `
# A comment
apache          https://dlcdn.apache.org/ https://apache.mirror.iphh.net/
cpan            https://cpan.metacpan.org https://www.cpan.org

debian          https://deb.debian.org/debian/ http://ftp.au.debian.org/debian/
gentoo          https://distfiles.gentoo.org/distfiles
`

	expected := map[string][]string{
		"apache": {"https://dlcdn.apache.org/", "https://apache.mirror.iphh.net/"},
		"cpan":   {"https://cpan.metacpan.org", "https://www.cpan.org"},
		"debian": {"https://deb.debian.org/debian/", "http://ftp.au.debian.org/debian/"},
		"gentoo": {"https://distfiles.gentoo.org/distfiles"},
	}

	mirrors, err := ParseThirdPartyMirrorsFromReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(mirrors, expected) {
		t.Errorf("expected %v, got %v", expected, mirrors)
	}
}
