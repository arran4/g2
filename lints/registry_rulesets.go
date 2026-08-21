package lints

type RuleSet struct {
	ID          string
	Description string
	Rules       map[string]RuleSetEntry
}

type RuleSetEntry struct {
	Enabled  bool
	Severity Severity
}

var builtInRuleSets = map[string]*RuleSet{
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

func RegisterRuleSet(rs *RuleSet) {
	builtInRuleSets[rs.ID] = rs
}

func GetRuleSet(id string) (*RuleSet, bool) {
	rs, ok := builtInRuleSets[id]
	return rs, ok
}
