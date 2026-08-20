package metadata

import (
	"fmt"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleRemoteIDType = lints.RuleMetadata{
	ID:          "RemoteIDType",
	Title:       "Valid Remote ID Type",
	Description: "Validates that the type attribute for <remote-id> elements matches the allowed enumeration in the DTD.",
	URLs:        []string{"https://devmanual.gentoo.org/ebuild-writing/misc-files/metadata.xml/#upstream"},
	Severity:    lints.SeverityError,
	Source:      lints.SourceG2,
	Tags:        []string{"metadata.xml", "site-quality"},
}

func init() {
	lints.RegisterRuleMetadata(ruleRemoteIDType)
	lints.RegisterLintRule(&RemoteIDTypeLintRule{})
}

type RemoteIDTypeLintRule struct{}

var validRemoteIDTypes = map[string]bool{
	"bitbucket": true, "codeberg": true, "cpan": true, "cpan-module": true,
	"cpe": true, "cran": true, "ctan": true, "freedesktop-gitlab": true,
	"gentoo": true, "github": true, "gitlab": true, "gnome-gitlab": true,
	"google-code": true, "hackage": true, "heptapod": true, "kde-invent": true,
	"launchpad": true, "osdn": true, "pear": true, "pecl": true,
	"pypi": true, "rubygems": true, "savannah": true, "savannah-nongnu": true,
	"sourceforge": true, "sourcehut": true, "vim": true,
}

func (r *RemoteIDTypeLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *RemoteIDTypeLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityError

	if pkg.Metadata != nil && pkg.Metadata.Upstream != nil {
		for _, rid := range pkg.Metadata.Upstream.RemoteID {
			if !validRemoteIDTypes[rid.Type] {
				res := lints.LintResult{
					RuleMetadata: ruleRemoteIDType,
					Message:      fmt.Sprintf("[%s] Invalid remote-id type '%s'", cases.Title(language.Und, cases.NoLower).String(string(severity)), rid.Type),
					Package:      pkg.Category + "/" + pkg.Name,
				}
				res.RuleMetadata.Severity = severity
				results = append(results, res)
			}
		}
	}

	return results
}
