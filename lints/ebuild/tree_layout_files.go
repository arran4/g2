package ebuild

import (
	"fmt"
	"regexp"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

var ruleTreeLayoutFiles = lints.RuleMetadata{
	ID:          "TreeLayoutFiles",
	Title:       "Tree Layout Files Naming",
	Description: "Checks if files in the package directory follow naming rules: no characters outside [A-Za-z0-9._+-] and not starting with a dot, a hyphen, or a plus sign.",
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"repo-layout"},
}

// TreeLayoutFilesLintRule checks if file names adhere to Gentoo tree rules.
type TreeLayoutFilesLintRule struct{}

func init() {
	lints.RegisterRuleMetadata(ruleTreeLayoutFiles)
	lints.RegisterLintRule(&TreeLayoutFilesLintRule{})
}

var invalidCharsRegex = regexp.MustCompile(`[^A-Za-z0-9._+-]`)
var invalidStartRegex = regexp.MustCompile(`^[.+-]`)

func (l *TreeLayoutFilesLintRule) Lint(repoDir string, pkg *g2.PackageData, ctx *lints.LintContext) []lints.LintResult {
	var results []lints.LintResult

	for _, file := range pkg.Files {
		name := file.Name

		if invalidStartRegex.MatchString(name) {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleTreeLayoutFiles,
				Message:      fmt.Sprintf("file '%s' starts with a dot, hyphen, or plus sign", name),
				Package:      pkg.Category + "/" + pkg.Name,
				File:         file.Path,
			})
		} else if invalidCharsRegex.MatchString(name) {
			results = append(results, lints.LintResult{
				RuleMetadata: ruleTreeLayoutFiles,
				Message:      fmt.Sprintf("file '%s' contains characters outside [A-Za-z0-9._+-]", name),
				Package:      pkg.Category + "/" + pkg.Name,
				File:         file.Path,
			})
		}
	}

	return results
}
