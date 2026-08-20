package eclass

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleEclassHeader = lints.RuleMetadata{
	ID:          "EclassHeader",
	Title:       "Eclass Header Documentation",
	Description: "Validates the presence of required documentation tags in eclasses.",
	URL:         "https://devmanual.gentoo.org/eclass-writing/index.html",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceG2,
	Tags:        []string{"eclass", "gentoo-policy", "PG0804"},
}

func init() {
	lints.RegisterRuleMetadata(ruleEclassHeader)
	lints.RegisterEclassLintRule(&EclassHeaderLintRule{})
}

type EclassHeaderLintRule struct{}

func (r *EclassHeaderLintRule) Lint(repoDir string, eclass *g2.Ebuild, ctx *lints.LintContext) []lints.LintResult {
	return r.LintWithQA(repoDir, eclass, nil, ctx)
}

func (r *EclassHeaderLintRule) LintWithQA(repoDir string, eclass *g2.Ebuild, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0804"]; ok {
			if val == "ignore" {
				return nil
			}
			if val == "notice" || val == "error" || val == "warning" {
				switch val {
				case "notice":
					severity = lints.SeverityNotice
				case "error":
					severity = lints.SeverityError
				case "warning":
					severity = lints.SeverityWarning
				}
			}
		}
	}

	if eclass == nil {
		return results
	}

	hasEclassTag := false
	hasMaintainerTag := false
	hasBlurbTag := false

	lines := strings.Split(eclass.RawText, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# @ECLASS:") {
			hasEclassTag = true
		} else if strings.HasPrefix(trimmed, "# @MAINTAINER:") {
			hasMaintainerTag = true
		} else if strings.HasPrefix(trimmed, "# @BLURB:") {
			hasBlurbTag = true
		}
	}

	eclassName := filepath.Base(eclass.Path)

	if !hasEclassTag {
		res := lints.LintResult{
			RuleMetadata: ruleEclassHeader,
			Message:      fmt.Sprintf("[%s] Eclass %s is missing the required @ECLASS: documentation tag.", severity, eclassName),
			Package:      eclassName,
			File:         eclass.Path,
		}
		res.RuleMetadata.Severity = severity
		results = append(results, res)
	}

	if !hasMaintainerTag {
		res := lints.LintResult{
			RuleMetadata: ruleEclassHeader,
			Message:      fmt.Sprintf("[%s] Eclass %s is missing the required @MAINTAINER: documentation tag.", severity, eclassName),
			Package:      eclassName,
			File:         eclass.Path,
		}
		res.RuleMetadata.Severity = severity
		results = append(results, res)
	}

	if !hasBlurbTag {
		res := lints.LintResult{
			RuleMetadata: ruleEclassHeader,
			Message:      fmt.Sprintf("[%s] Eclass %s is missing the required @BLURB: documentation tag.", severity, eclassName),
			Package:      eclassName,
			File:         eclass.Path,
		}
		res.RuleMetadata.Severity = severity
		results = append(results, res)
	}

	return results
}
