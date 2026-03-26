package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	lints.RegisterLintRule(&IUSELintRule{})
}

type IUSELintRule struct{}

func (r *IUSELintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	return r.LintWithQA(repoDir, pkg, nil)
}

func (r *IUSELintRule) LintWithQA(repoDir string, pkg *g2.PackageData, qa *g2.QAPolicy) []string {
	var warnings []string

	severity := "Warning"
	if qa != nil && qa.Policies != nil {
		if val, ok := qa.Policies["PG0801"]; ok { // Arbitrary PG code mapping for IUSE for example, or general
			if val == "notice" || val == "error" || val == "warning" {
				severity = cases.Title(language.English).String(val)
			}
		}
	}
	if pkg.Metadata == nil {
		return warnings
	}

	metaUseFlags := make(map[string]bool)
	for _, use := range pkg.Metadata.Use {
		for _, flag := range use.Flags {
			metaUseFlags[flag.Name] = true
		}
	}

	for _, ver := range pkg.Versions {
		if ver.Ebuild != nil && ver.Ebuild.Vars != nil {
			iuseRaw := strings.ReplaceAll(ver.Ebuild.Vars["IUSE"], "\"", "")
			if iuseRaw != "" {
				iuseFlags := strings.Fields(iuseRaw)
				for _, f := range iuseFlags {
					f = strings.TrimPrefix(f, "+")
					f = strings.TrimPrefix(f, "-")

					globalFlags := map[string]bool{"test": true, "doc": true, "debug": true}

					if !metaUseFlags[f] && !globalFlags[f] {
						warnings = append(warnings, fmt.Sprintf("[%s] Ebuild %s uses IUSE flag '%s' which is not documented in metadata.xml. Add the flag to metadata.xml <use> section.", severity, ver.Version, f))
					}
				}
			}
		}
	}
	return warnings
}
