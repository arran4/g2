package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdOverlayEbuild(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing subcommand for overlay ebuild (e.g., install)")
	}
	subcmd := args[0]
	if subcmd != "install" {
		return fmt.Errorf("unknown overlay ebuild subcommand: %s", subcmd)
	}

	return cfg.cmdOverlayEbuildInstall(args[1:])
}

func (cfg *MainArgConfig) cmdOverlayEbuildInstall(args []string) error {
	// Custom parsing for `g2 overlay ebuild install <ebuild.ebuild> [overlay path if not .] -- [FILES..]`
	if len(args) < 1 {
		return fmt.Errorf("usage: g2 overlay ebuild install <ebuild.ebuild> [overlay path if not .] [-- FILES..]")
	}

	ebuildFile := args[0]
	overlayPath := "."
	var files []string

	argsAfterEbuild := args[1:]
	dashDashIndex := -1

	for i, arg := range argsAfterEbuild {
		if arg == "--" {
			dashDashIndex = i
			break
		}
	}

	if dashDashIndex == -1 {
		if len(argsAfterEbuild) > 0 {
			overlayPath = argsAfterEbuild[0]
		}
	} else {
		if dashDashIndex > 0 {
			overlayPath = argsAfterEbuild[0]
		}
		files = argsAfterEbuild[dashDashIndex+1:]
	}

	log.Printf("Installing %s into overlay %s", ebuildFile, overlayPath)

	// Determine category and package name from the ebuild file name or path
	vars := g2.ParseEbuildVariables(ebuildFile)
	if vars == nil {
		return fmt.Errorf("could not parse variables from ebuild filename %s", ebuildFile)
	}

	pn := vars["PN"]

	// How to figure out category?
	// It's usually the parent directory if it's currently in a category/package structure.
	// We can read it from the ebuild variables if someone put CATEGORY= in it? (unlikely).
	// We can try to deduce from its current absolute path
	absEbuildFile, err := filepath.Abs(ebuildFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of ebuild file: %w", err)
	}

	category := ""

	// Parse ebuild content
	dir := filepath.Dir(absEbuildFile)
	base := filepath.Base(absEbuildFile)
	ebuild, err := g2.ParseEbuild(os.DirFS(dir), base, g2.ParseVariables)
	if err == nil && ebuild.Vars["CATEGORY"] != "" {
		category = ebuild.Vars["CATEGORY"]
	}

	if category == "" {
		// Try from path structure: path/to/category/package/package-1.0.ebuild
		pkgDir := filepath.Dir(absEbuildFile)
		categoryDir := filepath.Dir(pkgDir)
		category = filepath.Base(categoryDir)

		// If it's a valid gentoo category, use it. But let's assume it's right.
	}

	if category == "." || category == "/" || category == "" {
		return fmt.Errorf("could not determine category for ebuild")
	}

	targetDir := filepath.Join(overlayPath, category, pn)

	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	targetEbuild := filepath.Join(targetDir, filepath.Base(ebuildFile))
	absTargetEbuild, err := filepath.Abs(targetEbuild)
	if err != nil {
		return fmt.Errorf("failed to resolve target path %s: %w", targetEbuild, err)
	}

	// Copy ebuild if source != target
	if absEbuildFile != absTargetEbuild {
		source, err := os.Open(absEbuildFile)
		if err != nil {
			return fmt.Errorf("failed to open source ebuild: %w", err)
		}
		defer source.Close()

		dest, err := os.Create(targetEbuild)
		if err != nil {
			return fmt.Errorf("failed to create target ebuild: %w", err)
		}
		defer dest.Close()

		_, err = io.Copy(dest, source)
		if err != nil {
			return fmt.Errorf("failed to copy ebuild: %w", err)
		}
	}

	// Copy files
	if len(files) > 0 {
		filesDir := filepath.Join(targetDir, "files")
		err = os.MkdirAll(filesDir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create files directory: %w", err)
		}

		for _, file := range files {
			absFile, err := filepath.Abs(file)
			if err != nil {
				return fmt.Errorf("failed to resolve file path %s: %w", file, err)
			}
			targetFile := filepath.Join(filesDir, filepath.Base(absFile))
			absTargetFile, err := filepath.Abs(targetFile)
			if err != nil {
				return fmt.Errorf("failed to resolve target path %s: %w", targetFile, err)
			}
			if absFile != absTargetFile {
				sf, err := os.Open(absFile)
				if err != nil {
					return fmt.Errorf("failed to open file %s: %w", file, err)
				}
				df, err := os.Create(targetFile)
				if err != nil {
					sf.Close()
					return fmt.Errorf("failed to create file %s: %w", targetFile, err)
				}
				_, err = io.Copy(df, sf)
				sf.Close()
				df.Close()
				if err != nil {
					return fmt.Errorf("failed to copy file %s: %w", file, err)
				}
			}
		}
	}

	// Update manifest
	manifestCmd := &CmdManifestArgConfig{
		MainArgConfig: cfg,
	}

	// Re-parse the target dir for ebuilds, since we moved it
	hashes := []string{g2.HashBlake2b, g2.HashBlake2s, g2.HashMd5, g2.HashRmd160, g2.HashSha1, g2.HashSha256, g2.HashSha3_256, g2.HashSha3_512, g2.HashSha512}
	log.Printf("Updating manifest in %s", targetDir)

	// Call verify on targetDir with fix=true
	err = manifestCmd.cmdVerify([]string{"-fix", targetDir}, hashes)
	if err != nil {
		return fmt.Errorf("failed to update manifest: %w", err)
	}

	// Update use.desc
	log.Printf("Discovering USE flags in overlay %s", overlayPath)
	err = cfg.cmdUseDiscover([]string{overlayPath})
	if err != nil {
		return fmt.Errorf("failed to update use.desc: %w", err)
	}

	// Update pkg_desc_index
	log.Printf("Generating pkg_desc_index in overlay %s", overlayPath)
	err = cfg.cmdPkgDescIndexGenerate([]string{overlayPath})
	if err != nil {
		return fmt.Errorf("failed to update pkg_desc_index: %w", err)
	}

	return nil
}
