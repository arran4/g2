package lints

import (
	"path/filepath"

	"github.com/arran4/g2"
)

type LintRule interface {
	Lint(repoDir string, pkg *g2.PackageData) []string
}

type QAAwareLintRule interface {
	LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string
}

var lintRules []LintRule

func RegisterLintRule(rule LintRule) {
	lintRules = append(lintRules, rule)
}

func PerformLinting(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string

	// Try to load QA Policy
	qaPolicyPath := filepath.Join(repoDir, "metadata", "qa-policy.conf")
	qa, _ := g2.ParseQAPolicy(qaPolicyPath)

	for _, rule := range lintRules {
		if qaRule, ok := rule.(QAAwareLintRule); ok {
			warnings = append(warnings, qaRule.LintWithQA(repoDir, pkg, qa)...)
		} else {
			warnings = append(warnings, rule.Lint(repoDir, pkg)...)
		}
	}
	return warnings
}
