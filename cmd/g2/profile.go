package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
)

func ProfileCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected subcommand: list, describe")
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list":
		return profileListCommand(subArgs)
	case "describe":
		return profileDescribeCommand(subArgs)
	default:
		return fmt.Errorf("unknown subcommand: %s", subcommand)
	}
}

func profileListCommand(args []string) error {
	fs := flag.NewFlagSet("profile list", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "Path to repository")
	_ = fs.Parse(args)

	profilesDescBytes, err := os.ReadFile(filepath.Join(*repoDir, "profiles", "profiles.desc"))
	var profilesDescEntries []ProfileDescEntry
	if err == nil {
		profilesDescEntries = parseProfilesDesc(string(profilesDescBytes))
	}

	profilesData, err := parseProfilesDir(*repoDir, profilesDescEntries)
	if err != nil {
		return fmt.Errorf("failed to parse profiles: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "PATH\tARCH\tSTATUS")
	for _, p := range profilesData {
		arch := "-"
		status := "-"
		if p.IsDesc {
			arch = p.DescArch
			status = p.DescStat
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", p.Path, arch, status)
	}
	_ = w.Flush()

	return nil
}

func profileDescribeCommand(args []string) error {
	fs := flag.NewFlagSet("profile describe", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "Path to repository")
	_ = fs.Parse(args)

	positionals := fs.Args()
	if len(positionals) < 1 {
		return fmt.Errorf("expected profile path")
	}

	profilePath := positionals[0]

	profilesDescBytes, err := os.ReadFile(filepath.Join(*repoDir, "profiles", "profiles.desc"))
	var profilesDescEntries []ProfileDescEntry
	if err == nil {
		profilesDescEntries = parseProfilesDesc(string(profilesDescBytes))
	}

	profilesData, err := parseProfilesDir(*repoDir, profilesDescEntries)
	if err != nil {
		return fmt.Errorf("failed to parse profiles: %w", err)
	}

	var targetProfile *ProfileData
	for i, p := range profilesData {
		if p.Path == profilePath {
			targetProfile = &profilesData[i]
			break
		}
	}

	if targetProfile == nil {
		return fmt.Errorf("profile %s not found", profilePath)
	}

	fmt.Printf("Profile: %s\n", targetProfile.Path)
	if targetProfile.IsDesc {
		fmt.Printf("Arch: %s\n", targetProfile.DescArch)
		fmt.Printf("Status: %s\n", targetProfile.DescStat)
	}

	fmt.Println("\nParents:")
	if len(targetProfile.Parents) > 0 {
		for _, parent := range targetProfile.Parents {
			fmt.Printf("  - %s\n", parent)
		}
	} else {
		fmt.Println("  (none)")
	}

	fmt.Println("\nChildren:")
	if len(targetProfile.Children) > 0 {
		for _, child := range targetProfile.Children {
			fmt.Printf("  - %s\n", child)
		}
	} else {
		fmt.Println("  (none)")
	}

	fmt.Println("\nFiles:")
	if len(targetProfile.Files) > 0 {
		for name := range targetProfile.Files {
			fmt.Printf("  - %s\n", name)
		}
	} else {
		fmt.Println("  (none)")
	}

	// Calculate inherited configuration
	fmt.Println("\nInherited Configuration (Make Defaults):")

	visited := make(map[string]bool)
	var orderedParents []string

	// Collect parents via DFS
	var collectParents func(path string)
	collectParents = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true

		for _, p := range profilesData {
			if p.Path == path {
				for _, parent := range p.Parents {
					collectParents(parent)
				}
				break
			}
		}
		orderedParents = append(orderedParents, path)
	}

	collectParents(targetProfile.Path)

	makeDefaults := make(map[string]string)

	for _, parentPath := range orderedParents {
		for _, p := range profilesData {
			if p.Path == parentPath {
				if md, ok := p.Files["make.defaults"]; ok {
					makeDefaults[parentPath] = md.String()
				}
				break
			}
		}
	}

	for _, parentPath := range orderedParents {
		if content, ok := makeDefaults[parentPath]; ok {
			fmt.Printf("\n--- From %s ---\n%s\n", parentPath, content)
		}
	}

	return nil
}
