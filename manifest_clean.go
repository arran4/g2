package g2

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// CleanManifest removes unused DIST and EBUILD entries from a Manifest object
// based on the `.ebuild` files found in the given filesystem directory.
func CleanManifest(sysFS fs.FS, directory string, manifest *Manifest) error {
	entries, err := fs.ReadDir(sysFS, directory)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	foundFiles := make(map[string]bool)
	var uncertainEbuilds []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
			continue
		}

		ebuildName := entry.Name()

		e, err := ParseEbuild(sysFS, path.Join(directory, ebuildName), ParseFull)
		if err != nil {
			uncertainEbuilds = append(uncertainEbuilds, ebuildName)
			continue
		}

		if !e.IsSrcUriAuthoritative() {
			uncertainEbuilds = append(uncertainEbuilds, ebuildName)
			continue
		}

		foundFiles[ebuildName] = true
		for _, uri := range e.SrcUri {
			foundFiles[uri.Filename] = true
		}
	}

	if len(uncertainEbuilds) > 0 {
		return fmt.Errorf("skipped cleanup: directory contains uncertain ebuilds: %v", uncertainEbuilds)
	}

	var filesToRemove []string
	for _, entry := range manifest.Entries {
		if (entry.Type == "DIST" || entry.Type == "EBUILD") && !foundFiles[entry.Filename] {
			filesToRemove = append(filesToRemove, entry.Filename)
		}
	}

	for _, filename := range filesToRemove {
		manifest.Remove(filename)
	}

	return nil
}
