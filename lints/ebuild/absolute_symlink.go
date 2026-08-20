package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleAbsoluteSymlink = lints.RuleMetadata{
	ID:          "AbsoluteSymlink",
	Title:       "Absolute symbolic link targets",
	Description: "Packages must not install symbolic links with absolute targets. Instead, relative paths must be used.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0206",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "filesystem", "PG0206"},
}

func init() {
	lints.RegisterRuleMetadata(ruleAbsoluteSymlink)
	lints.RegisterLintRule(&AbsoluteSymlinkLintRule{})
}

type AbsoluteSymlinkLintRule struct{}

func isAbsoluteSymlink(word *syntax.Word) (bool, string) {
	var fullString strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			fullString.WriteString(p.Value)
		case *syntax.SglQuoted:
			fullString.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, pp := range p.Parts {
				switch ppp := pp.(type) {
				case *syntax.Lit:
					fullString.WriteString(ppp.Value)
				case *syntax.ParamExp:
					// Variable expansion like ${EPREFIX}, but for absolute path we check leading '/' mostly
					// However, variables at the beginning might mean it's absolute, but usually they are checked against explicit strings.
				}
			}
		}
	}

	s := fullString.String()
	sClean := strings.Trim(s, "\"'")
	sClean = filepath.Clean(sClean)

	// Exclude /proc, /run
	if strings.HasPrefix(sClean, "/proc") || strings.HasPrefix(sClean, "/run") || strings.HasPrefix(sClean, "/dev") || strings.HasPrefix(sClean, "/sys") {
		return false, ""
	}

	if filepath.IsAbs(sClean) {
		return true, sClean
	}
	return false, ""
}

func (l *AbsoluteSymlinkLintRule) Lint(repoDir string, pkgData *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil, ctx)
}

func (l *AbsoluteSymlinkLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult

	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0206"]; ok {
			if val == "ignore" {
				return nil
			}
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

	for _, version := range pkgData.Versions {
		if version.Ebuild == nil || version.Ebuild.RawText == "" {
			continue
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(version.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		syntax.Walk(f, func(node syntax.Node) bool {
			cmd, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}

			if len(cmd.Args) > 1 {
				var cmdName string
				if len(cmd.Args[0].Parts) == 1 {
					if lit, ok := cmd.Args[0].Parts[0].(*syntax.Lit); ok {
						cmdName = lit.Value
					}
				}

				if cmdName == "dosym" {
					isViolation, val := isAbsoluteSymlink(cmd.Args[1]) // dosym target location
					if isViolation {
						res := lints.LintResult{
							RuleMetadata: ruleAbsoluteSymlink,
							Message:      fmt.Sprintf("[%s] Ebuild %s attempts to create an absolute symlink using '%s %s', which must be relative instead.", severity, version.Ebuild.Path, cmdName, val),
							Package:      pkgData.Category + "/" + pkgData.Name,
						}
						res.RuleMetadata.Severity = severity
						results = append(results, res)
					}
				}
			}
			return true
		})
	}

	return results
}
