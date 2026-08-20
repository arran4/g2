package ebuild

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"mvdan.cc/sh/v3/syntax"
)

var ruleInstalledSmallFiles = lints.RuleMetadata{
	ID:          "InstalledSmallFiles",
	Title:       "Installation of Small Files",
	Description: "Ebuilds must not introduce USE flags to control installing files that are small in size, require no additional dependencies and not cause any negative consequences.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/installed-files.html#pg0301",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "installed-files"},
}

var ruleInstalledStaticLibs = lints.RuleMetadata{
	ID:          "InstalledStaticLibs",
	Title:       "Installation of Static Libraries",
	Description: "Packages must not install static libraries unless they are explicitly required. They should typically be behind a USE flag such as USE=static-libs.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/installed-files.html#pg0302",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "installed-files"},
}

var ruleInstalledManpages = lints.RuleMetadata{
	ID:          "InstalledManpages",
	Title:       "Installation of Manpages",
	Description: "Packages must not disable installing manpages via USE flags. They should be installed unconditionally.",
	URL:         "https://projects.gentoo.org/qa/policy-guide/installed-files.html#pg0305",
	Severity:    lints.SeverityWarning,
	Source:      lints.SourceQA,
	Tags:        []string{"ebuild", "gentoo-policy", "installed-files"},
}

func init() {
	lints.RegisterRuleMetadata(ruleInstalledSmallFiles)
	lints.RegisterRuleMetadata(ruleInstalledStaticLibs)
	lints.RegisterRuleMetadata(ruleInstalledManpages)
	lints.RegisterLintRule(&InstalledFilesLintRule{})
}

type InstalledFilesLintRule struct{}

func (r *InstalledFilesLintRule) Lint(repoDir string, pkg *g2.PackageData) []lints.LintResult {
	return r.LintWithQA(repoDir, pkg, nil)
}

func isUseFunc(cmdName string) bool {
	switch cmdName {
	case "use", "usev", "usex", "use_with", "use_enable", "useq":
		return true
	}
	return false
}

func hasUseCall(node syntax.Node) bool {
	found := false
	syntax.Walk(node, func(n syntax.Node) bool {
		if found {
			return false
		}
		if cmd, ok := n.(*syntax.CallExpr); ok && len(cmd.Args) > 0 {
			if len(cmd.Args[0].Parts) == 1 {
				if lit, ok := cmd.Args[0].Parts[0].(*syntax.Lit); ok {
					if isUseFunc(lit.Value) {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	return found
}

func (r *InstalledFilesLintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []lints.LintResult {
	var results []lints.LintResult

	smallFileFuncs := map[string]bool{
		"dobashcomp":          true,
		"newbashcomp":         true,
		"systemd_dounit":      true,
		"systemd_newunit":     true,
		"systemd_douserunit":  true,
		"systemd_newuserunit": true,
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild == nil {
			continue
		}

		parser := syntax.NewParser()
		f, err := parser.Parse(strings.NewReader(ver.Ebuild.RawText), "")
		if err != nil {
			continue
		}

		var walk func(node syntax.Node, inUseCond bool)
		walk = func(node syntax.Node, inUseCond bool) {
			if node == nil {
				return
			}

			switch n := node.(type) {
			case *syntax.CallExpr:
				if len(n.Args) > 0 {
					var cmdName string
					if len(n.Args[0].Parts) == 1 {
						if lit, ok := n.Args[0].Parts[0].(*syntax.Lit); ok {
							cmdName = lit.Value
						}
					}

					if inUseCond {
						if smallFileFuncs[cmdName] {
							if qa == nil || qa.Policies["PG0301"] != "ignore" {
								res := lints.LintResult{
									RuleMetadata: ruleInstalledSmallFiles,
									Message:      fmt.Sprintf("[%s] Ebuild %s calls '%s' conditionally based on a USE flag. Small files must be installed unconditionally (PG0301).", cases.Title(language.Und, cases.NoLower).String(string(ruleInstalledSmallFiles.Severity)), ver.Version, cmdName),
									Package:      pkg.Category + "/" + pkg.Name,
								}
								results = append(results, res)
							}
						}
						if cmdName == "doman" || cmdName == "newman" {
							if qa == nil || qa.Policies["PG0305"] != "ignore" {
								res := lints.LintResult{
									RuleMetadata: ruleInstalledManpages,
									Message:      fmt.Sprintf("[%s] Ebuild %s calls '%s' conditionally based on a USE flag. Manpages must be installed unconditionally (PG0305).", cases.Title(language.Und, cases.NoLower).String(string(ruleInstalledManpages.Severity)), ver.Version, cmdName),
									Package:      pkg.Category + "/" + pkg.Name,
								}
								results = append(results, res)
							}
						}
					} else {
						// Not in a USE conditional
						if cmdName == "dolib.a" {
							if qa == nil || qa.Policies["PG0302"] != "ignore" {
								res := lints.LintResult{
									RuleMetadata: ruleInstalledStaticLibs,
									Message:      fmt.Sprintf("[%s] Ebuild %s calls 'dolib.a' unconditionally. Static libraries should typically be explicitly required, usually behind a USE flag like 'static-libs' (PG0302).", cases.Title(language.Und, cases.NoLower).String(string(ruleInstalledStaticLibs.Severity)), ver.Version),
									Package:      pkg.Category + "/" + pkg.Name,
								}
								results = append(results, res)
							}
						}
					}
				}

				// Walk arguments
				for _, arg := range n.Args {
					walk(arg, inUseCond)
				}
				for _, assign := range n.Assigns {
					walk(assign, inUseCond)
				}
				return // we've handled the children manually

			case *syntax.IfClause:
				condHasUse := false
				for _, stmt := range n.Cond {
					if hasUseCall(stmt) {
						condHasUse = true
						break
					}
				}
				for _, stmt := range n.Cond {
					walk(stmt, inUseCond)
				}
				for _, stmt := range n.Then {
					walk(stmt, inUseCond || condHasUse)
				}
				if n.Else != nil {
					walk(n.Else, inUseCond || condHasUse)
				}
				return

			case *syntax.BinaryCmd:
				walk(n.X, inUseCond)
				xHasUse := hasUseCall(n.X)
				walk(n.Y, inUseCond || xHasUse)
				return

			case *syntax.FuncDecl:
				walk(n.Body, inUseCond)
				return
			}

			// Fallback generic walk via reflection
			val := reflect.ValueOf(node)
			if val.Kind() == reflect.Pointer && !val.IsNil() {
				val = val.Elem()
				for i := 0; i < val.NumField(); i++ {
					field := val.Field(i)
					if field.Kind() == reflect.Slice {
						for j := 0; j < field.Len(); j++ {
							if child, ok := field.Index(j).Interface().(syntax.Node); ok {
								walk(child, inUseCond)
							}
						}
					} else if child, ok := field.Interface().(syntax.Node); ok {
						walk(child, inUseCond)
					}
				}
			}
		}

		walk(f, false)
	}

	return results
}
