package g2

import (
	"strings"
	"testing"
)

func TestParseQAPolicyFromReader(t *testing.T) {
	content := `[policy]
# =-dependencies with no revision
PG0002 = warning
# Revision bumps on runtime dependency changes
PG0003 = warning
# Slot and subslot dependencies: on (sub-)slotted packages
PG0011 = warning`

	qa, err := ParseQAPolicyFromReader(strings.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(qa.Policies) != 3 {
		t.Errorf("expected 3 policies, got %d", len(qa.Policies))
	}

	if qa.Policies["PG0002"] != "warning" {
		t.Errorf("expected warning for PG0002, got %s", qa.Policies["PG0002"])
	}
}
