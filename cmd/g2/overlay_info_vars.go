package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdOverlayInfoVars(args []string) error {
	fs := flag.NewFlagSet("overlay info-vars", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 overlay info-vars <subcommand>\n")
		fmt.Printf("\tlist [overlay path if not .]\tList current info variables\n")
		fmt.Printf("\tadd <var> [overlay path if not .]\tAdd an info variable\n")
		fmt.Printf("\tremove <var> [overlay path if not .]\tRemove an info variable\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand for overlay info-vars")
	}

	subcmd := fs.Arg(0)
	switch subcmd {
	case "list":
		return cfg.cmdOverlayInfoVarsList(fs.Args()[1:])
	case "add":
		return cfg.cmdOverlayInfoVarsAdd(fs.Args()[1:])
	case "remove":
		return cfg.cmdOverlayInfoVarsRemove(fs.Args()[1:])
	default:
		fs.Usage()
		return fmt.Errorf("unknown overlay info-vars subcommand: %s", subcmd)
	}
}

func (cfg *MainArgConfig) cmdOverlayInfoVarsList(args []string) error {
	overlayPath := "."
	if len(args) > 0 {
		overlayPath = args[0]
	}

	infoVarsPath := filepath.Join(overlayPath, "profiles", "info_vars")
	vars, err := g2.ParseInfoVars(infoVarsPath)
	if err != nil {
		return fmt.Errorf("parsing info_vars: %w", err)
	}

	for _, v := range vars {
		fmt.Println(v)
	}
	return nil
}

func (cfg *MainArgConfig) cmdOverlayInfoVarsAdd(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: g2 overlay info-vars add <var> [overlay path if not .]")
	}

	varName := strings.TrimSpace(args[0])
	overlayPath := "."
	if len(args) > 1 {
		overlayPath = args[1]
	}

	infoVarsPath := filepath.Join(overlayPath, "profiles", "info_vars")
	vars, err := g2.ParseInfoVars(infoVarsPath)
	if err != nil {
		return fmt.Errorf("parsing info_vars: %w", err)
	}

	if slices.Contains(vars, varName) {
		fmt.Printf("Variable %s is already in info_vars\n", varName)
		return nil
	}

	vars = append(vars, varName)
	slices.Sort(vars)

	if err := g2.WriteInfoVarsFile(infoVarsPath, vars); err != nil {
		return fmt.Errorf("writing info_vars: %w", err)
	}

	fmt.Printf("Added %s to info_vars\n", varName)
	return nil
}

func (cfg *MainArgConfig) cmdOverlayInfoVarsRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: g2 overlay info-vars remove <var> [overlay path if not .]")
	}

	varName := strings.TrimSpace(args[0])
	overlayPath := "."
	if len(args) > 1 {
		overlayPath = args[1]
	}

	infoVarsPath := filepath.Join(overlayPath, "profiles", "info_vars")
	vars, err := g2.ParseInfoVars(infoVarsPath)
	if err != nil {
		return fmt.Errorf("parsing info_vars: %w", err)
	}

	idx := slices.Index(vars, varName)
	if idx == -1 {
		fmt.Printf("Variable %s is not in info_vars\n", varName)
		return nil
	}

	vars = slices.Delete(vars, idx, idx+1)

	if err := g2.WriteInfoVarsFile(infoVarsPath, vars); err != nil {
		return fmt.Errorf("writing info_vars: %w", err)
	}

	fmt.Printf("Removed %s from info_vars\n", varName)
	return nil
}
