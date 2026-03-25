package lints

import (
	"github.com/arran4/g2"
)

type LintRule interface {
	Lint(repoDir string, pkg *g2.PackageData) []string
}

var lintRules []LintRule

func RegisterLintRule(rule LintRule) {
	lintRules = append(lintRules, rule)
}

func PerformLinting(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string
	for _, rule := range lintRules {
		warnings = append(warnings, rule.Lint(repoDir, pkg)...)
	}
	return warnings
}
