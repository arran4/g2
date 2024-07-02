package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type MainArgConfig struct {
	Args []string
}

func main() {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	cfg := &MainArgConfig{
		Args: []string{os.Args[0]},
	}
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "manifest", "commands relating to Manifest files")
	}
	if err := fs.Parse(os.Args); err != nil {
		log.Printf("Flag parse error: %s", err)
		os.Exit(-1)
		return
	}
	cmd := "help"
	if fs.NArg() <= 1 {
		log.Printf("Please specify an argument, try -help for help")
		os.Exit(-1)
		return
	} else {
		cmd = fs.Arg(1)
	}
	cfg.Args = append(cfg.Args, cmd)
	switch cmd {
	case "manifest":
		if err := cfg.cmdManifest(fs.Args()[2:]); err != nil {
			log.Printf("generate error: %s", err)
			os.Exit(-1)
			return
		}
	default:
		fmt.Printf("Unknown command %s", cmd)
		fs.Usage()
		return
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(-1)
	}
}

type CmdManifestArgConfig struct {
	*MainArgConfig
}

func (cfg *MainArgConfig) cmdManifest(args []string) error {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "upsert-from-url", "To update or insert Manifest entries streamed from a URL")
	}
	config := &CmdManifestArgConfig{
		MainArgConfig: cfg,
	}
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)
	switch cmd {
	case "upsert-from-url":
		if err := config.cmdUpsertFromUrl(fs.Args()[1:]); err != nil {
			return fmt.Errorf("updsert file from url: %w", err)
		}
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(-1)
	}
	return nil
}

func (cfg *CmdManifestArgConfig) cmdUpsertFromUrl(args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: upsert-from-url <url> <filename> <manifestFileOrDir>")
	}

	url := args[0]
	filename := args[1]
	ebuildDirOrFile := args[2]

	size, blake2bSum, sha512Sum, err := g2.DownloadAndChecksum(url)
	if err != nil {
		return fmt.Errorf("downloading and calculating checksums: %v\n", err)
	}

	manifestLine := fmt.Sprintf("DIST %s %d BLAKE2B %s SHA512 %s", filename, size, blake2bSum, sha512Sum)
	manifestPath := ebuildDirOrFile
	if _, file := filepath.Split(manifestPath); file != "Manifest" {
		manifestPath = filepath.Join(ebuildDirOrFile, "Manifest")
	}

	err = g2.UpsertManifest(manifestPath, filename, manifestLine)
	if err != nil {
		return fmt.Errorf("updating manifest: %v\n", err)
	}

	log.Printf("Done")
	return nil
}
