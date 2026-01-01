package g2

import (
	"os"
	"slices"
	"strings"
)

func UpsertManifest(manifestPath, filename, manifestLine string) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(manifestPath, []byte(manifestLine+"\n"), 0644)
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	lines = slices.DeleteFunc(lines, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "DIST "+filename+" ") {
			lines[i] = manifestLine
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, manifestLine)
	}

	return os.WriteFile(manifestPath, []byte(strings.Join(lines, "\n")), 0644)
}

// ReadManifest reads the manifest file and returns a map of filename to manifest line.
func ReadManifest(manifestPath string) (map[string]string, error) {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	manifest := make(map[string]string)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == "DIST" {
			filename := parts[1]
			manifest[filename] = line
		}
	}
	return manifest, nil
}
