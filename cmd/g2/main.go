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
	if fs.NArg() <= 1 {
		log.Printf("Please specify an argument, try -help for help")
		os.Exit(-1)
		return
	}

	cmd := fs.Arg(1)
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

	blake2b := fs.Bool("blake2b", true, "Calculate BLAKE2B checksum")
	blake2s := fs.Bool("blake2s", false, "Calculate BLAKE2S checksum")
	md5 := fs.Bool("md5", false, "Calculate MD5 checksum")
	rmd160 := fs.Bool("rmd160", false, "Calculate RMD160 checksum")
	sha1 := fs.Bool("sha1", false, "Calculate SHA1 checksum")
	sha256 := fs.Bool("sha256", false, "Calculate SHA256 checksum")
	sha3_256 := fs.Bool("sha3_256", false, "Calculate SHA3_256 checksum")
	sha3_512 := fs.Bool("sha3_512", false, "Calculate SHA3_512 checksum")
	sha512 := fs.Bool("sha512", true, "Calculate SHA512 checksum")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}
	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)
	switch cmd {
	case "upsert-from-url":
		url := fs.Args()[1:]
		hashes := make([]string, 0)
		if *blake2b { hashes = append(hashes, g2.HashBlake2b) }
		if *blake2s { hashes = append(hashes, g2.HashBlake2s) }
		if *md5 { hashes = append(hashes, g2.HashMd5) }
		if *rmd160 { hashes = append(hashes, g2.HashRmd160) }
		if *sha1 { hashes = append(hashes, g2.HashSha1) }
		if *sha256 { hashes = append(hashes, g2.HashSha256) }
		if *sha3_256 { hashes = append(hashes, g2.HashSha3_256) }
		if *sha3_512 { hashes = append(hashes, g2.HashSha3_512) }
		if *sha512 { hashes = append(hashes, g2.HashSha512) }

		if err := config.cmdUpsertFromUrl(url, hashes); err != nil {
			return fmt.Errorf("upsert file from url %s: %w", url, err)
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

func (cfg *CmdManifestArgConfig) cmdUpsertFromUrl(args []string, hashes []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: upsert-from-url <url> <filename> <manifestFileOrDir>")
	}

	url := args[0]
	filename := args[1]
	ebuildDirOrFile := args[2]

	checksums, err := g2.DownloadAndChecksum(url, hashes)
	if err != nil {
		return fmt.Errorf("downloading and calculating checksums: %v\n", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("DIST %s %d", filename, checksums.Size))

	// Helper to append hash if it's computed
	appendHash := func(name, value string) {
		if value != "" {
			sb.WriteString(fmt.Sprintf(" %s %s", name, value))
		}
	}

	// Order matters: use the sorted order of keys as previously seen,
    // or just iterate through a known list in order.
    // The previous implementation used: BLAKE2B BLAKE2S MD5 RMD160 SHA1 SHA256 SHA3_256 SHA3_512 SHA512
	appendHash("BLAKE2B", checksums.Blake2b)
	appendHash("BLAKE2S", checksums.Blake2s)
	appendHash("MD5", checksums.Md5)
	appendHash("RMD160", checksums.Rmd160)
	appendHash("SHA1", checksums.Sha1)
	appendHash("SHA256", checksums.Sha256)
	appendHash("SHA3_256", checksums.Sha3_256)
	appendHash("SHA3_512", checksums.Sha3_512)
	appendHash("SHA512", checksums.Sha512)

	manifestLine := sb.String()

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
