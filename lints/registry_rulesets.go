package lints

import (
	"sync"
)

// RuleSet defines a named baseline collection of lint rules and their effective severities.
type RuleSet struct {
	ID          string
	Description string
	Rules       map[string]RuleSetEntry
}

// RuleSetEntry defines the enablement and effective severity of a rule within a RuleSet.
type RuleSetEntry struct {
	Enabled  bool
	Severity Severity
}

var (
	ruleSetsMu      sync.RWMutex
	builtInRuleSets = map[string]*RuleSet{
		"default": {
			ID:          "default",
			Description: "Repository-neutral behaviour.",
			Rules: map[string]RuleSetEntry{
				"CopyrightHeader": {Enabled: true, Severity: SeverityNotice},
			},
		},
		"gentoo-main": {
			ID:          "gentoo-main",
			Description: "Rules specifically appropriate to the Gentoo main repository.",
			Rules: map[string]RuleSetEntry{
				"CopyrightHeader":       {Enabled: true, Severity: SeverityNotice},
				"GentooCopyrightHeader": {Enabled: true, Severity: SeverityWarning},
			},
		},
		"guru": {
			ID:          "guru",
			Description: "GURU-specific policy.",
			Rules: map[string]RuleSetEntry{
				"CopyrightHeader": {Enabled: true, Severity: SeverityNotice},
			},
		},
	}
)

// RegisterRuleSet registers or updates a RuleSet in the central registry.
func RegisterRuleSet(rs *RuleSet) {
	if rs == nil || rs.ID == "" {
		return
	}
	ruleSetsMu.Lock()
	defer ruleSetsMu.Unlock()
	builtInRuleSets[rs.ID] = rs
}

// GetRuleSet retrieves a RuleSet by its ID.
func GetRuleSet(id string) (*RuleSet, bool) {
	ruleSetsMu.RLock()
	defer ruleSetsMu.RUnlock()
	rs, ok := builtInRuleSets[id]
	return rs, ok
}
