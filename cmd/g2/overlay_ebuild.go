package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdOverlayEbuild(args []string) error {
	fs := flag.NewFlagSet("overlay ebuild", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 overlay ebuild <subcommand>\n")
		fmt.Printf("\tmove <from> <to>\t\tRecord a package move in profiles/updates\n")
		fmt.Printf("\tslotmove <package> <from> <to>\tRecord a slot move in profiles/updates\n")
		fmt.Printf("\tinstall [flags] <ebuild.ebuild> [overlay path if not .] -- [FILES..]\tInstall an ebuild into an overlay\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand for overlay ebuild")
	}

	subcmd := fs.Arg(0)
	switch subcmd {
	case "move":
		return cfg.cmdOverlayEbuildMove(fs.Args()[1:])
	case "slotmove":
		return cfg.cmdOverlayEbuildSlotmove(fs.Args()[1:])
	case "install":
		return cfg.cmdOverlayEbuildInstall(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown overlay ebuild subcommand: %s", subcmd)
	}
}

func getQuarterFile() string {
	now := time.Now()
	quarter := (now.Month()-1)/3 + 1
	year := now.Year()
	return fmt.Sprintf("%dQ-%d", quarter, year)
}

func appendUpdateFile(newMove *g2.PackageMove, newSlotMove *g2.PackageSlotMove) error {
	quarterFile := getQuarterFile()
	updatesDir := filepath.Join("profiles", "updates")

	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		return fmt.Errorf("creating updates dir: %w", err)
	}

	updatesPath := filepath.Join(updatesDir, quarterFile)

	update := &g2.PackageUpdate{}

	if _, err := os.Stat(updatesPath); err == nil {
		// Parse existing updates in the current quarter file
		parsed, err := g2.ParseUpdatesDir(updatesDir)
		if err == nil && parsed != nil {
			update.Moves = parsed.Moves
			update.SlotMoves = parsed.SlotMoves
		}
	}

	if newMove != nil {
		update.Moves = append(update.Moves, *newMove)
		log.Printf("Recorded move %s -> %s in %s", newMove.Old, newMove.New, updatesPath)
	}

	if newSlotMove != nil {
		update.SlotMoves = append(update.SlotMoves, *newSlotMove)
		log.Printf("Recorded slotmove for %s: %s -> %s in %s", newSlotMove.Package, newSlotMove.Old, newSlotMove.New, updatesPath)
	}

	return g2.WriteUpdatesFile(updatesPath, update)
}

func (cfg *MainArgConfig) cmdOverlayEbuildMove(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: g2 overlay ebuild move <from> <to>")
	}

	oldPkg := args[0]
	newPkg := args[1]

	return appendUpdateFile(&g2.PackageMove{Old: oldPkg, New: newPkg}, nil)
}

func (cfg *MainArgConfig) cmdOverlayEbuildSlotmove(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: g2 overlay ebuild slotmove <package> <from> <to>")
	}

	pkg := args[0]
	oldSlot := args[1]
	newSlot := args[2]

	return appendUpdateFile(nil, &g2.PackageSlotMove{Package: pkg, Old: oldSlot, New: newSlot})
}

func (cfg *MainArgConfig) cmdOverlayEbuildInstall(args []string) error {
	fs := flag.NewFlagSet("overlay ebuild install", flag.ExitOnError)
	categoryFlag := fs.String("category", "", "The category for the ebuild")
	if err := fs.Parse(args); err != nil {
		return err
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

	if len(positionalArgs) < 1 {
		return fmt.Errorf("usage: g2 overlay ebuild install [flags] <ebuild.ebuild> [overlay path if not .] -- [FILES..]")
	}

	ebuildPath := positionalArgs[0]
	overlayPath := "."
	if len(positionalArgs) > 1 {
		overlayPath = positionalArgs[1]
	}

	absEbuildPath, err := filepath.Abs(ebuildPath)
	if err != nil {
		return fmt.Errorf("resolving ebuild path: %w", err)
	}

	ebuild, err := g2.ParseEbuild(os.DirFS(filepath.Dir(absEbuildPath)), filepath.Base(absEbuildPath), g2.ParseFull)
	if err != nil {
		return fmt.Errorf("parsing ebuild %s: %w", absEbuildPath, err)
	}

	vars := g2.ParseEbuildVariables(filepath.Base(absEbuildPath))
	if vars == nil {
		return fmt.Errorf("could not parse PN and PV from ebuild filename %s", filepath.Base(absEbuildPath))
	}
	pn := vars["PN"]
	pv := vars["PV"]

	category := *categoryFlag
	if category == "" {
		if c, ok := ebuild.Vars["CATEGORY"]; ok && c != "" {
			category = c
		} else {
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

	targetPkgDir := filepath.Join(overlayPath, category, pn)
	if err := os.MkdirAll(targetPkgDir, 0755); err != nil {
		return fmt.Errorf("creating target package directory: %w", err)
	}

	targetEbuildPath := filepath.Join(targetPkgDir, fmt.Sprintf("%s-%s.ebuild", pn, pv))
	if err := copyFile(absEbuildPath, targetEbuildPath); err != nil {
		return fmt.Errorf("copying ebuild: %w", err)
	}

	if len(fileArgs) > 0 {
		filesDir := filepath.Join(targetPkgDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			return fmt.Errorf("creating files directory: %w", err)
		}

		for _, f := range fileArgs {
			dest := filepath.Join(filesDir, filepath.Base(f))
			if err := copyFile(f, dest); err != nil {
				return fmt.Errorf("copying file %s: %w", f, err)
			}
		}
	}

	manifestCfg := &CmdManifestArgConfig{MainArgConfig: cfg}
	if err := manifestCfg.cmdVerify([]string{"-fix", targetPkgDir}, g2.AllHashes); err != nil {
		log.Printf("Warning: updating manifest: %v", err)
	}

	if err := cfg.cmdUseDiscover([]string{overlayPath}); err != nil {
		log.Printf("Warning: updating use.desc/use.local.desc: %v", err)
	}

	if err := cfg.cmdPkgDescIndexGenerate([]string{overlayPath}); err != nil {
		log.Printf("Warning: generating pkg_desc_index: %v", err)
	}

	if err := cfg.cmdCacheGenerate([]string{overlayPath}); err != nil {
		log.Printf("Warning: generating md5-cache: %v", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destination.Close() }()

	_, err = io.Copy(destination, source)
	return err
}
