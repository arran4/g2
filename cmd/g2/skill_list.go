package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

func (c *MainArgConfig) cmdSkillList(args []string) error {
	fs := flag.NewFlagSet("skill list", flag.ExitOnError)
	scope := fs.String("scope", "project", "Scope to list from (user, project, all)")
	agent := fs.String("agent", "common", "Target agent (common, codex, claude, copilot, cursor)")
	format := fs.String("format", "table", "Output format (table, json)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	scopesToSearch := []string{*scope}
	if *scope == "all" {
		scopesToSearch = []string{"user", "project"}
	}

	agentsToSearch := []string{*agent}
	if *agent == "all" {
		agentsToSearch = []string{"common", "codex", "claude", "copilot", "cursor"}
	}

	var allSkills []*SkillMetadata

	for _, s := range scopesToSearch {
		for _, a := range agentsToSearch {
			basePath, err := getSkillBasePath(s, a)
			if err != nil {
				continue
			}

			entries, err := os.ReadDir(basePath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", basePath, err)
				continue
			}

			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				destDir := filepath.Join(basePath, e.Name())
				meta, err := readSkillMetadata(destDir)
				if err != nil {
					continue // likely not a g2 skill
				}

				// override with where it was actually found in case it was moved
				meta.Scope = s
				meta.Agent = a

				allSkills = append(allSkills, meta)
			}
		}
	}

	if *format == "json" {
		data, err := json.MarshalIndent(allSkills, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(allSkills) == 0 {
		fmt.Println("No skills installed.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSCOPE\tAGENT\tREVISION\tSOURCE")

	for _, skill := range allSkills {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			skill.Name,
			skill.Scope,
			skill.Agent,
			skill.Revision,
			skill.Source,
		)
	}
	_ = w.Flush()

	return nil
}
