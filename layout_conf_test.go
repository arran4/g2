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
