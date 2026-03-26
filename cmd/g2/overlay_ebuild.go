package main

import (
	"flag"
	"fmt"
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
