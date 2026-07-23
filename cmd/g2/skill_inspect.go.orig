package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func (c *MainArgConfig) cmdSkillInspect(args []string) error {
	fs := flag.NewFlagSet("skill inspect", flag.ExitOnError)
	scope := fs.String("scope", "project", "Scope to inspect from (user, project)")
	agent := fs.String("agent", "common", "Target agent (common, codex, claude, copilot, cursor)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: g2 skill inspect <skill-name>")
	}

	skillName := fs.Arg(0)

	basePath, err := getSkillBasePath(*scope, *agent)
	if err != nil {
		return err
	}

	destDir := filepath.Join(basePath, skillName)

	meta, err := readSkillMetadata(destDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("skill %q not found or not managed by g2 (missing metadata)", skillName)
		}
		return fmt.Errorf("failed to read skill metadata: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))

	// Also print out local modifications if any
	currentDigest, err := calculateDirDigest(destDir)
	if err == nil && currentDigest != meta.Digest {
		fmt.Printf("\nWARNING: Installed skill has local modifications.\n")
	}

	return nil
}
