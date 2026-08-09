package g2

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// PkgDescIndexEntry represents a single entry in the pkg_desc_index file.
type PkgDescIndexEntry struct {
	Category    string
	Package     string
	Versions    []string
	Description string
}

// PkgDescIndex represents the parsed content of a pkg_desc_index file.
type PkgDescIndex struct {
	Entries []PkgDescIndexEntry
}

// ParsePkgDescIndex reads and parses the pkg_desc_index file from an io.Reader.
func ParsePkgDescIndex(r io.Reader) (*PkgDescIndex, error) {
	var index PkgDescIndex
	scanner := bufio.NewScanner(r)

	// Optional: You could read the whole file, but line-by-line is fine.
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Expected format: "category/package version1 version2: Description"
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: invalid format (missing ': ' separator)", lineNum)
		}

		pkgAndVers := strings.Fields(parts[0])
		if len(pkgAndVers) < 1 {
			return nil, fmt.Errorf("line %d: invalid format (missing package name)", lineNum)
		}

		pkgPath := pkgAndVers[0]
		pathParts := strings.SplitN(pkgPath, "/", 2)
		if len(pathParts) != 2 {
			return nil, fmt.Errorf("line %d: invalid package format (expected category/package)", lineNum)
		}

		entry := PkgDescIndexEntry{
			Category:    pathParts[0],
			Package:     pathParts[1],
			Versions:    pkgAndVers[1:],
			Description: parts[1],
		}

		index.Entries = append(index.Entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning input: %w", err)
	}

	return &index, nil
}

// ParsePkgDescIndexFile reads and parses a pkg_desc_index file from disk.
func ParsePkgDescIndexFile(path string) (*PkgDescIndex, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() { _ = file.Close() }()
	return ParsePkgDescIndex(file)
}

// Serialize formats the PkgDescIndex into the expected text format.
func (idx *PkgDescIndex) Serialize(w io.Writer) error {
	// Typically, the entries in the index should be sorted by category/package
	// The specification doesn't strictly say so, but gentoo-mirror's pkg_desc_index is sorted alphabetically

	for _, entry := range idx.Entries {
		pkgPath := fmt.Sprintf("%s/%s", entry.Category, entry.Package)

		var line string
		if len(entry.Versions) > 0 {
			versionsStr := strings.Join(entry.Versions, " ")
			line = fmt.Sprintf("%s %s: %s\n", pkgPath, versionsStr, entry.Description)
		} else {
			// This might be an invalid state, but we'll serialize what we have
			line = fmt.Sprintf("%s: %s\n", pkgPath, entry.Description)
		}

		if _, err := fmt.Fprint(w, line); err != nil {
			return fmt.Errorf("writing entry %s: %w", pkgPath, err)
		}
	}
	return nil
}

// Save serializes the PkgDescIndex and writes it to the specified file path.
func (idx *PkgDescIndex) Save(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	err = idx.Serialize(file)
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("closing file: %w", closeErr)
	}

	if err != nil {
		return fmt.Errorf("serializing index: %w", err)
	}

	return nil
}

// Sort sorts the entries alphabetically by category and package.
func (idx *PkgDescIndex) Sort() {
	sort.Slice(idx.Entries, func(i, j int) bool {
		ei, ej := idx.Entries[i], idx.Entries[j]
		if ei.Category == ej.Category {
			return ei.Package < ej.Package
		}
		return ei.Category < ej.Category
	})
}
