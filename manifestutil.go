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

	return AtomicWriteManifest(manifestPath, m)
}

// AtomicWriteManifest atomically writes the given manifest to the specified path.
func AtomicWriteManifest(manifestPath string, m *Manifest) error {
	tmpPath := manifestPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(m.String()), 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, manifestPath)
}
