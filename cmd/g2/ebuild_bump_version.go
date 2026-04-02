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

func (cfg *CmdEbuildArgConfig) cmdEbuildBumpVersion(args []string) error {
	fs := flag.NewFlagSet("bump-version", flag.ExitOnError)
	tagOnly := fs.Bool("tag", false, "Only focus on the ebuild file, do not update Manifest")
	bumpRevision := fs.Bool("revision", false, "Bump the revision version")
	bumpPatch := fs.Bool("patch", false, "Bump the patch version")
	bumpMinor := fs.Bool("minor", false, "Bump the minor version")
	bumpMajor := fs.Bool("major", false, "Bump the major version")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: g2 ebuild bump-version [--tag] [--revision|--patch|--minor|--major] <old-ebuild> [new-version]")
	}

	oldEbuildPath := fs.Arg(0)

	// Clean the path
	oldEbuildPath = filepath.Clean(oldEbuildPath)
	dir := filepath.Dir(oldEbuildPath)
	base := filepath.Base(oldEbuildPath)

	if !strings.HasSuffix(base, ".ebuild") {
		return fmt.Errorf("expected ebuild file, got %s", base)
	}

	vars := g2.ParseEbuildVariables(base)
	if vars == nil || vars["PN"] == "" {
		return fmt.Errorf("failed to parse PN from ebuild filename %s", base)
	}
	pn := vars["PN"]

	var newVersion string
	if *bumpRevision || *bumpPatch || *bumpMinor || *bumpMajor {
		if fs.NArg() > 1 {
			return fmt.Errorf("cannot specify both bump flags and a manual new-version")
		}
		gv := g2.ParseGentooVersion(vars["PV"])
		if !gv.IsValid {
			return fmt.Errorf("could not parse version %s for incrementing", vars["PV"])
		}

		if *bumpRevision {
			gv.IncrementPart("revision")
		} else if *bumpMajor {
			gv.IncrementPart("major")
		} else if *bumpMinor {
			gv.IncrementPart("minor")
		} else if *bumpPatch {
			gv.IncrementPart("patch")
		}
		newVersion = gv.String()
	} else {
		if fs.NArg() != 2 {
			return fmt.Errorf("usage: g2 ebuild bump-version [--tag] <old-ebuild> <new-version>")
		}
		newVersion = fs.Arg(1)
	}

	// Rename ebuild
	newBase := fmt.Sprintf("%s-%s.ebuild", pn, newVersion)
	newEbuildPath := filepath.Join(dir, newBase)

	log.Printf("Bumping %s to version %s (%s)\n", base, newVersion, newBase)

	if err := os.Rename(oldEbuildPath, newEbuildPath); err != nil {
		return fmt.Errorf("failed to rename ebuild: %w", err)
	}

	if *tagOnly {
		log.Printf("Tag mode specified, skipping Manifest update.")
		return nil
	}

	// Fix References (Manifest)
	// We check if Manifest exists
	manifestPath := filepath.Join(dir, "Manifest")
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			log.Printf("No Manifest found in %s, skipping Manifest update.", dir)
			return nil
		}
		return fmt.Errorf("failed to stat Manifest: %w", err)
	}

	// 1. Remove entries for the old ebuild
	content, err := os.ReadFile(newEbuildPath)
	if err != nil {
		return fmt.Errorf("failed to read new ebuild %s: %w", newEbuildPath, err)
	}

	oldVars := vars
	if oldVars["P"] == "" {
		// Calculate P if missing
		oldVars["P"] = strings.TrimSuffix(base, ".ebuild")
	}
	oldUris, _ := g2.ExtractURIs(string(content), oldVars)

	newVars := map[string]string{
		"P":  pn + "-" + newVersion,
		"PN": pn,
		"PV": newVersion,
	}
	newUris, err := g2.ExtractURIs(string(content), newVars)
	if err != nil {
		log.Printf("Warning: failed to extract URIs from new ebuild: %v", err)
		newUris = nil
	}

	// Check all other ebuilds in the directory to see if they need the old URIs
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	neededByOthers := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") || entry.Name() == newBase {
			continue
		}

		otherVars := g2.ParseEbuildVariables(entry.Name())
		if otherVars == nil {
			continue
		}

		otherContent, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		otherUris, _ := g2.ExtractURIs(string(otherContent), otherVars)
		for _, u := range otherUris {
			neededByOthers[u.Filename] = true
		}
	}

	// Remove old URIs and download new ones
	manifest, err := g2.ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	for _, uri := range oldUris {
		// Only remove if it's not present in the new ones AND not needed by other ebuilds
		foundInNew := false
		for _, newUri := range newUris {
			if uri.Filename == newUri.Filename {
				foundInNew = true
				break
			}
		}
		if !foundInNew && !neededByOthers[uri.Filename] {
			log.Printf("Removing old manifest entry: %s", uri.Filename)
			manifest.Remove(uri.Filename)
		}
	}

	// 2. Add entries for the new ebuild
	// Find the hashes required
	hashes := []string{g2.HashBlake2b, g2.HashSha512} // Default
	repoDir := filepath.Dir(dir)
	layoutConfPath := filepath.Join(repoDir, "metadata", "layout.conf")
	if lc, err := g2.ParseLayoutConf(layoutConfPath); err == nil {
		if manifestHashes := lc.GetValuesAsSlice("manifest-hashes"); len(manifestHashes) > 0 {
			hashes = manifestHashes
		}
	}

	for _, uri := range newUris {
		if manifest.GetEntry(uri.Filename) == nil {
			log.Printf("Fetching and hashing new manifest entry: %s (%s)", uri.Filename, uri.URL)
			checksums, err := g2.DownloadAndChecksum(uri.URL, hashes)
			if err != nil {
				// Don't fail the whole bump if one file is missing, maybe it hasn't been uploaded yet or is a dummy test
				log.Printf("Warning: error downloading and checksumming %s: %v", uri.URL, err)
				continue
			}

			entry := g2.NewManifestEntry("DIST", uri.Filename, checksums.Size)
			for _, h := range g2.AllHashes {
				if checksums.Hashes[h] != "" {
					entry.AddHash(h, checksums.Hashes[h])
				}
			}
			manifest.AddOrReplace(entry)
		}
	}

	manifest.Sort()
	if err := os.WriteFile(manifestPath, []byte(manifest.String()), 0644); err != nil {
		return fmt.Errorf("failed to write updated Manifest: %w", err)
	}

	return nil
}
