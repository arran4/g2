package ebuild

import (
	"fmt"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
)

func init() {
	lints.RegisterLintRule(&IUSELintRule{})
}

type IUSELintRule struct{}

func (r *IUSELintRule) Lint(repoDir string, pkg *g2.PackageData) []string {
	var warnings []string
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
						warnings = append(warnings, fmt.Sprintf("Ebuild %s uses IUSE flag '%s' which is not documented in metadata.xml. Add the flag to metadata.xml <use> section.", ver.Version, f))
					}
				}
			}
		}
	}
	return warnings
}
