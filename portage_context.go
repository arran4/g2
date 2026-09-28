package g2

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// PortageContext abstracts the capability to execute portageq commands
// to resolve evaluated ebuild metadata on a live Gentoo system.
type PortageContext interface {
	// QueryMetadata runs portageq metadata and returns the authoritative string values for the given keys.
	QueryMetadata(category, pkg, pvr string, keys []string) (map[string]string, error)
}

// OSExecPortageContext uses os/exec to run portageq metadata.
type OSExecPortageContext struct{}

func (c *OSExecPortageContext) QueryMetadata(category, pkg, pvr string, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	// portageq metadata / ebuild sys-devel/gcc-13.2.1_p20231216 HOMEPAGE
	args := []string{"metadata", "/", "ebuild", fmt.Sprintf("%s/%s-%s", category, pkg, pvr)}
	args = append(args, keys...)

	cmd := exec.Command("portageq", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("portageq failed: %w (stderr: %s)", err, stderr.String())
	}

	output := stdout.String()
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")

	if len(lines) != len(keys) {
		return nil, fmt.Errorf("portageq returned %d lines, expected %d", len(lines), len(keys))
	}

	result := make(map[string]string)
	for i, key := range keys {
		result[key] = lines[i]
	}

	return result, nil
}
