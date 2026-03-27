package g2

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

// ParseInfoVarsReader parses a stream containing info_vars data,
// ignoring empty lines and lines starting with '#'.
func parseInfoVarsReader(r io.Reader) ([]string, error) {
	var results []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		results = append(results, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading info_vars: %w", err)
	}

	return results, nil
}

// ParseInfoVarsFS parses a profiles/info_vars file from a filesystem.
func ParseInfoVarsFS(sysFS fs.FS, path string) ([]string, error) {
	f, err := sysFS.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return parseInfoVarsReader(f)
}

// ParseInfoVars parses a profiles/info_vars file from the local filesystem.
func ParseInfoVars(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	return parseInfoVarsReader(f)
}

// SerializeInfoVars writes a list of variables to an io.Writer.
func SerializeInfoVars(w io.Writer, vars []string) error {
	for _, v := range vars {
		if _, err := fmt.Fprintln(w, v); err != nil {
			return err
		}
	}
	return nil
}

// WriteInfoVarsFile writes a list of variables to a file on the local filesystem.
func WriteInfoVarsFile(path string, vars []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return SerializeInfoVars(f, vars)
}
