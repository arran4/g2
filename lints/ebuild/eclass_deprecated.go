package ebuild

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
	"github.com/arran4/g2/lints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var ruleEclassDeprecated = lints.RuleMetadata{
	ID:          "EclassDeprecated",
	Title:       "Deprecated eclasses",
	Description: "Deprecated eclasses should not be used in new ebuilds.",
	References: []lints.RuleReference{
		{URL: "https://projects.gentoo.org/qa/policy-guide/deprecation.html#pg1003", Label: "Gentoo QA Policy Guide PG1003"},
	},
	Severity: lints.SeverityWarning,
	Source:   lints.SourceG2,
	Tags:     []string{"ebuild", "gentoo-policy", "PG1003"},
}

// Ensure the rule is registered with the correct paths. We expose them for testing if needed.
var DefaultReposConfPath = "/etc/portage/repos.conf"
var DefaultReposBasePath = "/var/db/repos"

func init() {
	lints.RegisterRuleMetadata(ruleEclassDeprecated)
	lints.RegisterRepoLintRule(&EclassDeprecatedLintRule{})
}

type EclassDeprecatedLintRule struct{}

func (r *EclassDeprecatedLintRule) LintRepo(repoDir string, site *g2.SiteData) []lints.LintResult {
	var results []lints.LintResult
	severity := lints.SeverityWarning

	if site != nil && site.QAPolicy != nil && site.QAPolicy.Policies != nil {
		if val, ok := site.QAPolicy.Policies["PG1003"]; ok {
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

	deprecatedEclasses := make(map[string]bool)

	// Helper to extract deprecated eclass names from an eclass file quickly
	findDeprecatedEclasses := func(eclassPath string) {
		file, err := os.Open(eclassPath)
		if err != nil {
			return
		}
		defer func() { _ = file.Close() }()

		// Scan file line by line, looking for @DEPRECATED
		scanner := bufio.NewScanner(file)
		// Create a custom buffer to handle very long lines, e.g. large arrays
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024*10) // 10MB max line length
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(strings.TrimSpace(line), "# @DEPRECATED:") {
				eclassName := strings.TrimSuffix(filepath.Base(eclassPath), ".eclass")
				deprecatedEclasses[eclassName] = true
				break
			}
		}
		if err := scanner.Err(); err != nil {
			// Just ignore and continue as per original behavior of ignoring read errors (except missing file)
		}
	}

	// Helper to find deprecated eclasses in a directory
	findDeprecatedEclassesInDir := func(eclassDir string) {
		if info, err := os.Stat(eclassDir); err == nil && info.IsDir() {
			entries, err := os.ReadDir(eclassDir)
			if err == nil {
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".eclass") {
						findDeprecatedEclasses(filepath.Join(eclassDir, e.Name()))
					}
				}
			}
		}
	}

	if site != nil {
		// First check locally
		for _, eclass := range site.Eclasses {
			if eclass == nil {
				continue
			}
			lines := strings.Split(eclass.RawText, "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "# @DEPRECATED:") {
					eclassName := strings.TrimSuffix(filepath.Base(eclass.Path), ".eclass")
					deprecatedEclasses[eclassName] = true
					break
				}
			}
		}

		// Parse metadata/layout.conf for masters (upstream overlays)
		if site.LayoutConf != nil {
			var masters []string
			for _, entry := range site.LayoutConf.Entries {
				if entry.Key == "masters" {
					masters = strings.Fields(entry.Value)
					break
				}
			}

			if len(masters) > 0 {
				// Determine paths for upstream masters
				masterPaths := make(map[string]string)

				// Usually gentoo puts its default repos.conf in /etc/portage/repos.conf
				// We'll parse it if it exists to get absolute paths.
				if rc, err := g2.ParseReposConf(DefaultReposConfPath); err == nil && rc != nil {
					for _, file := range rc.Files {
						for _, sec := range file.Sections {
							if loc := sec.Get("location"); loc != "" {
								masterPaths[sec.Name] = loc
							}
						}
					}
				}

				for _, master := range masters {
					loc, ok := masterPaths[master]
					if !ok {
						// fallback to a generic path if not found in repos.conf
						loc = filepath.Join(DefaultReposBasePath, master)
					}
					findDeprecatedEclassesInDir(filepath.Join(loc, "eclass"))
				}
			}
		}
	}

	if site != nil {
		// Loop over packages to check inheritance
		for _, cat := range site.Categories {
			for _, pkg := range cat.Packages {
				for _, ver := range pkg.Versions {
					if ver.Ebuild == nil || ver.Ebuild.Vars == nil {
						continue
					}

					inheritedStr := ver.Ebuild.Vars["INHERITED"]
					if inheritedStr == "" {
						continue
					}

					inheritedList := strings.Fields(inheritedStr)
					for _, inherited := range inheritedList {
						if deprecatedEclasses[inherited] {
							res := lints.LintResult{
								RuleMetadata: ruleEclassDeprecated,
								Message:      fmt.Sprintf("[%s] Ebuild %s inherits a deprecated eclass '%s'.", cases.Title(language.Und, cases.NoLower).String(string(severity)), ver.Version, inherited),
								Package:      pkg.Category + "/" + pkg.Name,
							}
							res.RuleMetadata.Severity = severity
							results = append(results, res)
						}
					}
				}
			}
		}
	}

	return results
}
