package g2

import (
	"os"
)

// UpsertManifest updates or inserts a manifest entry.
func UpsertManifest(manifestPath string, newEntry *ManifestEntry) error {
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return err
	}

	m.AddOrReplace(newEntry)

	m.Sort()

	return os.WriteFile(manifestPath, []byte(m.String()), 0644)
}
