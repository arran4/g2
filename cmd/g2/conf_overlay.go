package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/arran4/g2"
)

// parseFlagsAndArgs parses a FlagSet repeatedly to permit interleaved flags and positional arguments.
func parseFlagsAndArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		remaining := fs.Args()
		if len(remaining) == 0 {
			break
		}
		positional = append(positional, remaining[0])
		args = remaining[1:]
	}
	return positional, nil
}

func (cfg *MainArgConfig) cmdConfOverlay(args []string) error {
	fs := flag.NewFlagSet("conf overlay", flag.ContinueOnError)
	configRootOpt := fs.String("config-root", "/etc/portage", "Path to config root")
	reposConfOpt := fs.String("repos-conf", "", "Path to repos.conf")

	positional, err := parseFlagsAndArgs(fs, args)
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if len(positional) == 0 {
		return fmt.Errorf("missing repository or subcommand for conf overlay")
	}

	// Check if the command is `g2 conf overlay list`
	if positional[0] == "list" {
		if len(positional) > 1 {
			return fmt.Errorf("unexpected argument %q for conf overlay list", positional[1])
		}
		rcPath := *reposConfOpt
		if rcPath == "" {
			rcPath = filepath.Join(*configRootOpt, "repos.conf")
		}
		return cfg.runConfOverlayListAll(rcPath)
	}

	repo := positional[0]
	if len(positional) < 2 {
		return fmt.Errorf("missing subcommand for repository %q", repo)
	}
	subcmd := positional[1]

	return cfg.dispatchRepoSubcmd(repo, subcmd, *configRootOpt, *reposConfOpt, positional[2:])
}

