package ebuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"mvdan.cc/sh/v3/syntax"
)

var ruleStaticLibraries = lints.RuleMetadata{
	ID:          "StaticLibraries",
	Title:       "Static libraries and libtool files",
	Description: "Static libraries and libtool files (.la) must be installed into /usr hierarchy and never to root filesystem (/lib*).",
	URL:         "https://projects.gentoo.org/qa/policy-guide/filesystem.html#pg0204",
	Severity:    lints.SeverityError,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "filesystem", "PG0204"},
}

func init() {
	lints.RegisterRuleMetadata(ruleStaticLibraries)
	lints.RegisterLintRule(&StaticLibrariesLintRule{})
}

type StaticLibrariesLintRule struct{}

func isLibPath(word *syntax.Word) (bool, string) {
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

	if strings.HasPrefix(sClean, "/lib") {
		return true, sClean
	}
	return false, ""
}

func isStaticLib(word *syntax.Word) bool {
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
				}
			}
		}
	}

	s := fullString.String()
	sClean := strings.Trim(s, "\"'")

	if strings.HasSuffix(sClean, ".a") || strings.HasSuffix(sClean, ".la") {
		return true
	}
	return false
}


func (l *StaticLibrariesLintRule) Lint(repoDir string, pkgData *g2.PackageData) []lints.LintResult {
	return l.LintWithQA(repoDir, pkgData, nil)
}

func (l *StaticLibrariesLintRule) LintWithQA(repoDir string, pkgData *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	severity := lints.SeverityError
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0204"]; ok {
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

		// Keep track of current install path
		currentInstallPath := ""

		syntax.Walk(f, func(node syntax.Node) bool {
			cmd, ok := node.(*syntax.CallExpr)
			if !ok {
				return true
			}

			if len(cmd.Args) > 0 {
				var cmdName string
				if len(cmd.Args[0].Parts) == 1 {
					if lit, ok := cmd.Args[0].Parts[0].(*syntax.Lit); ok {
						cmdName = lit.Value
					}
				}

				if cmdName == "into" || cmdName == "insinto" || cmdName == "exeinto" || cmdName == "docinto" || cmdName == "dodir" || cmdName == "keepdir" || cmdName == "diropts" || cmdName == "exeopts" || cmdName == "insopts" || cmdName == "libopts" {
					if len(cmd.Args) > 1 {
						_, val := isLibPath(cmd.Args[1])
						if val != "" {
							currentInstallPath = val
						} else {
							// If we change directory away from /lib*, clear state. (simplified)
							currentInstallPath = ""
						}
					}
				}

				if (cmdName == "doins" || cmdName == "dolib" || cmdName == "dolib.a" || cmdName == "dolib.so") {
					for i := 1; i < len(cmd.Args); i++ {
						if isStaticLib(cmd.Args[i]) {
							if currentInstallPath != "" { // Means we are in /lib*
								res := lints.LintResult{
									RuleMetadata: ruleStaticLibraries,
									Message:      fmt.Sprintf("[%s] Ebuild %s attempts to install static library or libtool file into '%s' using '%s', which must be in /usr hierarchy.", severity, version.Ebuild.Path, currentInstallPath, cmdName),
									Package:      pkgData.Category + "/" + pkgData.Name,
								}
								res.RuleMetadata.Severity = severity
								results = append(results, res)
							}
						}
					}
				}

				// Also catch direct installs to /lib if dolib path is specified or via another mechanism (simplified to into/insinto tracking + doins/dolib)
			}
			return true
		})
	}

	return results
}
