package g2

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// LayoutConf represents the contents of a metadata/layout.conf file
type LayoutConf struct {
	Entries []LayoutConfEntry
}

// LayoutConfEntry represents a single key-value entry with its preceding comments
type LayoutConfEntry struct {
	Comments []string
	Key      string
	Value    string
}

// ParseLayoutConf parses a metadata/layout.conf file
func ParseLayoutConf(path string) (*LayoutConf, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return parseLayoutConfFromReader(file)
}

// ParseLayoutConfFromReader parses a layout.conf from an io.Reader
func ParseLayoutConfFromReader(r io.Reader) (*LayoutConf, error) {
	return parseLayoutConfFromReader(r)
}

func parseLayoutConfFromReader(r io.Reader) (*LayoutConf, error) {
	scanner := bufio.NewScanner(r)
	lc := &LayoutConf{}
	var currentComments []string

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" {
			// keep empty lines as part of comments or formatting if you want,
			// but usually we can just ignore or treat them as a blank comment
			currentComments = append(currentComments, "")
			continue
		}

		if strings.HasPrefix(trimmedLine, "#") {
			currentComments = append(currentComments, trimmedLine)
			continue
		}

		parts := strings.SplitN(trimmedLine, "=", 2)
		if len(parts) == 2 {
			entry := LayoutConfEntry{
				Comments: currentComments,
				Key:      strings.TrimSpace(parts[0]),
				Value:    strings.TrimSpace(parts[1]),
			}
			lc.Entries = append(lc.Entries, entry)
			currentComments = nil // reset for next entry
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lc, nil
}

// WriteLayoutConf writes a LayoutConf back to a file
func WriteLayoutConf(lc *LayoutConf, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	for i, entry := range lc.Entries {
		for _, comment := range entry.Comments {
			if comment == "" && i == 0 { // avoid leading blank line if that's all it is
				continue
			}
			if _, err := fmt.Fprintln(file, comment); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(file, "%s = %s\n", entry.Key, entry.Value); err != nil {
			return err
		}
	}
	return nil
}

// HasKey returns true if a specific key exists
func (lc *LayoutConf) HasKey(key string) bool {
	for _, entry := range lc.Entries {
		if entry.Key == key {
			return true
		}
	}
	return false
}

// GetValue returns the value for a specific key
func (lc *LayoutConf) GetValue(key string) string {
	for _, entry := range lc.Entries {
		if entry.Key == key {
			return entry.Value
		}
	}
	return ""
}

// SetValue sets the value for a specific key, updating it if it exists or appending if it doesn't
func (lc *LayoutConf) SetValue(key, value string) {
	for i, entry := range lc.Entries {
		if entry.Key == key {
			lc.Entries[i].Value = value
			return
		}
	}
	lc.Entries = append(lc.Entries, LayoutConfEntry{Key: key, Value: value})
}

// UnsetValue removes a specific key
func (lc *LayoutConf) UnsetValue(key string) {
	newEntries := make([]LayoutConfEntry, 0, len(lc.Entries))
	for _, entry := range lc.Entries {
		if entry.Key != key {
			newEntries = append(newEntries, entry)
		}
	}
	lc.Entries = newEntries
}

// GetValuesAsSlice returns the value for a specific key split by spaces
func (lc *LayoutConf) GetValuesAsSlice(key string) []string {
	val := lc.GetValue(key)
	if val == "" {
		return nil
	}
	return strings.Fields(val)
}

// Masters returns the list of master repositories.
func (lc *LayoutConf) Masters() []string {
	return lc.GetValuesAsSlice("masters")
}

// ManifestHashes returns the list of allowed manifest hashes.
func (lc *LayoutConf) ManifestHashes() []string {
	return lc.GetValuesAsSlice("manifest-hashes")
}

// ManifestRequiredHashes returns the list of required manifest hashes.
func (lc *LayoutConf) ManifestRequiredHashes() []string {
	return lc.GetValuesAsSlice("manifest-required-hashes")
}

// UseManifests returns the policy for creating and using Manifest files (e.g., strict, true, false).
func (lc *LayoutConf) UseManifests() string {
	return lc.GetValue("use-manifests")
}

// UpdateChangelog indicates whether development tools should write ChangeLog files.
func (lc *LayoutConf) UpdateChangelog() bool {
	return lc.GetValue("update-changelog") == "true"
}

// CacheFormats returns the cache formats used by the repository.
func (lc *LayoutConf) CacheFormats() []string {
	return lc.GetValuesAsSlice("cache-formats")
}

// EapisDeprecated returns the list of deprecated EAPIs for ebuilds.
func (lc *LayoutConf) EapisDeprecated() []string {
	return lc.GetValuesAsSlice("eapis-deprecated")
}

// EapisBanned returns the list of banned EAPIs for ebuilds.
func (lc *LayoutConf) EapisBanned() []string {
	return lc.GetValuesAsSlice("eapis-banned")
}

// EapisTesting returns the list of testing EAPIs for ebuilds.
func (lc *LayoutConf) EapisTesting() []string {
	return lc.GetValuesAsSlice("eapis-testing")
}

// ProfileEapisDeprecated returns the list of deprecated EAPIs for profiles.
func (lc *LayoutConf) ProfileEapisDeprecated() []string {
	return lc.GetValuesAsSlice("profile-eapis-deprecated")
}

// ProfileEapisBanned returns the list of banned EAPIs for profiles.
func (lc *LayoutConf) ProfileEapisBanned() []string {
	return lc.GetValuesAsSlice("profile-eapis-banned")
}

// RepoName returns the specified repository name.
func (lc *LayoutConf) RepoName() string {
	return lc.GetValue("repo-name")
}

// Aliases returns the list of alternative names for the repository.
func (lc *LayoutConf) Aliases() []string {
	return lc.GetValuesAsSlice("aliases")
}

// ThinManifests indicates whether thin manifests are used.
func (lc *LayoutConf) ThinManifests() bool {
	return lc.GetValue("thin-manifests") == "true"
}

// SignCommits indicates whether git commits should be signed.
func (lc *LayoutConf) SignCommits() bool {
	return lc.GetValue("sign-commits") == "true"
}

// SignManifests indicates whether individual package manifests should be signed.
func (lc *LayoutConf) SignManifests() bool {
	return lc.GetValue("sign-manifests") == "true"
}

// PropertiesAllowed returns the list of properties permitted in ebuilds.
func (lc *LayoutConf) PropertiesAllowed() []string {
	return lc.GetValuesAsSlice("properties-allowed")
}

// RestrictAllowed returns the list of restrict tokens permitted in ebuilds.
func (lc *LayoutConf) RestrictAllowed() []string {
	return lc.GetValuesAsSlice("restrict-allowed")
}

// ProfileFormats returns the formats used by profiles.
func (lc *LayoutConf) ProfileFormats() []string {
	return lc.GetValuesAsSlice("profile-formats")
}
