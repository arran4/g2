package g2

import (
	"strings"
	"testing"
)

func TestParseLayoutConfFromReader(t *testing.T) {
	content := `masters = gentoo

# This is a comment
sign-commits = true


# Another comment
# Multiple lines
update-changelog = false`

	lc, err := ParseLayoutConfFromReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lc.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(lc.Entries))
	}

	// First entry
	if lc.Entries[0].Key != "masters" {
		t.Errorf("expected key 'masters', got '%s'", lc.Entries[0].Key)
	}
	if lc.Entries[0].Value != "gentoo" {
		t.Errorf("expected value 'gentoo', got '%s'", lc.Entries[0].Value)
	}
	if len(lc.Entries[0].Comments) != 0 {
		t.Errorf("expected 0 comments for masters, got %d", len(lc.Entries[0].Comments))
	}

	// Second entry
	if lc.Entries[1].Key != "sign-commits" {
		t.Errorf("expected key 'sign-commits', got '%s'", lc.Entries[1].Key)
	}
	if lc.Entries[1].Value != "true" {
		t.Errorf("expected value 'true', got '%s'", lc.Entries[1].Value)
	}
	if len(lc.Entries[1].Comments) != 2 {
		t.Errorf("expected 2 comments for sign-commits, got %d", len(lc.Entries[1].Comments))
	} else {
		if lc.Entries[1].Comments[0] != "" {
			t.Errorf("expected empty string as first comment, got '%s'", lc.Entries[1].Comments[0])
		}
		if lc.Entries[1].Comments[1] != "# This is a comment" {
			t.Errorf("expected '# This is a comment', got '%s'", lc.Entries[1].Comments[1])
		}
	}

	// Third entry
	if lc.Entries[2].Key != "update-changelog" {
		t.Errorf("expected key 'update-changelog', got '%s'", lc.Entries[2].Key)
	}
	if lc.Entries[2].Value != "false" {
		t.Errorf("expected value 'false', got '%s'", lc.Entries[2].Value)
	}
	if len(lc.Entries[2].Comments) != 4 {
		t.Errorf("expected 4 comments for update-changelog, got %d", len(lc.Entries[2].Comments))
	} else {
		if lc.Entries[2].Comments[0] != "" {
			t.Errorf("expected empty comment, got '%s'", lc.Entries[2].Comments[0])
		}
		if lc.Entries[2].Comments[1] != "" {
			t.Errorf("expected empty comment, got '%s'", lc.Entries[2].Comments[1])
		}
		if lc.Entries[2].Comments[2] != "# Another comment" {
			t.Errorf("expected '# Another comment', got '%s'", lc.Entries[2].Comments[2])
		}
		if lc.Entries[2].Comments[3] != "# Multiple lines" {
			t.Errorf("expected '# Multiple lines', got '%s'", lc.Entries[2].Comments[3])
		}
	}
}

func TestHasKey(t *testing.T) {
	content := `masters = gentoo
cache-formats =
`
	lc, err := ParseLayoutConfFromReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !lc.HasKey("masters") {
		t.Errorf("expected true for masters")
	}

	if !lc.HasKey("cache-formats") {
		t.Errorf("expected true for cache-formats")
	}

	if lc.HasKey("non-existent") {
		t.Errorf("expected false for non-existent")
	}
}

func TestLayoutConfTypedGetters(t *testing.T) {
	content := `masters = gentoo other
manifest-hashes = BLAKE2B SHA512
manifest-required-hashes = BLAKE2B
use-manifests = strict
update-changelog = true
cache-formats = md5-dict pms
eapis-deprecated = 0 1 2 3 4
eapis-banned = 5
eapis-testing = 6
profile-eapis-deprecated = 1 2
profile-eapis-banned = 0
repo-name = myrepo
aliases = alias1 alias2
thin-manifests = true
sign-commits = false
sign-manifests = true
properties-allowed = live
restrict-allowed = test
profile-formats = pms portage-2
`
	lc, err := ParseLayoutConfFromReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if masters := lc.Masters(); len(masters) != 2 || masters[0] != "gentoo" || masters[1] != "other" {
		t.Errorf("Masters() = %v", masters)
	}
	if hashes := lc.ManifestHashes(); len(hashes) != 2 || hashes[0] != "BLAKE2B" || hashes[1] != "SHA512" {
		t.Errorf("ManifestHashes() = %v", hashes)
	}
	if hashes := lc.ManifestRequiredHashes(); len(hashes) != 1 || hashes[0] != "BLAKE2B" {
		t.Errorf("ManifestRequiredHashes() = %v", hashes)
	}
	if um := lc.UseManifests(); um != "strict" {
		t.Errorf("UseManifests() = %q", um)
	}
	if uc := lc.UpdateChangelog(); uc != true {
		t.Errorf("UpdateChangelog() = %v", uc)
	}
	if formats := lc.CacheFormats(); len(formats) != 2 || formats[0] != "md5-dict" || formats[1] != "pms" {
		t.Errorf("CacheFormats() = %v", formats)
	}
	if eapis := lc.EapisDeprecated(); len(eapis) != 5 || eapis[0] != "0" {
		t.Errorf("EapisDeprecated() = %v", eapis)
	}
	if eapis := lc.EapisBanned(); len(eapis) != 1 || eapis[0] != "5" {
		t.Errorf("EapisBanned() = %v", eapis)
	}
	if eapis := lc.EapisTesting(); len(eapis) != 1 || eapis[0] != "6" {
		t.Errorf("EapisTesting() = %v", eapis)
	}
	if eapis := lc.ProfileEapisDeprecated(); len(eapis) != 2 || eapis[0] != "1" {
		t.Errorf("ProfileEapisDeprecated() = %v", eapis)
	}
	if eapis := lc.ProfileEapisBanned(); len(eapis) != 1 || eapis[0] != "0" {
		t.Errorf("ProfileEapisBanned() = %v", eapis)
	}
	if repoName := lc.RepoName(); repoName != "myrepo" {
		t.Errorf("RepoName() = %q", repoName)
	}
	if aliases := lc.Aliases(); len(aliases) != 2 || aliases[0] != "alias1" || aliases[1] != "alias2" {
		t.Errorf("Aliases() = %v", aliases)
	}
	if tm := lc.ThinManifests(); tm != true {
		t.Errorf("ThinManifests() = %v", tm)
	}
	if sc := lc.SignCommits(); sc != false {
		t.Errorf("SignCommits() = %v", sc)
	}
	if sm := lc.SignManifests(); sm != true {
		t.Errorf("SignManifests() = %v", sm)
	}
	if props := lc.PropertiesAllowed(); len(props) != 1 || props[0] != "live" {
		t.Errorf("PropertiesAllowed() = %v", props)
	}
	if restr := lc.RestrictAllowed(); len(restr) != 1 || restr[0] != "test" {
		t.Errorf("RestrictAllowed() = %v", restr)
	}
	if formats := lc.ProfileFormats(); len(formats) != 2 || formats[0] != "pms" || formats[1] != "portage-2" {
		t.Errorf("ProfileFormats() = %v", formats)
	}
}
