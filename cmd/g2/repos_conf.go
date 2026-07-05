package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (cfg *MainArgConfig) cmdReposConf(args []string) error {
	fs := flag.NewFlagSet("repos-conf", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 repos-conf <subcommand>\n")
		fmt.Printf("\ttui [location]\t\t\tOpen terminal UI to manage repos.conf\n")
		fmt.Printf("\tlist [location]\t\t\tList all parsed sections and files (default: /etc/portage/repos.conf)\n")
		fmt.Printf("\tset <repo> <key> <value>\tSet a value in a repository section\n")
		fmt.Printf("\tunset <repo> <key>\t\tUnset a value in a repository section\n")
		fmt.Printf("\tdisable <repo>\t\t\tDisable a repository section\n")
		fmt.Printf("\tenable <repo>\t\t\tEnable a repository section\n")
		fmt.Printf("\tmove <repo> <file>\t\tMove a repository section to a different file (useful for directory formats)\n")
	}

	locationOpt := fs.String("location", "/etc/portage/repos.conf", "Path to repos.conf file or directory")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand for repos-conf")
	}

	subcmd := fs.Arg(0)
	switch subcmd {
	case "tui":
		return cfg.cmdReposConfTUI(*locationOpt)
	case "list":
		return cfg.cmdReposConfList(*locationOpt)
	case "set":
		return cfg.cmdReposConfSet(*locationOpt, fs.Args()[1:])
	case "unset":
		return cfg.cmdReposConfUnset(*locationOpt, fs.Args()[1:])
	case "disable":
		return cfg.cmdReposConfDisable(*locationOpt, fs.Args()[1:])
	case "enable":
		return cfg.cmdReposConfEnable(*locationOpt, fs.Args()[1:])
	case "move":
		return cfg.cmdReposConfMove(*locationOpt, fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown repos-conf subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) cmdReposConfList(location string) error {
	rc, err := g2.ParseReposConf(location)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", location, err)
	}

	for _, f := range rc.Files {
		fmt.Printf("File: %s\n", f.Path)
		for _, s := range f.Sections {
			status := ""
			if s.Disabled {
				status = " (disabled)"
			}
			fmt.Printf("  Section: %s%s\n", s.Name, status)
			for _, line := range s.Lines {
				fmt.Printf("    %s\n", line)
			}
		}
	}
	return nil
}

func (cfg *MainArgConfig) cmdReposConfSet(location string, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: g2 repos-conf set <repo> <key> <value>")
	}
	repo, key, value := args[0], args[1], args[2]

	rc, err := g2.ParseReposConf(location)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", location, err)
	}

	var targetFile *g2.ReposConfFile
	var targetSection *g2.ReposConfSection

	for _, f := range rc.Files {
		for _, s := range f.Sections {
			if s.Name == repo {
				targetFile = f
				targetSection = s
				break
			}
		}
		if targetFile != nil {
			break
		}
	}

	if targetFile == nil {
		// Not found, create it in the main file or in a new file if it's a dir
		if rc.IsDir {
			newFilePath := filepath.Join(location, repo+".conf")
			targetFile = &g2.ReposConfFile{Path: newFilePath}
			rc.Files = append(rc.Files, targetFile)
		} else {
			if len(rc.Files) == 0 {
				targetFile = &g2.ReposConfFile{Path: location}
				rc.Files = append(rc.Files, targetFile)
			} else {
				targetFile = rc.Files[0]
			}
		}
		targetSection = &g2.ReposConfSection{Name: repo}
		targetFile.Sections = append(targetFile.Sections, targetSection)
	}

	targetSection.Set(key, value)
	return targetFile.Write()
}

func (cfg *MainArgConfig) cmdReposConfUnset(location string, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: g2 repos-conf unset <repo> <key>")
	}
	repo, key := args[0], args[1]

	rc, err := g2.ParseReposConf(location)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", location, err)
	}

	for _, f := range rc.Files {
		for _, s := range f.Sections {
			if s.Name == repo {
				s.Unset(key)
				return f.Write()
			}
		}
	}
	log.Printf("Repository section '%s' not found.", repo)
	return nil
}

func (cfg *MainArgConfig) cmdReposConfDisable(location string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: g2 repos-conf disable <repo>")
	}
	repo := args[0]

	rc, err := g2.ParseReposConf(location)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", location, err)
	}

	for _, f := range rc.Files {
		for _, s := range f.Sections {
			if s.Name == repo {
				s.Disable()
				return f.Write()
			}
		}
	}
	return fmt.Errorf("repository section '%s' not found", repo)
}

func (cfg *MainArgConfig) cmdReposConfEnable(location string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: g2 repos-conf enable <repo>")
	}
	repo := args[0]

	rc, err := g2.ParseReposConf(location)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", location, err)
	}

	for _, f := range rc.Files {
		for _, s := range f.Sections {
			if s.Name == repo {
				s.Enable()
				return f.Write()
			}
		}
	}
	return fmt.Errorf("repository section '%s' not found", repo)
}

func (cfg *MainArgConfig) cmdReposConfMove(location string, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: g2 repos-conf move <repo> <target_file>")
	}
	repo, targetFileName := args[0], args[1]

	rc, err := g2.ParseReposConf(location)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", location, err)
	}

	if !rc.IsDir {
		return fmt.Errorf("moving sections is only useful when repos.conf is a directory")
	}

	var sourceFile *g2.ReposConfFile
	var targetSection *g2.ReposConfSection
	var sectionIndex int

	for _, f := range rc.Files {
		for i, s := range f.Sections {
			if s.Name == repo {
				sourceFile = f
				targetSection = s
				sectionIndex = i
				break
			}
		}
		if sourceFile != nil {
			break
		}
	}

	if sourceFile == nil {
		return fmt.Errorf("repository section '%s' not found", repo)
	}

	targetPath := filepath.Join(location, targetFileName)
	if sourceFile.Path == targetPath {
		log.Printf("Section is already in %s", targetPath)
		return nil
	}

	// Remove from source
	sourceFile.Sections = append(sourceFile.Sections[:sectionIndex], sourceFile.Sections[sectionIndex+1:]...)

	var destFile *g2.ReposConfFile
	for _, f := range rc.Files {
		if f.Path == targetPath {
			destFile = f
			break
		}
	}

	if destFile == nil {
		destFile = &g2.ReposConfFile{Path: targetPath}
		rc.Files = append(rc.Files, destFile)
	}

	destFile.Sections = append(destFile.Sections, targetSection)

	if err := sourceFile.Write(); err != nil {
		return fmt.Errorf("writing source file: %w", err)
	}
	if err := destFile.Write(); err != nil {
		return fmt.Errorf("writing destination file: %w", err)
	}

	log.Printf("Moved section '%s' to %s", repo, targetFileName)

	// Optional: remove source file if empty
	if len(sourceFile.Sections) == 0 && len(sourceFile.HeaderLines) == 0 {
		_ = os.Remove(sourceFile.Path)
	}

	return nil
}

func (cfg *MainArgConfig) cmdReposConfTUI(location string) error {
	info, err := os.Stat(location)
	if err == nil && info.IsDir() {
		return fmt.Errorf("location %s is a directory; please specify a file to edit", location)
	}

	content, err := os.ReadFile(location)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading repos.conf: %w", err)
		}
		content = []byte{}
	}

	lines := strings.Split(string(content), "\n")
	return runConfTUI(location, lines, "repos.conf")
}
