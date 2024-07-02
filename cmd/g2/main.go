package main

import (
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"log"
	"os"
	"path/filepath"
)

type MainArgConfig struct {
}

func main() {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	config := &MainArgConfig{}
	if err := fs.Parse(os.Args); err != nil {
		log.Printf("Flag parse error: %s", err)
		os.Exit(-1)
		return
	}
	if fs.NArg() <= 1 {
		log.Printf("Please specify an argument, try -help for help")
		os.Exit(-1)
		return
	}
	switch fs.Arg(1) {
	case "manifest":
		if err := config.cmdManifest(fs.Args()[2:]); err != nil {
			log.Printf("generate error: %s", err)
			os.Exit(-1)
			return
		}
	default:
		log.Printf("Unknown command %s", fs.Arg(1))
		log.Printf("Try %s for %s", "manifest", "commands relating to Manifest files")
		os.Exit(-1)
	}
}

type CmdManifestArgConfig struct {
	*MainArgConfig
}

func (cfg *MainArgConfig) cmdManifest(args []string) error {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	config := &CmdManifestArgConfig{
		MainArgConfig: cfg,
	}
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	switch fs.Arg(0) {
	case "upsert-file-from-url":
		if err := config.cmdUpsertFileFromUrl(fs.Args()[1:]); err != nil {
			return fmt.Errorf("updsert file from url: %w", err)
		}
	default:
		log.Printf("Unknown command %s", fs.Arg(0))
		log.Printf("Try %s for %s", "upsert-file-from-url", "To update or insert Manifest entries streamed from a URL")
		os.Exit(-1)
	}
	return nil
}

func (cfg *CmdManifestArgConfig) cmdUpsertFileFromUrl(args []string) error {
	if len(args) != 4 {
		return fmt.Errorf("usage: go run main.go <url> <filename> <Manifest file or dir>")
	}

	url := args[1]
	filename := args[2]
	ebuildDirOrFile := args[3]

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
