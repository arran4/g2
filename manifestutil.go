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

	found := false
	for i, entry := range m.Entries {
		if entry.Filename == newEntry.Filename {
			m.Entries[i] = newEntry
			found = true
			break
		}
	}

	if !found {
		m.Entries = append(m.Entries, newEntry)
	}

	m.Sort()

	return os.WriteFile(manifestPath, []byte(m.String()), 0644)
}
