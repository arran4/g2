package g2

import (
	"strings"
	"testing"
)

func TestParsePackageUnmasked(t *testing.T) {
	content := `# comment
# comment 2
# John Doe <john@doe.com> (2024-01-01)
# Unmasking package-1.2.3 for testing
cat/package-1.2.3

# Jane Doe <jane@doe.com> (2024-01-02)
# Unmasking package-4.5.6 for testing
cat/package-4.5.6
`

	unmask, err := parsePackageUnmaskedReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(unmask) != 2 {
		t.Fatalf("expected 2 unmask entries, got %d", len(unmask))
	}
}