func (cfg *MainArgConfig) dispatchRepoSubcmd(repo, subcmd, defaultCfgRoot, defaultReposConf string, positional []string) error {
	rcPath := defaultReposConf
	if rcPath == "" {
		rcPath = filepath.Join(defaultCfgRoot, "repos.conf")
	}

	switch subcmd {
	case "list":
		if len(positional) > 0 {
			return fmt.Errorf("unexpected argument %q for conf overlay %s list", positional[0], repo)
		}
		return cfg.cmdConfOverlayRepoList(repo, defaultCfgRoot, rcPath)
	case "mask":
		if len(positional) == 0 {
			return fmt.Errorf("missing package atom")
		}
		if len(positional) > 1 {
			return fmt.Errorf("unexpected extra argument %q for conf overlay %s mask", positional[1], repo)
		}
		return cfg.cmdConfOverlayRepoMask(repo, defaultCfgRoot, rcPath, positional[0])
	case "unmask":
		if len(positional) == 0 {
			return fmt.Errorf("missing package atom")
		}
		if len(positional) > 1 {
			return fmt.Errorf("unexpected extra argument %q for conf overlay %s unmask", positional[1], repo)
		}
		return cfg.cmdConfOverlayRepoUnmask(repo, defaultCfgRoot, rcPath, positional[0])
	case "mask-reset":
		if len(positional) == 0 {
			return fmt.Errorf("missing package atom")
		}
		if len(positional) > 1 {
			return fmt.Errorf("unexpected extra argument %q for conf overlay %s mask-reset", positional[1], repo)
		}
		return cfg.cmdConfOverlayRepoMaskReset(repo, defaultCfgRoot, rcPath, positional[0])
	case "unmask-reset":
		if len(positional) == 0 {
			return fmt.Errorf("missing package atom")
		}
		if len(positional) > 1 {
			return fmt.Errorf("unexpected extra argument %q for conf overlay %s unmask-reset", positional[1], repo)
		}
		return cfg.cmdConfOverlayRepoUnmaskReset(repo, defaultCfgRoot, rcPath, positional[0])
	default:
		return fmt.Errorf("unknown conf overlay subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) runConfOverlayListAll(rcPath string) error {
	repos, err := g2.ListConfiguredRepos(rcPath)
	if err != nil {
		return fmt.Errorf("listing repositories: %w", err)
	}

	var names []string
	seen := make(map[string]bool)
	for _, r := range repos {
		if !seen[r.RepoName] {
			seen[r.RepoName] = true
			names = append(names, r.RepoName)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}

func (cfg *MainArgConfig) cmdConfOverlayRepoList(repo, configRoot, rcPath string) error {
	repoInfo, err := g2.ResolveRepo(repo, rcPath)
	if err != nil {
		return err
	}

	maskEntries, err := g2.ReadUserConfigEntries(filepath.Join(configRoot, "package.mask"))
	if err != nil {
		return fmt.Errorf("reading package.mask: %w", err)
	}

	unmaskEntries, err := g2.ReadUserConfigEntries(filepath.Join(configRoot, "package.unmask"))
	if err != nil {
		return fmt.Errorf("reading package.unmask: %w", err)
	}

	var repoMasks []g2.UserConfigEntry
	for _, e := range maskEntries {
		if e.Atom.Repo == repoInfo.RepoName {
			repoMasks = append(repoMasks, e)
		}
	}

	var repoUnmasks []g2.UserConfigEntry
	for _, e := range unmaskEntries {
		if e.Atom.Repo == repoInfo.RepoName {
			repoUnmasks = append(repoUnmasks, e)
		}
	}

	fmt.Printf("Repository: %s\n\n", repoInfo.RepoName)

	fmt.Printf("Masks:\n")
	for _, m := range repoMasks {
		fmt.Printf("  %s\n", m.AtomString)
		fmt.Printf("    %s:%d\n", m.FilePath, m.LineNumber)
	}

	fmt.Printf("\nUnmasks:\n")
	for _, u := range repoUnmasks {
		fmt.Printf("  %s\n", u.AtomString)
		fmt.Printf("    %s:%d\n", u.FilePath, u.LineNumber)
	}

	return nil
}

func (cfg *MainArgConfig) cmdConfOverlayRepoMask(repo, configRoot, rcPath, atomArg string) error {
	repoInfo, err := g2.ResolveRepo(repo, rcPath)
	if err != nil {
		return err
	}

	qualifiedAtom, err := g2.QualifyAtomForRepo(atomArg, repoInfo.RepoName)
	if err != nil {
		return err
	}

	maskPath := filepath.Join(configRoot, "package.mask")
	added, targetFile, err := g2.AddUserConfigAtom(maskPath, qualifiedAtom)
	if err != nil {
		return fmt.Errorf("masking package: %w", err)
	}

	if added {
		fmt.Printf("Added %s to %s\n", qualifiedAtom, targetFile)
	} else {
		fmt.Printf("Rule %s already exists in %s\n", qualifiedAtom, targetFile)
	}
	return nil
}

func (cfg *MainArgConfig) cmdConfOverlayRepoUnmask(repo, configRoot, rcPath, atomArg string) error {
	repoInfo, err := g2.ResolveRepo(repo, rcPath)
	if err != nil {
		return err
	}

	qualifiedAtom, err := g2.QualifyAtomForRepo(atomArg, repoInfo.RepoName)
	if err != nil {
		return err
	}

	unmaskPath := filepath.Join(configRoot, "package.unmask")
	added, targetFile, err := g2.AddUserConfigAtom(unmaskPath, qualifiedAtom)
	if err != nil {
		return fmt.Errorf("unmasking package: %w", err)
	}

	if added {
		fmt.Printf("Added %s to %s\n", qualifiedAtom, targetFile)
	} else {
		fmt.Printf("Rule %s already exists in %s\n", qualifiedAtom, targetFile)
	}
	return nil
}

func (cfg *MainArgConfig) cmdConfOverlayRepoMaskReset(repo, configRoot, rcPath, atomArg string) error {
	repoInfo, err := g2.ResolveRepo(repo, rcPath)
	if err != nil {
		return err
	}

	qualifiedAtom, err := g2.QualifyAtomForRepo(atomArg, repoInfo.RepoName)
	if err != nil {
		return err
	}

	maskPath := filepath.Join(configRoot, "package.mask")
	removedCount, err := g2.RemoveUserConfigAtom(maskPath, qualifiedAtom)
	if err != nil {
		return fmt.Errorf("resetting mask: %w", err)
	}

	if removedCount > 0 {
		fmt.Printf("Removed %s from %s\n", qualifiedAtom, maskPath)
	} else {
		fmt.Printf("No matching mask rule for %s in %s\n", qualifiedAtom, maskPath)
	}
	return nil
}

func (cfg *MainArgConfig) cmdConfOverlayRepoUnmaskReset(repo, configRoot, rcPath, atomArg string) error {
	repoInfo, err := g2.ResolveRepo(repo, rcPath)
	if err != nil {
		return err
	}

	qualifiedAtom, err := g2.QualifyAtomForRepo(atomArg, repoInfo.RepoName)
	if err != nil {
		return err
	}

	unmaskPath := filepath.Join(configRoot, "package.unmask")
	removedCount, err := g2.RemoveUserConfigAtom(unmaskPath, qualifiedAtom)
	if err != nil {
		return fmt.Errorf("resetting unmask: %w", err)
	}

	if removedCount > 0 {
		fmt.Printf("Removed %s from %s\n", qualifiedAtom, unmaskPath)
	} else {
		fmt.Printf("No matching unmask rule for %s in %s\n", qualifiedAtom, unmaskPath)
	}
	return nil
}
