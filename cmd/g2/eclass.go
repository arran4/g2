package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

type CmdEclassArgConfig struct {
	*MainArgConfig
}

func (cfg *MainArgConfig) cmdEclass(args []string) error {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "list", "List available eclasses")
		fmt.Printf("\t\t %s \t\t %s\n", "install", "Install an eclass from gentoo stable")
		fmt.Printf("\t\t %s \t\t %s\n", "explain", "Human-readable summary output of an eclass")
		fmt.Printf("\t\t %s \t\t %s\n", "remove", "Remove an eclass")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	config := &CmdEclassArgConfig{
		MainArgConfig: cfg,
	}

	switch cmd {
	case "list":
		return config.cmdEclassList(fs.Args()[1:])
	case "install":
		return config.cmdEclassInstall(fs.Args()[1:])
	case "explain":
		return config.cmdEclassExplain(fs.Args()[1:])
	case "remove":
		return config.cmdEclassRemove(fs.Args()[1:])
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(-1)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}

	return nil
}

func (cfg *CmdEclassArgConfig) cmdEclassList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	dir := "eclass"
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("eclass directory not found")
			return nil
		}
		return fmt.Errorf("reading eclass dir: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".eclass") {
			fmt.Println(e.Name())
		}
	}

	return nil
}

func (cfg *CmdEclassArgConfig) cmdEclassInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: g2 eclass install <eclass-name>")
	}

	eclassName := fs.Arg(0)
	if !strings.HasSuffix(eclassName, ".eclass") {
		eclassName += ".eclass"
	}

	dir := "eclass"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating eclass dir: %w", err)
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/gentoo-mirror/gentoo/stable/eclass/%s", eclassName)
	destPath := filepath.Join(dir, eclassName)

	log.Printf("Downloading %s from %s", eclassName, url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading eclass: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download eclass, HTTP status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating eclass file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("writing eclass file: %w", err)
	}

	log.Printf("Successfully installed %s to %s", eclassName, destPath)
	return nil
}

func (cfg *CmdEclassArgConfig) cmdEclassExplain(args []string) error {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: g2 eclass explain <file.eclass>")
	}

	filename := fs.Arg(0)
	ebuild, err := g2.ParseEbuild(os.DirFS(filepath.Dir(filename)), filepath.Base(filename), g2.ParseFull)
	if err != nil {
		return fmt.Errorf("parsing eclass: %w", err)
	}

	fmt.Printf("Eclass: %s\n", filepath.Base(filename))
	fmt.Println(strings.Repeat("-", len(filepath.Base(filename))+8))

	fmt.Println("\nVariables:")
	for k, v := range ebuild.Vars {
		fmt.Printf("  %s=\"%s\"\n", k, v)
	}

	fmt.Println("\nFunctions:")
	for k, v := range ebuild.Functions {
		fmt.Printf("  %s() {\n", k)
		lines := strings.Split(v.Value, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				fmt.Printf("    %s\n", line)
			}
		}
		fmt.Printf("  }\n")
	}

	if len(ebuild.ParseWarnings) > 0 {
		fmt.Println("\nParse Warnings:")
		for _, w := range ebuild.ParseWarnings {
			fmt.Printf("  %s\n", w)
		}
	}

	return nil
}

func (cfg *CmdEclassArgConfig) cmdEclassRemove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: g2 eclass remove <eclass-name>")
	}

	eclassName := fs.Arg(0)
	if !strings.HasSuffix(eclassName, ".eclass") {
		eclassName += ".eclass"
	}

	destPath := filepath.Join("eclass", eclassName)
	if err := os.Remove(destPath); err != nil {
		if os.IsNotExist(err) {
			log.Printf("Eclass %s not found in eclass directory", eclassName)
			return nil
		}
		return fmt.Errorf("removing eclass: %w", err)
	}

	log.Printf("Successfully removed %s", destPath)
	return nil
}
