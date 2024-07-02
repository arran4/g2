package main

import (
	"crypto/sha512"
	"flag"
	"fmt"
	"golang.org/x/crypto/blake2b"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	case "generate":
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

	size, blake2bSum, sha512Sum, err := downloadAndChecksum(url)
	if err != nil {
		return fmt.Errorf("downloading and calculating checksums: %v\n", err)
	}

	manifestLine := fmt.Sprintf("DIST %s %d BLAKE2B %s SHA512 %s", filename, size, blake2bSum, sha512Sum)
	manifestPath := ebuildDirOrFile
	if _, file := filepath.Split(manifestPath); file != "Manifest" {
		manifestPath = filepath.Join(ebuildDirOrFile, "Manifest")
	}

	err = upsertManifest(manifestPath, filename, manifestLine)
	if err != nil {
		return fmt.Errorf("updating manifest: %v\n", err)
	}

	log.Printf("Done")
	return nil
}

func downloadAndChecksum(url string) (int64, string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", "", fmt.Errorf("bad status: %s", resp.Status)
	}

	blake2bHash, err := blake2b.New512(nil)
	if err != nil {
		return 0, "", "", fmt.Errorf("bad blake2b initialization: %w", err)
	}
	sha512Hash := sha512.New()

	multiWriter := io.MultiWriter(blake2bHash, sha512Hash)

	sizeCh := make(chan int64)
	errCh := make(chan error)

	go func() {
		defer close(sizeCh)
		defer close(errCh)

		size, err := io.Copy(multiWriter, resp.Body)
		if err != nil {
			errCh <- err
			return
		}

		sizeCh <- size
	}()

	size := <-sizeCh
	err = <-errCh
	if err != nil {
		return 0, "", "", err
	}

	blake2bSum := fmt.Sprintf("%x", blake2bHash.Sum(nil))
	sha512Sum := fmt.Sprintf("%x", sha512Hash.Sum(nil))

	return size, blake2bSum, sha512Sum, nil
}

func upsertManifest(manifestPath, filename, manifestLine string) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(manifestPath, []byte(manifestLine+"\n"), 0644)
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, "DIST "+filename+" ") {
			lines[i] = manifestLine
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, manifestLine)
	}

	return os.WriteFile(manifestPath, []byte(strings.Join(lines, "\n")), 0644)
}
