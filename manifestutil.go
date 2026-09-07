package g2

import (
	"os"
	"path/filepath"
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
	dir := filepath.Dir(manifestPath)
	tmpFile, err := os.CreateTemp(dir, "Manifest.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// Track if successfully closed/renamed
	success := false

	defer func() {
		if !success {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(0644); err != nil {
		return err
	}

	if _, err := tmpFile.Write([]byte(m.String())); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Rename over the old file
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		// Even if Rename fails, we already Closed above, so the defer will just run Remove.
		// tmpFile.Close() twice is harmless.
		return err
	}

	success = true
	return nil
}
