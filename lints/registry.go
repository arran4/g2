package lints

import (
	"path/filepath"

	"github.com/arran4/g2"
)

type Severity string

const (
	SeverityError   Severity = "Error"
	SeverityWarning Severity = "Warning"
	SeverityNotice  Severity = "Notice"
	SeverityInfo    Severity = "Info"
)

type Source string

const (
	SourceG2       Source = "g2"
	SourcePkgcheck Source = "pkgcheck"
)

type RuleMetadata struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	URL         string   `json:"url,omitempty"`
	Severity    Severity `json:"severity"`
	Source      Source   `json:"source"`
	Tags        []string `json:"tags,omitempty"`
}

type LintResult struct {
	RuleMetadata RuleMetadata `json:"rule"`
	Message      string       `json:"message"`
	Package      string       `json:"package,omitempty"`
	File         string       `json:"file,omitempty"`
	Line         int          `json:"line,omitempty"`
}

func (lr LintResult) String() string {
	return lr.Message
}

type LintRule interface {
	Lint(repoDir string, pkg *g2.PackageData) []LintResult
}

type QAAwareLintRule interface {
	LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []LintResult
}

var lintRules []LintRule

func RegisterLintRule(rule LintRule) {
	lintRules = append(lintRules, rule)
}

func PerformLintingResults(repoDir string, pkg *g2.PackageData) []LintResult {
	var results []LintResult

	// Try to load QA Policy
	qaPolicyPath := filepath.Join(repoDir, "metadata", "qa-policy.conf")
	qa, _ := g2.ParseQAPolicy(qaPolicyPath)

	for _, rule := range lintRules {
		if qaRule, ok := rule.(QAAwareLintRule); ok {
			results = append(results, qaRule.LintWithQA(repoDir, pkg, qa)...)
		} else {
			results = append(results, rule.Lint(repoDir, pkg)...)
		}
	}
	return results
}

func PerformLinting(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string
	for _, res := range PerformLintingResults(repoDir, pkg) {
		warnings = append(warnings, res.Message)
	}
	return warnings
}

type MetadataAwareLintRule interface {
	LintRule
	Metadata() RuleMetadata
}

var registeredMetadata []RuleMetadata

func RegisterRuleMetadata(meta RuleMetadata) {
	registeredMetadata = append(registeredMetadata, meta)
}

func GetAllRules() []RuleMetadata {
	return registeredMetadata
}
