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

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ebuild") {
			continue
		}

		ebuildName := entry.Name()
		foundFiles[ebuildName] = true

		variables := ParseEbuildVariables(ebuildName)
		if variables == nil {
			continue
		}

		content, err := fs.ReadFile(sysFS, path.Join(directory, ebuildName))
		if err != nil {
			return fmt.Errorf("reading ebuild %s: %w", ebuildName, err)
		}

		uris, err := ExtractURIs(string(content), variables)
		if err != nil {
			continue
		}

		for _, uri := range uris {
			foundFiles[uri.Filename] = true
		}
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
