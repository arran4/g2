package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdEbuildInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	repoFlag := fs.String("repo", ".", "Repository directory (default: current directory)")
	categoryFlag := fs.String("category", "", "The category for the ebuild (optional, auto-detected if possible)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: g2 ebuild install --repo <repo name / dir> <ebuild fn | - for stdin> [-- [FILES..]]")
	}

	var positionalArgs []string
	var fileArgs []string
	foundDashDash := false

	fsArgs := fs.Args()
	for i, arg := range fsArgs {
		if arg == "--" {
			positionalArgs = fsArgs[:i]
			fileArgs = fsArgs[i+1:]
			foundDashDash = true
			break
		}
	}

	if !foundDashDash {
		positionalArgs = fsArgs
	}

	if len(positionalArgs) == 0 {
		return fmt.Errorf("usage: g2 ebuild install --repo <repo name / dir> <ebuild fn | - for stdin> [-- [FILES..]]")
	}

	ebuildPath := positionalArgs[0]
	repoPath := *repoFlag

	var inputContent []byte
	var readErr error
	if ebuildPath == "-" {
		inputContent, readErr = io.ReadAll(os.Stdin)
	} else {
		inputContent, readErr = os.ReadFile(ebuildPath)
	}

	if readErr != nil {
		return fmt.Errorf("failed to read input: %w", readErr)
	}

	vars := g2.ParseEbuildVariablesFromReader(bytes.NewReader(inputContent))
	if vars == nil || vars["PN"] == "" || vars["PV"] == "" {
		if ebuildPath != "-" {
			vars = g2.ParseEbuildVariables(filepath.Base(ebuildPath))
		}
		if vars == nil || vars["PN"] == "" || vars["PV"] == "" {
			return fmt.Errorf("could not parse PN and PV from ebuild")
		}
	}

	pn := vars["PN"]
	pv := vars["PV"]

	// ebuild is missing EAPI / doesn't get fully parsed by ParseEbuildVariablesFromReader, so we can try to parse it via parser to get CATEGORY if it's there
	var ebuildParsed *g2.Ebuild
	var err error
	if ebuildPath != "-" {
		ebuildParsed, err = g2.ParseEbuild(os.DirFS(filepath.Dir(ebuildPath)), filepath.Base(ebuildPath), g2.ParseVariables)
	}
	err = nil

	category := *categoryFlag
	if category == "" && err == nil && ebuildParsed != nil {
		if c, ok := ebuildParsed.Vars["CATEGORY"]; ok && c != "" {
			category = c
		}
	}

	if category == "" && ebuildPath != "-" {
		absEbuildPath, err := filepath.Abs(ebuildPath)
		if err == nil {
			dir1 := filepath.Dir(absEbuildPath)
			dir2 := filepath.Dir(dir1)
			catInfer := filepath.Base(dir2)
			if catInfer != "." && catInfer != "/" {
				category = catInfer
			}
		}
	}

	if category == "" {
		return fmt.Errorf("could not determine category for ebuild, please specify with -category")
	}

	targetPkgDir := filepath.Join(repoPath, category, pn)
	if err := os.MkdirAll(targetPkgDir, 0755); err != nil {
		return fmt.Errorf("creating target package directory: %w", err)
	}

	targetEbuildPath := filepath.Join(targetPkgDir, fmt.Sprintf("%s-%s.ebuild", pn, pv))
	if err := os.WriteFile(targetEbuildPath, inputContent, 0644); err != nil {
		return fmt.Errorf("writing ebuild: %w", err)
	}

	if len(fileArgs) > 0 {
		filesDir := filepath.Join(targetPkgDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			return fmt.Errorf("creating files directory: %w", err)
		}

		for _, f := range fileArgs {
			dest := filepath.Join(filesDir, filepath.Base(f))
			if err := func() error {
				sourceFileStat, err := os.Stat(f)
				if err != nil {
					return err
				}
				if !sourceFileStat.Mode().IsRegular() {
					return fmt.Errorf("%s is not a regular file", f)
				}
				source, err := os.Open(f)
				if err != nil {
					return err
				}
				defer source.Close()
				destination, err := os.Create(dest)
				if err != nil {
					return err
				}
				defer destination.Close()
				_, err = io.Copy(destination, source)
				return err
			}(); err != nil {
				return fmt.Errorf("copying file %s: %w", f, err)
			}
		}
	}

	manifestCfg := &CmdManifestArgConfig{MainArgConfig: cfg}
	if err := manifestCfg.cmdVerify([]string{"-fix", targetPkgDir}, g2.AllHashes); err != nil {
		log.Printf("Warning: updating manifest: %v", err)
	}

	if err := cfg.cmdUseDiscover([]string{repoPath}); err != nil {
		log.Printf("Warning: updating use.desc/use.local.desc: %v", err)
	}

	if err := cfg.cmdPkgDescIndexGenerate([]string{repoPath}); err != nil {
		log.Printf("Warning: generating pkg_desc_index: %v", err)
	}

	if err := cfg.cmdCacheGenerate([]string{repoPath}); err != nil {
		log.Printf("Warning: generating md5-cache: %v", err)
	}

	fmt.Printf("Installed %s to %s\n", filepath.Base(targetEbuildPath), targetPkgDir)

	return nil
}
