package g2

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type UseExpandDesc struct {
	Prefix      string
	Flags       map[string]string // flag -> desc
	HeaderLines []string
}

func ParseUseExpandDesc(prefix string, r io.Reader) (*UseExpandDesc, error) {
	ud := &UseExpandDesc{
		Prefix:      prefix,
		Flags:       make(map[string]string),
		HeaderLines: make([]string, 0),
	}

	scanner := bufio.NewScanner(r)
	inHeader := true

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if inHeader {
				ud.HeaderLines = append(ud.HeaderLines, line)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			if inHeader {
				ud.HeaderLines = append(ud.HeaderLines, line)
			}
			continue
		}

		inHeader = false

		parts := strings.SplitN(trimmed, " - ", 2)
		if len(parts) == 2 {
			flag := strings.TrimSpace(parts[0])
			desc := strings.TrimSpace(parts[1])
			ud.Flags[flag] = desc
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ud, nil
}

func ParseUseExpandDescFile(filename string) (*UseExpandDesc, error) {
	prefix := strings.TrimSuffix(filepath.Base(filename), ".desc")
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &UseExpandDesc{
				Prefix: prefix,
				Flags:  make(map[string]string),
				HeaderLines: []string{
					"# Copyright 1999-2024 Gentoo Authors",
					"# Distributed under the terms of the GNU General Public License v2",
				},
			}, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseUseExpandDesc(prefix, file)
}

func (ud *UseExpandDesc) Write(w io.Writer) error {
	for _, line := range ud.HeaderLines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}

	var flags []string
	for k := range ud.Flags {
		flags = append(flags, k)
	}
	sort.Strings(flags)

	for _, flag := range flags {
		desc := ud.Flags[flag]
		if _, err := fmt.Fprintf(w, "%s - %s\n", flag, desc); err != nil {
			return err
		}
	}

	return nil
}

func (ud *UseExpandDesc) WriteFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return ud.Write(file)
}

// ParseUseExpandDescDir parses all .desc files in a directory (e.g., profiles/desc/)
func ParseUseExpandDescDir(dir string) (map[string]*UseExpandDesc, error) {
	result := make(map[string]*UseExpandDesc)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil // Return empty map if dir doesn't exist
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".desc") {
			filename := filepath.Join(dir, entry.Name())
			ud, err := ParseUseExpandDescFile(filename)
			if err != nil {
				return nil, fmt.Errorf("parsing %s: %w", filename, err)
			}
			result[ud.Prefix] = ud
		}
	}

	return result, nil
}
