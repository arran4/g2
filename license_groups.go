package g2

import (
	"bufio"
	"io"
	"strings"
)

// ParseLicenseGroups parses a profiles/license_groups file.
// It returns a map where the key is the alias/group name, and the value
// is a list of target licenses that map to it.
func ParseLicenseGroups(r io.Reader) (map[string][]string, error) {
	scanner := bufio.NewScanner(r)
	groups := make(map[string][]string)

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		parts := strings.Fields(trimmedLine)
		if len(parts) >= 2 {
			group := parts[0]
			licenses := parts[1:]
			groups[group] = append(groups[group], licenses...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return groups, nil
}
