package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdMasksAudit(args []string) error {
	fs := flag.NewFlagSet("masks audit", flag.ContinueOnError)
	configRoot := fs.String("config-root", "/etc/portage", "Portage user configuration directory")
	reposConf := fs.String("repos-conf", "", "repos.conf file or directory (default: config-root/repos.conf)")
	profile := fs.String("profile", "", "Active profile directory (default: config-root/make.profile)")
	format := fs.String("format", "text", "Output format: text or json")
	var repositories auditRepositoryFlags
	fs.Var(&repositories, "repository", "Override configured repositories with name=path (repeatable; include all masters)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse masks audit options: %w", err)
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("masks audit accepts no positional arguments")
	}
	if *format != "text" && *format != "json" {
		return fmt.Errorf("unsupported masks audit output format %q", *format)
	}
	log.Printf("Auditing package.unmask under %s (read-only)", *configRoot)
	in, err := g2.LoadMaskAuditInput(g2.MaskAuditOptions{
		ConfigRoot: *configRoot, ReposConf: *reposConf, Profile: *profile, Repositories: repositories,
	})
	if err != nil {
		return fmt.Errorf("load masks audit inputs: %w", err)
	}
	report := g2.AuditUnmasks(in)
	log.Printf("Audited %d user unmask entries against %d available versions", len(report.Entries), len(in.Candidates))
	return writeMasksAudit(os.Stdout, report, *format)
}

type auditRepositoryFlags []*g2.RepoInfo

func (r *auditRepositoryFlags) String() string {
	var values []string
	for _, repo := range *r {
		values = append(values, repo.RepoName+"="+repo.Location)
	}
	return strings.Join(values, ", ")
}

func (r *auditRepositoryFlags) Set(value string) error {
	name, location, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(location) == "" {
		return fmt.Errorf("repository override must be name=path")
	}
	if err := g2.ValidateRepoName(name); err != nil {
		return fmt.Errorf("invalid audit repository override: %w", err)
	}
	*r = append(*r, &g2.RepoInfo{Name: name, RepoName: name, Location: location})
	return nil
}

func writeMasksAudit(w io.Writer, report g2.UnmaskAuditReport, format string) error {
	if format == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return fmt.Errorf("write masks audit JSON: %w", err)
		}
		return nil
	}
	var out strings.Builder
	for _, entry := range report.Entries {
		fmt.Fprintf(&out, "%s:%d  %s\n  %s: %s\n", entry.Source.File, entry.Source.Line, entry.Source.Atom, entry.Classification, entry.Explanation)
		for _, affected := range entry.Affected {
			fmt.Fprintf(&out, "    %s would become masked\n", affected.Candidate.String())
			for _, mask := range affected.Masks {
				fmt.Fprintf(&out, "      %s:%d  %s\n", mask.File, mask.Line, mask.Atom)
			}
		}
		for _, other := range entry.CoveredBy {
			fmt.Fprintf(&out, "    covered by %s:%d  %s\n", other.File, other.Line, other.Atom)
		}
	}
	s := report.Summary
	fmt.Fprintf(&out, "Summary: required=%d non-matching=%d no-change=%d duplicate/shadowed=%d indeterminate=%d\n",
		s.Required, s.NonMatching, s.NoChange, s.Shadowed, s.Indeterminate)
	for _, warning := range report.Warnings {
		fmt.Fprintf(&out, "Warning: %s\n", warning)
	}
	if _, err := io.WriteString(w, out.String()); err != nil {
		return fmt.Errorf("write masks audit report: %w", err)
	}
	return nil
}
