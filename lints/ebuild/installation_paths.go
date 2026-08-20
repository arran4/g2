package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleInstallationPaths = lints.RuleMetadata{
	ID:          "InstallationPaths",
	Title:       "Installation paths",
	Description: "Gentoo packages may only install into permitted top-level directories and subdirectories.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0201",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "filesystem", "PG0201"},
}

func init() {
	lints.RegisterRuleMetadata(ruleInstallationPaths)
	lints.RegisterLintRule(&InstallationPathsLintRule{})
}

type InstallationPathsLintRule struct{}

func isAllowedTopLevel(path string) bool {
	allowed := []string{
		"/bin", "/boot", "/dev", "/etc", "/opt", "/sbin", "/srv", "/usr", "/var",
		"/gnu", "/nix", // exceptions
	}
	for _, a := range allowed {
		if path == a || strings.HasPrefix(path, a+"/") {
			return true
		}
	}

	if strings.HasPrefix(path, "/lib") {
		return true
	}

	return false
}

func isAllowedUsrSubdir(path string) bool {
	if path == "/usr" {
		return true
	}

	allowedPrefixes := []string{
		"/usr/bin", "/usr/include", "/usr/libexec", "/usr/sbin", "/usr/share", "/usr/src",
	}

	for _, a := range allowedPrefixes {
		if path == a || strings.HasPrefix(path, a+"/") {
			return true
		}
	}

	if strings.HasPrefix(path, "/usr/lib") {
		return true
	}

	// Toolchain triplets like /usr/x86_64-pc-linux-gnu are allowed, but harder to validate statically without CHOST.
	// We'll allow anything under /usr for now except specific banned ones if we want to be strict,
	// or we can strictly enforce and allow exceptions.
	// A common heuristic is to allow if it doesn't violate the "no subdirectories in /usr/bin" rule.

	return true
}

func isBannedSubdir(path string) bool {
	if path != "/bin" && strings.HasPrefix(path, "/bin/") {
		return true // No subdirectories in /bin
	}
	if path != "/sbin" && strings.HasPrefix(path, "/sbin/") {
		return true // No subdirectories in /sbin
	}
	if path != "/usr/bin" && strings.HasPrefix(path, "/usr/bin/") {
		return true // No subdirectories in /usr/bin
	}
	if path != "/usr/sbin" && strings.HasPrefix(path, "/usr/sbin/") {
		return true // No subdirectories in /usr/sbin
	}
	return false
}


func isInstallationPathsViolation(word *syntax.Word) (bool, string) {
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
				}
			}
		}
	}

	s := fullString.String()
	sClean := strings.Trim(s, "\"'")
	sClean = filepath.Clean(sClean)

	if !strings.HasPrefix(sClean, "/") {
		return false, ""
	}

	if isBannedSubdir(sClean) {
		return true, sClean
	}

	if !isAllowedTopLevel(sClean) {
		return true, sClean
	}

	if strings.HasPrefix(sClean, "/usr/") && !isAllowedUsrSubdir(sClean) {
		return true, sClean
	}

	return false, ""
}


func (l *InstallationPathsLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil)
}

func (l *InstallationPathsLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0201"]; ok {
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

				if cmdName == "into" || cmdName == "insinto" || cmdName == "exeinto" || cmdName == "docinto" || cmdName == "dodir" || cmdName == "keepdir" || cmdName == "diropts" || cmdName == "exeopts" || cmdName == "insopts" || cmdName == "libopts" {
					for i := 1; i < len(cmd.Args); i++ {
						isViolation, val := isInstallationPathsViolation(cmd.Args[i])
						if isViolation {
							res := lints.LintResult{
								RuleMetadata: ruleInstallationPaths,
								Message:      fmt.Sprintf("[%s] Ebuild %s attempts to install into invalid path '%s' using '%s'.", severity, version.Ebuild.Path, val, cmdName),
								Package:      pkgData.Category + "/" + pkgData.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				}
			}
			return true
		})
	}

	return results
}
