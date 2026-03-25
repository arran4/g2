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
	var newEntries []LayoutConfEntry
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
