package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func (c *MainArgConfig) cmdSkillRemove(args []string) error {
	fs := flag.NewFlagSet("skill remove", flag.ExitOnError)
	scope := fs.String("scope", "project", "Scope to remove from (user, project)")
	agent := fs.String("agent", "common", "Target agent (common, codex, claude, copilot, cursor)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: g2 skill remove <skill-name>")
	}

	skillName := fs.Arg(0)
	if !isValidSkillName(skillName) {
		return fmt.Errorf("invalid skill name: %q", skillName)
	}

	basePath, err := getSkillBasePath(*scope, *agent)
	if err != nil {
		return err
	}

	destDir := filepath.Join(basePath, skillName)

	// Safety check to ensure we only remove something that looks like an installed skill
	if _, err := os.Stat(filepath.Join(destDir, "skill-metadata.json")); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill %q not found or not managed by g2 (missing metadata)", skillName)
		}
		return fmt.Errorf("failed to read skill metadata: %w", err)
	}

	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to remove skill directory: %w", err)
	}

	fmt.Printf("Successfully removed skill %q from %s scope for %s agent\n", skillName, *scope, *agent)
	return nil
}
