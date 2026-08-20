package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdConfOverlay(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing repository name")
	}

	repoName := args[0]
	subArgs := args[1:]

	if len(subArgs) == 0 {
		return fmt.Errorf("missing subcommand for conf overlay")
	}

	subcmd := subArgs[0]

	switch subcmd {
	case "mask":
		return cfg.cmdConfOverlayMask(repoName, subArgs[1:])
	case "unmask":
		return cfg.cmdConfOverlayUnmask(repoName, subArgs[1:])
	case "mask-reset":
		return cfg.cmdConfOverlayMaskReset(repoName, subArgs[1:])
	case "unmask-reset":
		return cfg.cmdConfOverlayUnmaskReset(repoName, subArgs[1:])
	case "list":
		return cfg.cmdConfOverlayList(repoName, subArgs[1:])
	default:
		return fmt.Errorf("unknown conf overlay subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) resolveRepo(repoName string, reposConfPath string) (string, error) {
	rc, err := g2.ParseReposConf(reposConfPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("parsing repos.conf: %w", err)
		}
	} else {
		for _, f := range rc.Files {
			for _, s := range f.Sections {
				if s.Name == repoName && !s.Disabled {
					loc := s.Get("location")
					if loc != "" {
						return repoName, nil
					}
				}
			}
		}
	}

	reposDir := "/var/db/repos"
	if entries, err := os.ReadDir(reposDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && entry.Name() == repoName {
				return repoName, nil
			}
		}
	}

	return "", fmt.Errorf("repository %q not found in repos.conf or /var/db/repos", repoName)
}

func (cfg *MainArgConfig) cmdConfOverlayMask(repoName string, args []string) error {
	fs := flag.NewFlagSet("mask", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
	reposConfOpt := fs.String("repos-conf", "/etc/portage/repos.conf", "Path to repos.conf")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing package name")
	}

	_, err := cfg.resolveRepo(repoName, *reposConfOpt)
	if err != nil {
		return err
	}

	pkgName := fs.Arg(0)

	if strings.Contains(pkgName, "::") {
		parts := strings.SplitN(pkgName, "::", 2)
		if parts[1] != repoName {
			return fmt.Errorf("package qualifier %q does not match selected repository %q", parts[1], repoName)
		}
	} else {
		pkgName = fmt.Sprintf("%s::%s", pkgName, repoName)
	}

	return appendToConfig(filepath.Join(*configRootOpt, "package.mask"), pkgName)
}

func (cfg *MainArgConfig) cmdConfOverlayUnmask(repoName string, args []string) error {
	fs := flag.NewFlagSet("unmask", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
	reposConfOpt := fs.String("repos-conf", "/etc/portage/repos.conf", "Path to repos.conf")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing package name")
	}

	_, err := cfg.resolveRepo(repoName, *reposConfOpt)
	if err != nil {
		return err
	}


	pkgName := fs.Arg(0)

	if strings.Contains(pkgName, "::") {
		parts := strings.SplitN(pkgName, "::", 2)
		if parts[1] != repoName {
			return fmt.Errorf("package qualifier %q does not match selected repository %q", parts[1], repoName)
		}
	} else {
		pkgName = fmt.Sprintf("%s::%s", pkgName, repoName)
	}


	return appendToConfig(filepath.Join(*configRootOpt, "package.unmask"), pkgName)
}

func (cfg *MainArgConfig) cmdConfOverlayMaskReset(repoName string, args []string) error {
	fs := flag.NewFlagSet("mask-reset", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
	reposConfOpt := fs.String("repos-conf", "/etc/portage/repos.conf", "Path to repos.conf")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing package name")
	}

	_, err := cfg.resolveRepo(repoName, *reposConfOpt)
	if err != nil {
		return err
	}


	pkgName := fs.Arg(0)
	if !strings.Contains(pkgName, "::") {
		pkgName = fmt.Sprintf("%s::%s", pkgName, repoName)
	}
	return removeFromConfig(filepath.Join(*configRootOpt, "package.mask"), pkgName)
}

func (cfg *MainArgConfig) cmdConfOverlayUnmaskReset(repoName string, args []string) error {
	fs := flag.NewFlagSet("unmask-reset", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
	reposConfOpt := fs.String("repos-conf", "/etc/portage/repos.conf", "Path to repos.conf")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		return fmt.Errorf("missing package name")
	}

	_, err := cfg.resolveRepo(repoName, *reposConfOpt)
	if err != nil {
		return err
	}

	pkgName := fs.Arg(0)
	if !strings.Contains(pkgName, "::") {
		pkgName = fmt.Sprintf("%s::%s", pkgName, repoName)
	}

	return removeFromConfig(filepath.Join(*configRootOpt, "package.unmask"), pkgName)
}


func (cfg *MainArgConfig) cmdConfOverlayList(repoName string, args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
	reposConfOpt := fs.String("repos-conf", "/etc/portage/repos.conf", "Path to repos.conf")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	_, err := cfg.resolveRepo(repoName, *reposConfOpt)
	if err != nil {
		return err
	}


	fmt.Printf("Masks:\n")
	userMasked, err := parsePackageMaskDir(filepath.Join(*configRootOpt, "package.mask"))
	if err == nil {
		for _, m := range userMasked {
			for _, entry := range m.Entries {
				if strings.HasSuffix(entry.Package, "::"+repoName) {
					fmt.Printf("  %s\n", entry.Package)
				}
			}
		}
	}

	fmt.Printf("Unmasks:\n")
	userUnmasked, err := parsePackageMaskDir(filepath.Join(*configRootOpt, "package.unmask"))
	if err == nil {
		for _, m := range userUnmasked {
			for _, entry := range m.Entries {
				if strings.HasSuffix(entry.Package, "::"+repoName) {
					fmt.Printf("  %s\n", entry.Package)
				}
			}
		}
	}


	return nil
}



func appendToConfig(path string, pkgName string) error {
	targetFile := path
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		targetFile = filepath.Join(path, "g2.conf")
	} else if os.IsNotExist(err) {
		os.MkdirAll(path, 0755)
		targetFile = filepath.Join(path, "g2.conf")
	}

	content, err := os.ReadFile(targetFile)
	if err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == pkgName {
				fmt.Printf("Rule already exists in %s\n", targetFile)
				return nil
			}
		}
	}

	f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := fmt.Fprintf(f, "%s\n", pkgName); err != nil {
		return fmt.Errorf("writing to file: %w", err)
	}

	fmt.Printf("Added %s to %s\n", pkgName, targetFile)
	return nil
}

func removeFromConfig(path string, pkgName string) error {
	removeFromFile := func(file string) error {
		content, err := os.ReadFile(file)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		lines := strings.Split(string(content), "\n")
		var newLines []string
		found := false
		for _, line := range lines {
			if strings.TrimSpace(line) == pkgName {
				found = true
			} else {
				newLines = append(newLines, line)
			}
		}

		if found {
			// Write safely
			tempFile := file + ".tmp"
			f, err := os.Create(tempFile)
			if err != nil {
				return err
			}
			for i, line := range newLines {
				if i < len(newLines)-1 || line != "" {
					_, err := fmt.Fprintf(f, "%s\n", line)
					if err != nil {
						f.Close()
						os.Remove(tempFile)
						return err
					}
				}
			}

			// check if file is empty
			if err := f.Close(); err != nil {
				return err
			}

			if err := os.Rename(tempFile, file); err != nil {
				os.Remove(tempFile)
				return err
			}
			fmt.Printf("Removed %s from %s\n", pkgName, file)
		}
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return removeFromFile(path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			if err := removeFromFile(filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
