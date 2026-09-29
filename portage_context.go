package g2

import (
	"fmt"
	"strings"
)

// PortageContext abstracts the capability to execute portageq commands
// to resolve evaluated ebuild metadata on a live Gentoo system.
type PortageContext interface {
	// QueryMetadata runs portageq metadata and returns the authoritative string values for the given keys.
	// The repo context can be supplied to form a repo-qualified package specifier (e.g. cat/pkg::repo).
	QueryMetadata(repo, category, pkg, pvr string, keys []string) (map[string]string, error)
}

// OSExecPortageContext uses os/exec to run portageq metadata.
type OSExecPortageContext struct {
	Runner CmdRunner
}

func NewOSExecPortageContext() *OSExecPortageContext {
	return &OSExecPortageContext{Runner: &OSExecRunner{}}
}

func (c *OSExecPortageContext) QueryMetadata(repo, category, pkg, pvr string, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	cpv := fmt.Sprintf("%s/%s-%s", category, pkg, pvr)
	if repo != "" {
		cpv += "::" + repo
	}

	// portageq metadata / ebuild sys-devel/gcc-13.2.1_p20231216 HOMEPAGE
	args := []string{"metadata", "/", "ebuild", cpv}
	args = append(args, keys...)

	stdout, stderr, err := c.Runner.Run("portageq", args...)
	if err != nil {
		if err == ErrPortageUnavailable {
			return nil, err
		}
		return nil, fmt.Errorf("portageq failed: %w (stderr: %s)", err, string(stderr))
	}

	output := string(stdout)
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
