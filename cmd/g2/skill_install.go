package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (c *MainArgConfig) cmdSkillInstall(args []string) error {
	fs := flag.NewFlagSet("skill install", flag.ExitOnError)
	scope := fs.String("scope", "project", "Scope to install the skill (user, project)")
	agent := fs.String("agent", "common", "Target agent (common, codex, claude, copilot, cursor)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: g2 skill install <source> [skill-name-or-path]")
	}

	source := fs.Arg(0)
	skillNameOrPath := ""
	if fs.NArg() > 1 {
		skillNameOrPath = fs.Arg(1)
	} else {
		skillNameOrPath = filepath.Base(source)
	}

	basePath, err := getSkillBasePath(*scope, *agent)
	if err != nil {
		return err
	}

	skillName := filepath.Base(skillNameOrPath)
	if !isValidSkillName(skillName) {
		return fmt.Errorf("invalid skill name: %q", skillNameOrPath)
	}
	destDir := filepath.Join(basePath, skillName)

	// Ensure destination directory is clean
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to clear destination directory: %w", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	var srcPath string
	var subPath string

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "git://") {
		return fmt.Errorf("remote git sources not yet supported")
	} else if strings.Contains(source, "/") && !strings.HasPrefix(source, ".") && !strings.HasPrefix(source, "/") {
		return fmt.Errorf("github sources not yet supported")
	} else {
		// local filesystem path
		srcPath = filepath.Clean(source)
		if fs.NArg() > 1 && strings.Contains(skillNameOrPath, string(filepath.Separator)) {
			srcPath = filepath.Join(srcPath, skillNameOrPath)
			subPath = skillNameOrPath
		}
	}

	// Check if SKILL.md exists
	skillMdPath := filepath.Join(srcPath, "SKILL.md")
	if _, err := os.Stat(skillMdPath); os.IsNotExist(err) {
		// Try to see if it's a multi-skill dir without explicit subpath
		if fs.NArg() == 1 {
			entries, _ := os.ReadDir(filepath.Join(srcPath, "skills"))
			if len(entries) > 0 {
				names := []string{}
				for _, e := range entries {
					if e.IsDir() {
						names = append(names, e.Name())
					}
				}
				if len(names) > 0 {
					return fmt.Errorf("repository contains multiple skills: %s; specify one explicitly", strings.Join(names, ", "))
				}
			}
		}
		return fmt.Errorf("SKILL.md not found in source: %s", srcPath)
	}

	if err := copyDir(srcPath, destDir); err != nil {
		return fmt.Errorf("failed to copy skill files: %w", err)
	}

	digest, err := calculateDirDigest(destDir)
	if err != nil {
		return fmt.Errorf("failed to calculate digest: %w", err)
	}

	meta := &SkillMetadata{
		Name:             filepath.Base(destDir),
		Source:           source,
		Subpath:          subPath,
		Revision:         "local", // or commit hash if git
		Digest:           digest,
		InstallationTime: time.Now(),
		Scope:            *scope,
		Agent:            *agent,
	}

	if err := writeSkillMetadata(destDir, meta); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	fmt.Printf("Successfully installed skill %q to %s\n", meta.Name, destDir)
	return nil
}
