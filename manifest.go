package g2

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Manifest struct {
	Entries []*ManifestEntry
}

type ManifestEntry struct {
	Type     string
	Filename string
	Size     int64
	Hashes   []Hash
}

type Hash struct {
	Type  string
	Value string
}

func (m *Manifest) String() string {
	var sb strings.Builder
	// Sort entries by filename to ensure deterministic output
	// Although sometimes we might want to preserve order, typically manifests are sorted.
	// But let's check if the existing code does sorting.
	// The existing UpsertManifest appends to the end or replaces in place.
	// If we want a "First Class Struct", usually that implies full control.
	// For now, I'll just iterate over entries. If sorting is needed, it can be done on the slice.

	// Actually, Gentoo manifests are usually sorted. But let's just output in the order they are in the struct.
	for _, entry := range m.Entries {
		sb.WriteString(entry.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

func (e *ManifestEntry) String() string {
	var sb strings.Builder
	sb.WriteString(e.Type)
	sb.WriteString(" ")
	sb.WriteString(e.Filename)
	sb.WriteString(" ")
	sb.WriteString(strconv.FormatInt(e.Size, 10))

	for _, hash := range e.Hashes {
		sb.WriteString(" ")
		sb.WriteString(hash.Type)
		sb.WriteString(" ")
		sb.WriteString(hash.Value)
	}
	return sb.String()
}

func (e *ManifestEntry) AddHash(hType, hValue string) {
	// Check if exists and update, or append
	for i, h := range e.Hashes {
		if h.Type == hType {
			e.Hashes[i].Value = hValue
			return
		}
	}
	e.Hashes = append(e.Hashes, Hash{Type: hType, Value: hValue})
}

func (e *ManifestEntry) GetHash(hType string) string {
	for _, h := range e.Hashes {
		if h.Type == hType {
			return h.Value
		}
	}
	return ""
}


func ParseManifest(path string) (*Manifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{}, nil
		}
		return nil, err
	}
	return ParseManifestContent(string(content))
}

func ParseManifestContent(content string) (*Manifest, error) {
	m := &Manifest{}
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, err := ParseManifestEntry(line)
		if err != nil {
			// Decide if we want to error on one bad line or continue.
			// Typically strict parsing is better.
			return nil, fmt.Errorf("parsing line '%s': %w", line, err)
		}
		m.Entries = append(m.Entries, entry)
	}
	return m, nil
}

func ParseManifestEntry(line string) (*ManifestEntry, error) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid manifest entry: not enough fields")
	}

	entry := &ManifestEntry{
		Type:     parts[0],
		Filename: parts[1],
	}

	size, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid size: %w", err)
	}
	entry.Size = size

	// hashes are in pairs
	hashParts := parts[3:]
	if len(hashParts)%2 != 0 {
		return nil, fmt.Errorf("invalid hashes: odd number of hash fields")
	}

	for i := 0; i < len(hashParts); i += 2 {
		entry.Hashes = append(entry.Hashes, Hash{
			Type:  hashParts[i],
			Value: hashParts[i+1],
		})
	}

	return entry, nil
}

// Sort sorts the manifest entries by type then filename
func (m *Manifest) Sort() {
	sort.Slice(m.Entries, func(i, j int) bool {
		if m.Entries[i].Type != m.Entries[j].Type {
			return m.Entries[i].Type < m.Entries[j].Type
		}
		return m.Entries[i].Filename < m.Entries[j].Filename
	})
}

// GetEntry returns the entry for a filename if it exists
func (m *Manifest) GetEntry(filename string) *ManifestEntry {
	for _, e := range m.Entries {
		if e.Filename == filename {
			return e
		}
	}
	return nil
}

func NewManifestEntry(typeStr, filename string, size int64, hashes ...Hash) *ManifestEntry {
	return &ManifestEntry{
		Type:     typeStr,
		Filename: filename,
		Size:     size,
		Hashes:   hashes,
	}
}

func (m *Manifest) AddOrReplace(entry *ManifestEntry) {
	for i, e := range m.Entries {
		if e.Filename == entry.Filename {
			m.Entries[i] = entry
			return
		}
	}
	m.Entries = append(m.Entries, entry)
}

func (m *Manifest) Remove(filename string) {
	var newEntries []*ManifestEntry
	for _, e := range m.Entries {
		if e.Filename != filename {
			newEntries = append(newEntries, e)
		}
	}
	m.Entries = newEntries
}
