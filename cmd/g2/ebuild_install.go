package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
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

	repoDir := *repoFlag
	wfs := NewOSFS("")

	return cfg.InstallEbuild(wfs, targets, repoDir, os.Stdin)
}

func (cfg *CmdEbuildArgConfig) InstallEbuild(wfs WritableFS, targets []string, repoDir string, stdin io.Reader) error {
	target := targets[0]

	var content []byte
	var ebuildName string

	if target == "-" {
		var err error
		content, err = io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}

		if len(targets) < 2 {
			return fmt.Errorf("when using stdin, provide the ebuild filename as the second argument")
		}
		ebuildName = filepath.Base(targets[1])
	} else {
		ebuildName = filepath.Base(target)
		var err error
		// Use os.ReadFile for standard file if it's absolute, otherwise join. But actually wfs handles it.
		// Wait, WritableFS might not have ReadFile, we only put WriteFile.
		// Actually fs.ReadFile(wfs, target) works.
		content, err = fs.ReadFile(wfs, filepath.ToSlash(target))
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
	// Since wfs is the filesystem, we can just write it to a temp dir and parse.
	// Since wfs doesn't have MkdirTemp, we'll just write it directly to the target dir,
	// but we don't know the category yet.
	// We'll write it to repoDir/tmp-install-xyz.
	tmpDir := filepath.Join(repoDir, "tmp-install")
	if err := wfs.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = wfs.RemoveAll(tmpDir) }()

	tmpFile := filepath.Join(tmpDir, ebuildName)
	if err := wfs.WriteFile(tmpFile, content, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// We can use os.DirFS if we are using OSFS, or use wfs directly for g2.ParseEbuild if we wrap it.
	// Wait, g2.ParseEbuild takes an fs.FS and a string. `wfs` is an `fs.FS`! We can just pass `wfs` and the relative path, but ParseEbuild looks in the root of the fs.
	// But it actually calls `fs.Open(filepath.Join(".", filename))` internally or similar.
	// Let's use `fs.Sub` to get the dir.
	subFs, err := fs.Sub(wfs, filepath.ToSlash(tmpDir))
	if err != nil {
		// fallback to os.DirFS
		subFs = os.DirFS(tmpDir)
	}

	ebuild, err := g2.ParseEbuild(subFs, ebuildName, g2.ParseVariables)
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
	if err := wfs.MkdirAll(targetPkgDir, 0755); err != nil {
		return fmt.Errorf("creating target package directory: %w", err)
	}

	targetEbuildPath := filepath.Join(targetPkgDir, ebuildName)
	if err := wfs.WriteFile(targetEbuildPath, content, 0644); err != nil {
		return fmt.Errorf("writing ebuild: %w", err)
	}

	log.Printf("Installed %s to %s", ebuildName, targetEbuildPath)

	manifestCfg := &CmdManifestArgConfig{MainArgConfig: cfg.MainArgConfig}
	if err := manifestCfg.cmdVerify([]string{"-fix", targetPkgDir}, g2.AllHashes); err != nil {
		log.Printf("Warning: updating manifest: %v", err)
	}

	if err := cfg.cmdUseDiscover([]string{repoDir}); err != nil {
		log.Printf("Warning: updating use.desc/use.local.desc: %v", err)
	}

	if err := cfg.cmdPkgDescIndexGenerate([]string{repoDir}); err != nil {
		log.Printf("Warning: generating pkg_desc_index: %v", err)
	}

	if err := cfg.cmdCacheGenerate([]string{repoDir}); err != nil {
		log.Printf("Warning: generating md5-cache: %v", err)
	}

	return nil
}
