package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func (c *MainArgConfig) cmdSkillUpdate(args []string) error {
	fs := flag.NewFlagSet("skill update", flag.ExitOnError)
	force := fs.Bool("force", false, "Force update even if local modifications exist")
	all := fs.Bool("all", false, "Update all installed skills")
	scope := fs.String("scope", "project", "Scope to update from (user, project)")
	agent := fs.String("agent", "common", "Target agent (common, codex, claude, copilot, cursor)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*all && fs.NArg() < 1 {
		return fmt.Errorf("usage: g2 skill update <skill-name> [--force] or g2 skill update --all")
	}

	basePath, err := getSkillBasePath(*scope, *agent)
	if err != nil {
		return err
	}

	skillsToUpdate := []string{}
	if *all {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No skills installed.")
				return nil
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				skillsToUpdate = append(skillsToUpdate, e.Name())
			}
		}
	} else {
		skillsToUpdate = append(skillsToUpdate, fs.Arg(0))
	}

	for _, skillName := range skillsToUpdate {
		destDir := filepath.Join(basePath, skillName)

		meta, err := readSkillMetadata(destDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("no source metadata is available for %q, so this skill cannot be updated automatically\n", skillName)
				continue
			}
			return fmt.Errorf("failed to read metadata for %s: %w", skillName, err)
		}

		// Check for local modifications
		currentDigest, err := calculateDirDigest(destDir)
		if err != nil {
			return fmt.Errorf("failed to calculate digest for %s: %w", skillName, err)
		}

		if currentDigest != meta.Digest && !*force {
			fmt.Printf("installed skill %q has local modifications; rerun with --force to replace\n", skillName)
			continue
		}

		// For now we only support local path updates. Real remote updates would fetch here.
		if meta.Revision == "local" {
			srcPath := meta.Source
			if meta.Subpath != "" {
				srcPath = filepath.Join(srcPath, meta.Subpath)
			}

			// Validate source still exists
			skillMdPath := filepath.Join(srcPath, "SKILL.md")
			if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
				fmt.Printf("skill %q source no longer exists at %s\n", skillName, srcPath)
				continue
			}

			// Re-calculate source digest
			newSrcDigest, err := calculateDirDigest(srcPath)
			if err != nil {
				return err
			}

			if newSrcDigest == meta.Digest && currentDigest == meta.Digest {
				fmt.Printf("skill %q is up to date\n", skillName)
				continue
			}

			// Perform update
			if err := os.RemoveAll(destDir); err != nil {
				return err
			}

			if err := copyDir(srcPath, destDir); err != nil {
				return err
			}

			meta.Digest = newSrcDigest
			if err := writeSkillMetadata(destDir, meta); err != nil {
				return err
			}

			fmt.Printf("Successfully updated skill %q\n", skillName)
		} else {
			fmt.Printf("updating remote skills not yet supported for %q\n", skillName)
		}
	}

	return nil
}
