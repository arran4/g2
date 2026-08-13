package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *CmdEbuildArgConfig) cmdEbuildInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	repoFlag := fs.String("repo", ".", "Repo name or dir")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	targets := fs.Args()
	if len(targets) == 0 {
		return fmt.Errorf("usage: g2 ebuild install --repo <repo name / dir> <ebuild fn | - for stdin>")
	}

	target := targets[0]
	repoDir := *repoFlag

	var content []byte
	var ebuildName string

	if target == "-" {
		var err error
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		// If read from stdin, we need some way to know the filename.
		// Usually install ebuild has to know the name, we could try to parse it from the content but ebuilds don't contain their own name usually.
		// If the user uses `-`, they would need to provide an explicit filename, or we parse from stdin content somehow...
		// However, standard portage doesn't have an exact `ebuild install -` but we can write it to a temp file and parse,
		// but we still wouldn't know the package name.
		// Wait, if it's `-`, we can't easily guess. The prompt just says `<ebuild fn | - for stdin>`.
		// Let's assume there is a second argument for filename if stdin, or we could parse variables like `PN` and `PV`? But those are often not in the file, but implied from the filename.

		// If they don't provide a name, it's an error.
		if len(targets) < 2 {
			return fmt.Errorf("when using stdin, provide the ebuild filename as the second argument")
		}
		ebuildName = filepath.Base(targets[1])
	} else {
		ebuildName = filepath.Base(target)
		var err error
		content, err = os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", target, err)
		}
	}

	if !strings.HasSuffix(ebuildName, ".ebuild") {
		return fmt.Errorf("filename must end in .ebuild")
	}

	vars := g2.ParseEbuildVariables(ebuildName)
	if vars == nil {
		return fmt.Errorf("could not parse PN and PV from ebuild filename %s", ebuildName)
	}

	pn := vars["PN"]

	// Now we need CATEGORY. We can parse the ebuild to see if it sets CATEGORY.
	tmpDir, err := os.MkdirTemp("", "g2-install-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, ebuildName)
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	ebuild, err := g2.ParseEbuild(os.DirFS(tmpDir), ebuildName, g2.ParseVariables)
	if err != nil {
		return fmt.Errorf("parsing ebuild: %w", err)
	}

	category := ebuild.Vars["CATEGORY"]
	if category == "" {
		// Try to infer from current directory if we are installing from a file
		if target != "-" {
			absTarget, err := filepath.Abs(target)
			if err == nil {
				dir1 := filepath.Dir(absTarget)
				dir2 := filepath.Dir(dir1)
				catInfer := filepath.Base(dir2)
				if catInfer != "." && catInfer != "/" {
					category = catInfer
				}
			}
		}
		if category == "" {
			return fmt.Errorf("could not determine category for ebuild, and it's not set in the ebuild")
		}
	}

	targetPkgDir := filepath.Join(repoDir, category, pn)
	if err := os.MkdirAll(targetPkgDir, 0755); err != nil {
		return fmt.Errorf("creating target package directory: %w", err)
	}

	targetEbuildPath := filepath.Join(targetPkgDir, ebuildName)
	if err := os.WriteFile(targetEbuildPath, content, 0644); err != nil {
		return fmt.Errorf("writing ebuild: %w", err)
	}

	log.Printf("Installed %s to %s", ebuildName, targetEbuildPath)

	manifestCfg := &CmdManifestArgConfig{MainArgConfig: cfg.MainArgConfig}
	if err := manifestCfg.cmdVerify([]string{"-fix", targetPkgDir}, g2.AllHashes); err != nil {
		log.Printf("Warning: updating manifest: %v", err)
	}

	if err := cfg.MainArgConfig.cmdUseDiscover([]string{repoDir}); err != nil {
		log.Printf("Warning: updating use.desc/use.local.desc: %v", err)
	}

	if err := cfg.MainArgConfig.cmdPkgDescIndexGenerate([]string{repoDir}); err != nil {
		log.Printf("Warning: generating pkg_desc_index: %v", err)
	}

	if err := cfg.MainArgConfig.cmdCacheGenerate([]string{repoDir}); err != nil {
		log.Printf("Warning: generating md5-cache: %v", err)
	}

	return nil
}
