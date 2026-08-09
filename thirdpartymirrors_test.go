package g2

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
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

func TestParseThirdPartyMirrorsFS(t *testing.T) {
	content := `
apache          https://dlcdn.apache.org/
`
	sysFS := fstest.MapFS{
		"profiles/thirdpartymirrors": &fstest.MapFile{
			Data: []byte(content),
		},
	}

	expected := map[string][]string{
		"apache": {"https://dlcdn.apache.org/"},
	}

	mirrors, err := ParseThirdPartyMirrorsFS(sysFS, "profiles/thirdpartymirrors")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(mirrors, expected) {
		t.Errorf("expected %v, got %v", expected, mirrors)
	}

	// Test missing file
	_, err = ParseThirdPartyMirrorsFS(sysFS, "missing/file")
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestParseThirdPartyMirrors(t *testing.T) {
	content := `
apache          https://dlcdn.apache.org/
`
	dir := t.TempDir()
	path := filepath.Join(dir, "thirdpartymirrors")

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	expected := map[string][]string{
		"apache": {"https://dlcdn.apache.org/"},
	}

	mirrors, err := ParseThirdPartyMirrors(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(mirrors, expected) {
		t.Errorf("expected %v, got %v", expected, mirrors)
	}

	// Test missing file
	_, err = ParseThirdPartyMirrors(filepath.Join(dir, "missing"))
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}
