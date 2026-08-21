package lints_test

import (
	"testing"

	"github.com/arran4/g2/lints"
)

func TestRuleSetRetrieval(t *testing.T) {
	rs, ok := lints.GetRuleSet("default")
	if !ok {
		t.Fatalf("expected 'default' ruleset to exist")
	}
	if rs.ID != "default" {
		t.Errorf("expected ID 'default', got %s", rs.ID)
	}

	_, ok = lints.GetRuleSet("unknown-ruleset")
	if ok {
		t.Errorf("expected 'unknown-ruleset' to not exist")
	}
}

func TestRuleSetSeverityOverrides(t *testing.T) {
	rsGentoo, _ := lints.GetRuleSet("gentoo-main")

	entry, ok := rsGentoo.Rules["GentooCopyrightHeader"]
	if !ok || !entry.Enabled {
		t.Fatalf("expected GentooCopyrightHeader to be enabled in gentoo-main")
	}
	if entry.Severity != lints.SeverityWarning {
		t.Errorf("expected GentooCopyrightHeader severity to be %s, got %s", lints.SeverityWarning, entry.Severity)
	}
}
