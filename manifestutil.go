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
