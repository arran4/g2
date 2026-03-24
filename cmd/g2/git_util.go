package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func getGitOriginURL(repoDir string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting remote origin url: %w", err)
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("remote.origin.url is empty")
	}
	return url, nil
}

func getFileCommit(repoDir string, filepath string) (string, error) {
	cmd := exec.Command("git", "log", "-n", "1", "--pretty=format:%H", "--", filepath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting file commit: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func generateGitHubRawURL(remoteURL, commitHash, relativeFilepath string) string {
	// e.g. https://github.com/arran4/arrans_overlay.git
	// e.g. git@github.com:arran4/arrans_overlay.git
	// Output: https://github.com/arran4/arrans_overlay/blob/<commitHash>/<relativeFilepath>

	url := strings.TrimSuffix(remoteURL, ".git")

	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	} else if strings.HasPrefix(url, "git@gitlab.com:") {
		url = strings.Replace(url, "git@gitlab.com:", "https://gitlab.com/", 1)
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// Just a fallback if it doesn't match standard prefixes
		return ""
	}

	return fmt.Sprintf("%s/blob/%s/%s", url, commitHash, relativeFilepath)
}
