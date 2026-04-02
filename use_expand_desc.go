package g2

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type UseExpandDesc struct {
	Name  string
	Flags map[string]string
	Lines []DescLine // ordered list of lines
}

type DescLine struct {
	Text string
	Flag string // empty if it's a comment or blank line
}

func ParseUseExpandDesc(r io.Reader, name string) (*UseExpandDesc, error) {
	ud := &UseExpandDesc{
		Name:  name,
		Flags: make(map[string]string),
	}

	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			ud.Lines = append(ud.Lines, DescLine{Text: line})
			continue
		}

		parts := strings.SplitN(trimmed, " - ", 2)
		if len(parts) == 2 {
			flag := strings.TrimSpace(parts[0])
			desc := strings.TrimSpace(parts[1])
			ud.Flags[flag] = desc
			ud.Lines = append(ud.Lines, DescLine{Flag: flag})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ud, nil
}
func ParseUseExpandDescFile(filename string, name string) (*UseExpandDesc, error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &UseExpandDesc{
				Name:  name,
				Flags: make(map[string]string),
				Lines: []DescLine{
					{Text: "# Copyright 1999-2024 Gentoo Authors"},
					{Text: "# Distributed under the terms of the GNU General Public License v2"},
				},
			}, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseUseExpandDesc(file, name)
}

func ParseUseExpandDescDir(dir string) (map[string]*UseExpandDesc, error) {
	return ParseUseExpandDescFS(os.DirFS(dir), ".")
}

func ParseUseExpandDescFS(sysFS fs.FS, dir string) (map[string]*UseExpandDesc, error) {
	result := make(map[string]*UseExpandDesc)

	entries, err := fs.ReadDir(sysFS, filepath.ToSlash(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".desc") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".desc")
		path := filepath.Join(dir, entry.Name())

		file, err := sysFS.Open(filepath.ToSlash(path))
		if err != nil {
			return nil, err
		}

		ud, err := ParseUseExpandDesc(file, name)
		_ = file.Close()

		if err != nil {
			return nil, err
		}

		result[name] = ud
	}

	return result, nil
}

func (ud *UseExpandDesc) Write(w io.Writer) error {
	writtenFlags := make(map[string]bool)
	for _, line := range ud.Lines {
		if line.Flag != "" {
			if desc, ok := ud.Flags[line.Flag]; ok {
				if _, err := fmt.Fprintf(w, "%s - %s\n", line.Flag, desc); err != nil {
					return err
				}
				writtenFlags[line.Flag] = true
			}
		} else {
			if _, err := fmt.Fprintln(w, line.Text); err != nil {
				return err
			}
		}
	}

	// Write any new flags that were added programmatically
	var newFlags []string
	for flag := range ud.Flags {
		if !writtenFlags[flag] {
			newFlags = append(newFlags, flag)
		}
	}
	sort.Strings(newFlags)
	for _, flag := range newFlags {
		if _, err := fmt.Fprintf(w, "%s - %s\n", flag, ud.Flags[flag]); err != nil {
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
