package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdConfAll(args []string) error {
	fs := flag.NewFlagSet("conf all", flag.ExitOnError)
	repoDir := fs.String("repo", "/var/db/repos/gentoo", "Path to repository")
	profilePath := fs.String("profile", "", "Path to specific profile (relative to repo/profiles)")
	makeConfOverride := fs.String("make-conf", "/etc/portage/make.conf", "Path to make.conf")
	portageConfDir := fs.String("config-root", "/etc/portage", "Path to portage config root (e.g. /etc/portage)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	makeConfPath := *makeConfOverride

	// 1. Get Make.conf
	fmt.Printf("=== make.conf (%s) ===\n", makeConfPath)
	vars, err := g2.ParseMakeConf(makeConfPath)
	if err != nil {
		fmt.Printf("Warning: failed to parse make.conf: %v\n", err)
		vars = make(map[string]string)
	}

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s=%s\n", k, vars[k])
	}
	fmt.Println()

	// 2. Get Profile
	profileToUse := *profilePath
	makeProfilePath := filepath.Join(*portageConfDir, "make.profile")

	if profileToUse == "" {
		// Read /etc/portage/make.profile symlink
		target, err := os.Readlink(makeProfilePath)
		if err == nil {
			profileToUse = target
		} else {
			fmt.Printf("Warning: failed to read %s: %v\n", makeProfilePath, err)
		}
	}

	if profileToUse != "" {
		fmt.Printf("=== Profile: %s ===\n", profileToUse)

		profilesDescBytes, err := os.ReadFile(filepath.Join(*repoDir, "profiles", "profiles.desc"))
		var profilesDescEntries []g2.ProfileDescEntry
		if err == nil {
			profilesDescEntries = parseProfilesDesc(string(profilesDescBytes))
		}

		profilesData, err := parseProfilesDir(*repoDir, profilesDescEntries)
		if err != nil {
			fmt.Printf("Warning: failed to parse profiles: %v\n", err)
		} else {

			// Try to find the profile
			var targetProfile *g2.ProfileData

			// Resolve absolute profileToUse to relative path if possible
			relProfilePath := profileToUse
			if filepath.IsAbs(profileToUse) {
				profilesDirAbs, err := filepath.Abs(filepath.Join(*repoDir, "profiles"))
				if err == nil {
					rel, err := filepath.Rel(profilesDirAbs, profileToUse)
					if err == nil && !strings.HasPrefix(rel, "..") {
						relProfilePath = rel
					}
				}
			}

			// Try resolving as relative from just repo/profiles
			if strings.HasPrefix(relProfilePath, *repoDir) {
				rel, err := filepath.Rel(filepath.Join(*repoDir, "profiles"), relProfilePath)
				if err == nil && !strings.HasPrefix(rel, "..") {
					relProfilePath = rel
				}
			}
			// Just blindly try to clean it
			if filepath.IsAbs(relProfilePath) {
				p := filepath.Clean(relProfilePath)
				idx := strings.Index(p, "profiles/")
				if idx != -1 {
					relProfilePath = p[idx+len("profiles/"):]
				}
			}

			for i, p := range profilesData {
				if p.Path == relProfilePath || p.Path == profileToUse {
					targetProfile = &profilesData[i]
					break
				}
			}

			if targetProfile != nil {
				visited := make(map[string]bool)
				var orderedParents []string

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
								makeDefaults[parentPath] = md
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
			} else {
				fmt.Printf("Profile %s (rel: %s) not found in parsed data\n", profileToUse, relProfilePath)
			}
		}
	} else {
		fmt.Println("=== Profile ===")
		fmt.Println("No profile found or specified.")
	}

	return nil
}
