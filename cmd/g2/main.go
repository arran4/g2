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
		fmt.Printf("\t\t %s \t\t %s\n", "verify", "To verify the manifest against ebuild files")
		fmt.Printf("\t\t %s \t\t %s\n", "clean", "To clean up the manifest from unused entries")
	}

	config := &CmdManifestArgConfig{
		MainArgConfig: cfg,
	}

	// Flags for checksums, shared across commands if needed, or specific to upsert
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

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	getHashes := func() []string {
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
		return hashes
	}

	switch cmd {
	case "upsert-from-url":
		urlArgs := fs.Args()[1:]
		if err := config.cmdUpsertFromUrl(urlArgs, getHashes()); err != nil {
			return fmt.Errorf("upsert file from url: %w", err)
		}
	case "verify":
		verifyArgs := fs.Args()[1:]
		if err := config.cmdVerify(verifyArgs, getHashes()); err != nil {
			return fmt.Errorf("verify manifest: %w", err)
		}
	case "clean":
		cleanArgs := fs.Args()[1:]
		if err := config.cmdClean(cleanArgs); err != nil {
			return fmt.Errorf("clean manifest: %w", err)
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

    // Logic to be moved to a reusable function if we want to reuse it in verify --fix
    // For now I'll just keep it here and maybe call this function or copy logic.

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

func (cfg *CmdManifestArgConfig) cmdVerify(args []string, hashes []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	fix := fs.Bool("fix", false, "Force fix missing manifest entries")
    clean := fs.Bool("clean", false, "Clean up unused manifest entries")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: verify [--fix] [--clean] <manifestFileOrDir>")
	}

	target := fs.Arg(0)

    // Determine manifest path and directory
    var manifestPath, directory string
    info, err := os.Stat(target)
    if err != nil {
        return fmt.Errorf("stat target: %w", err)
    }

    if info.IsDir() {
        directory = target
        manifestPath = filepath.Join(target, "Manifest")
    } else {
        manifestPath = target
        directory = filepath.Dir(target)
    }

    log.Printf("Processing directory: %s", directory)

    // Load Manifest
    manifestEntries, err := g2.ReadManifest(manifestPath)
    if err != nil {
        return fmt.Errorf("reading manifest: %w", err)
    }

    // Find all ebuilds
    entries, err := os.ReadDir(directory)
    if err != nil {
        return fmt.Errorf("reading directory: %w", err)
    }

    foundFiles := make(map[string]bool)

    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
            continue
        }

        ebuildName := entry.Name()
        log.Printf("  Parsing %s...", ebuildName)

        variables := g2.ParseEbuildVariables(ebuildName)
        if variables == nil {
             log.Printf("  Skipping %s: Could not parse version/name.", ebuildName)
             continue
        }

        content, err := os.ReadFile(filepath.Join(directory, ebuildName))
        if err != nil {
             return fmt.Errorf("reading ebuild %s: %w", ebuildName, err)
        }

        uris, err := g2.ExtractURIs(string(content), variables)
        if err != nil {
            // Log error but maybe continue?
             log.Printf("    Error extracting URIs from %s: %v", ebuildName, err)
             continue
        }

        for _, uri := range uris {
            foundFiles[uri.Filename] = true

            if _, ok := manifestEntries[uri.Filename]; ok {
                // Entry exists.
                // In a full verify we might want to check checksums if file exists locally,
                // but the prompt implies verifying the manifest *entries* exist for the ebuilds.
                // The prompt says "with a force fix", which implies if it's missing, we fix it.
                // The python script calls upsert-from-url.
                log.Printf("    Found in manifest: %s", uri.Filename)
            } else {
                log.Printf("    MISSING in manifest: %s (URL: %s)", uri.Filename, uri.URL)
                if *fix {
                     log.Printf("    Upserting: %s -> %s", uri.URL, uri.Filename)
                     // Reuse logic from upsert-from-url
                     // We need to call internal logic, not the CLI command ideally, but I can call cmdUpsertFromUrl
                     // or refactor the logic.
                     // I'll call a helper function.

                     err := cfg.upsertFromUrlLogic(uri.URL, uri.Filename, manifestPath, hashes)
                     if err != nil {
                         log.Printf("    Error updating manifest for %s: %v", uri.URL, err)
                     } else {
                         // Update in-memory manifest map so we don't think it is missing later?
                         // Or just rely on it being written to disk.
                     }
                }
            }
        }
    }

    if *clean {
        // Run clean logic
         if err := cfg.cleanLogic(manifestPath, foundFiles); err != nil {
             return fmt.Errorf("cleaning manifest: %w", err)
         }
    }

	return nil
}

func (cfg *CmdManifestArgConfig) cmdClean(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: clean <manifestFileOrDir>")
	}
	target := args[0]

    // Determine manifest path and directory
    var manifestPath, directory string
    info, err := os.Stat(target)
    if err != nil {
        return fmt.Errorf("stat target: %w", err)
    }

    if info.IsDir() {
        directory = target
        manifestPath = filepath.Join(target, "Manifest")
    } else {
        manifestPath = target
        directory = filepath.Dir(target)
    }

    log.Printf("Processing directory: %s", directory)

    // Parse all ebuilds to find used files
    foundFiles := make(map[string]bool)

     entries, err := os.ReadDir(directory)
    if err != nil {
        return fmt.Errorf("reading directory: %w", err)
    }

    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
            continue
        }

        ebuildName := entry.Name()

        variables := g2.ParseEbuildVariables(ebuildName)
        if variables == nil {
             continue
        }

        content, err := os.ReadFile(filepath.Join(directory, ebuildName))
        if err != nil {
             return fmt.Errorf("reading ebuild %s: %w", ebuildName, err)
        }

        uris, err := g2.ExtractURIs(string(content), variables)
        if err != nil {
             continue
        }

        for _, uri := range uris {
            foundFiles[uri.Filename] = true
        }
    }

    return cfg.cleanLogic(manifestPath, foundFiles)
}

func (cfg *CmdManifestArgConfig) cleanLogic(manifestPath string, usedFiles map[string]bool) error {
    manifestEntries, err := g2.ReadManifest(manifestPath)
    if err != nil {
        return fmt.Errorf("reading manifest: %w", err)
    }

    var linesToRemove []string
    for filename := range manifestEntries {
        if !usedFiles[filename] {
            log.Printf("  Unused entry: %s", filename)
            linesToRemove = append(linesToRemove, filename)
        }
    }

    if len(linesToRemove) == 0 {
        log.Println("Nothing to clean.")
        return nil
    }

    // Read the file lines to rewrite it preserving comments if any (though ReadManifest logic skips them/doesn't track them well)
    // Actually UpsertManifest reads file and rewrites it.
    // Ideally we should just read lines, filter out unwanted DIST lines, and write back.

    content, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
    var newLines []string

    for _, line := range lines {
        if strings.TrimSpace(line) == "" {
             // Keep empty lines?
             continue
        }
        parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == "DIST" {
			filename := parts[1]
            if !usedFiles[filename] {
                continue // Skip this line
            }
		}
        newLines = append(newLines, line)
    }

    // Add trailing newline if needed

    return os.WriteFile(manifestPath, []byte(strings.Join(newLines, "\n") + "\n"), 0644)
}


func (cfg *CmdManifestArgConfig) upsertFromUrlLogic(url, filename, manifestPath string, hashes []string) error {
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

	return g2.UpsertManifest(manifestPath, filename, manifestLine)
}
