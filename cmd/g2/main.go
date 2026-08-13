package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
)

var version = "dev"

type MainArgConfig struct {
	Args []string
}

// ExitError is a custom error that can be returned by commands to exit with a specific code silently or with an error.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	fs := flag.NewFlagSet("", flag.ExitOnError)
	cfg := &MainArgConfig{
		Args: []string{os.Args[0]},
	}
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "manifest", "commands relating to Manifest files")
		fmt.Printf("\t\t %s \t\t %s\n", "versions", "commands relating to version utilities")
		fmt.Printf("\t\t %s \t\t %s\n", "metadata", "commands relating to metadata.xml files")
		fmt.Printf("\t\t %s \t\t %s\n", "ebuild", "commands relating to ebuild files")
		fmt.Printf("\t\t %s \t\t %s\n", "overlay", "commands relating to a single overlay")
		fmt.Printf("\t\t %s \t\t %s\n", "overlays", "commands relating to multiple overlays")
		fmt.Printf("\t\t %s \t\t %s\n", "lint", "lints the repository for errors (supports optional target packages)")
		fmt.Printf("\t\t %s \t\t %s\n", "use", "commands relating to USE flags, use.desc, and use.local.desc")
		fmt.Printf("\t\t %s \t\t %s\n", "site", "commands relating to static sites")
		fmt.Printf("\t\t %s \t\t %s\n", "cache", "commands relating to md5-dict/cache")
		fmt.Printf("\t\t %s \t\t %s\n", "pkg-desc-index", "commands relating to pkg_desc_index")
		fmt.Printf("\t\t %s \t\t %s\n", "dev", "tools for developers and agents")
		fmt.Printf("\t\t %s \t\t %s\n", "package", "commands relating to packages and search indexing")
		fmt.Printf("\t\t %s \t\t %s\n", "eclass", "commands relating to eclasses")
		fmt.Printf("\t\t %s \t\t %s\n", "arch", "commands relating to architectures")
		fmt.Printf("\t\t %s \t\t %s\n", "profile", "commands relating to profiles")
		fmt.Printf("\t\t %s \t\t %s\n", "repos-conf", "commands relating to repos.conf")
		fmt.Printf("\t\t %s \t\t %s\n", "make-conf", "commands relating to make.conf")
		fmt.Printf("\t\t %s \t\t %s\n", "conf", "commands relating to portage configuration")
		fmt.Printf("\t\t %s \t\t %s\n", "skill", "manage agent skills")
		fmt.Printf("\t\t %s \t\t %s\n", "world", "manage the portage world file via TUI")
	}
	if err := fs.Parse(os.Args); err != nil {
		log.Printf("Flag parse error: %s", err)
		os.Exit(1)
		return
	}
	if fs.NArg() <= 1 {
		log.Printf("Please specify an argument, try -help for help")
		os.Exit(1)
		return
	}

	cmd := fs.Arg(1)
	cfg.Args = append(cfg.Args, cmd)
	var err error
	logPrefix := cmd
	switch cmd {
	case "skill":
		err = cfg.cmdSkill(fs.Args()[2:])
	case "masks":
		err = cfg.cmdMasks(fs.Args()[2:])
	case "conf":
		err = cfg.cmdConf(fs.Args()[2:])
	case "arch":
		err = cfg.cmdArch(fs.Args()[2:])
	case "profile":
		err = ProfileCommand(fs.Args()[2:])
	case "repos-conf":
		err = cfg.cmdReposConf(fs.Args()[2:])
	case "make-conf":
		err = cfg.cmdMakeConf(fs.Args()[2:])
	case "world":
		err = cfg.cmdWorld(fs.Args()[2:])
	case "manifest":
		logPrefix = "generate"
		err = cfg.cmdManifest(fs.Args()[2:])
	case "layout-conf":
		err = cfg.cmdLayoutConf(fs.Args()[2:])
	case "versions":
		err = cfg.CmdVersions(fs.Args()[2:])
	case "metadata":
		err = cfg.cmdMetadata(fs.Args()[2:])
	case "ebuild":
		err = cfg.cmdEbuild(fs.Args()[2:])
	case "overlay":
		err = cfg.cmdOverlay(fs.Args()[2:])
	case "overlays":
		err = cfg.cmdOverlays(fs.Args()[2:])
	case "lint":
		err = cfg.cmdLint(fs.Args()[2:])
	case "use":
		err = cfg.cmdUse(fs.Args()[2:])
	case "site":
		err = cfg.cmdSite(fs.Args()[2:])
	case "cache":
		err = cfg.cmdCache(fs.Args()[2:])
	case "pkg-desc-index":
		err = cfg.cmdPkgDescIndex(fs.Args()[2:])
	case "package":
		err = cfg.cmdPackage(fs.Args()[2:])
	case "eclass":
		err = cfg.cmdEclass(fs.Args()[2:])
	case "dev":
		err = cmdDev()
	case "help", "-help", "--help":
		fs.Usage()
		return
	default:
		fmt.Printf("Unknown command %s\n", cmd)
		fs.Usage()
		return
	}

	if err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			if exitErr.Err != nil {
				log.Printf("%s error: %s", logPrefix, exitErr.Err)
			}
			os.Exit(exitErr.Code)
		}
		log.Printf("%s error: %s", logPrefix, err)
		os.Exit(1)
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
		if *blake2b {
			hashes = append(hashes, g2.HashBlake2b)
		}
		if *blake2s {
			hashes = append(hashes, g2.HashBlake2s)
		}
		if *md5 {
			hashes = append(hashes, g2.HashMd5)
		}
		if *rmd160 {
			hashes = append(hashes, g2.HashRmd160)
		}
		if *sha1 {
			hashes = append(hashes, g2.HashSha1)
		}
		if *sha256 {
			hashes = append(hashes, g2.HashSha256)
		}
		if *sha3_256 {
			hashes = append(hashes, g2.HashSha3_256)
		}
		if *sha3_512 {
			hashes = append(hashes, g2.HashSha3_512)
		}
		if *sha512 {
			hashes = append(hashes, g2.HashSha512)
		}
		return hashes
	}

	switch cmd {
	case "upsert-from-url":
		urlArgs := fs.Args()[1:]
		hashes := getHashes()
		if len(urlArgs) >= 3 {
			ebuildDirOrFile := urlArgs[2]
			dir := filepath.Dir(ebuildDirOrFile)
			if filepath.Base(ebuildDirOrFile) == "Manifest" {
				dir = filepath.Dir(ebuildDirOrFile)
			}
			repoDir := filepath.Dir(filepath.Dir(dir)) // Assuming category/package
			layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
			if lc, err := g2.ParseLayoutConf(layoutConfPath); err == nil {
				if manifestHashes := lc.GetValuesAsSlice("manifest-hashes"); len(manifestHashes) > 0 {
					hashes = manifestHashes
				}
			}
		}
		if err := config.cmdUpsertFromUrl(urlArgs, hashes); err != nil {
			urlStr := ""
			if len(urlArgs) > 0 {
				urlStr = " " + urlArgs[0]
			}
			return &ExitError{Code: 1, Err: fmt.Errorf("upsert file from url%s: %w", urlStr, err)}
		}
	case "verify":
		verifyArgs := fs.Args()[1:]
		hashes := getHashes()
		if len(verifyArgs) > 0 {
			target := verifyArgs[len(verifyArgs)-1] // Last arg should be target
			if !strings.HasPrefix(target, "-") {
				dir := target
				if !strings.HasSuffix(dir, "/") {
					// We might be passing a directory or file. Let's make sure we find the repo root properly.
					if filepath.Base(target) == "Manifest" {
						dir = filepath.Dir(target)
					}
				}

				// Keep going up until we find metadata/layout.conf or hit root
				currentDir, _ := filepath.Abs(dir)
				var layoutConfPath string
				for {
					candidate := filepath.Join(currentDir, "metadata", "layout.conf")
					if _, err := os.Stat(candidate); err == nil {
						layoutConfPath = candidate
						break
					}
					parent := filepath.Dir(currentDir)
					if parent == currentDir {
						break // Root reached
					}
					currentDir = parent
				}

				if layoutConfPath != "" {
					if lc, err := g2.ParseLayoutConf(layoutConfPath); err == nil {
						if manifestHashes := lc.GetValuesAsSlice("manifest-hashes"); len(manifestHashes) > 0 {
							hashes = manifestHashes
						}
					}
				}
			}
		}
		if err := config.cmdVerify(verifyArgs, hashes); err != nil {
			return fmt.Errorf("verify manifest: %w", err)
		}
	case "clean":
		cleanArgs := fs.Args()[1:]
		if err := config.cmdClean(cleanArgs); err != nil {
			return fmt.Errorf("clean manifest: %w", err)
		}
	case "help", "-help", "--help":
		fs.Usage()
		return nil
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
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
		return fmt.Errorf("downloading and calculating checksums for %s: %w", url, err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "DIST %s %d", filename, checksums.Size)

	// Helper to append hash if it's computed
	appendHash := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&sb, " %s %s", name, value)
		}
	}

	for _, h := range g2.AllHashes {
		appendHash(h, checksums.Hashes[h])
	}

	manifestPath := ebuildDirOrFile
	if _, file := filepath.Split(manifestPath); file != "Manifest" {
		manifestPath = filepath.Join(ebuildDirOrFile, "Manifest")
	}

	entry := g2.NewManifestEntry("DIST", filename, checksums.Size)

	// Helper to append hash if it's computed
	appendHashToEntry := func(name, value string) {
		if value != "" {
			entry.AddHash(name, value)
		}
	}

	for _, h := range g2.AllHashes {
		appendHashToEntry(h, checksums.Hashes[h])
	}

	err = g2.UpsertManifest(manifestPath, entry)
	if err != nil {
		return fmt.Errorf("updating manifest: %w", err)
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
	manifest, err := g2.ParseManifest(manifestPath)
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
		foundFiles[ebuildName] = true
		log.Printf("  Parsing %s...", ebuildName)

		if manifestEntry := manifest.GetEntry(ebuildName); manifestEntry == nil {
			log.Printf("    MISSING in manifest: %s", ebuildName)
		}

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

			if entry := manifest.GetEntry(uri.Filename); entry != nil {
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
					}
				}
			}
		}
	}

	for _, entry := range manifest.Entries {
		if entry.Type == "DIST" || entry.Type == "EBUILD" {
			if !foundFiles[entry.Filename] {
				log.Printf("    EXTRA in manifest: %s", entry.Filename)
			}
		}
	}

	if *clean {
		// Run clean logic
		err = g2.CleanManifest(os.DirFS(directory), ".", manifest)
		if err != nil {
			return fmt.Errorf("cleaning manifest: %w", err)
		}

		err = os.WriteFile(manifestPath, []byte(manifest.String()), 0644)
		if err != nil {
			return fmt.Errorf("writing clean manifest: %w", err)
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

	manifest, err := g2.ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	err = g2.CleanManifest(os.DirFS(directory), ".", manifest)
	if err != nil {
		return fmt.Errorf("cleaning manifest: %w", err)
	}

	return os.WriteFile(manifestPath, []byte(manifest.String()), 0644)
}

func (cfg *CmdManifestArgConfig) upsertFromUrlLogic(url, filename, manifestPath string, hashes []string) error {
	checksums, err := g2.DownloadAndChecksum(url, hashes)
	if err != nil {
		return fmt.Errorf("downloading and calculating checksums for %s: %w", url, err)
	}

	entry := g2.NewManifestEntry("DIST", filename, checksums.Size)

	// Helper to append hash if it's computed
	appendHash := func(name, value string) {
		if value != "" {
			entry.AddHash(name, value)
		}
	}

	for _, h := range g2.AllHashes {
		appendHash(h, checksums.Hashes[h])
	}

	return g2.UpsertManifest(manifestPath, entry)
}
