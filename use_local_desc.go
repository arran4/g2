package g2

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type UseLocalDesc struct {
	Flags       map[string]map[string]string // pkg -> flag -> desc
	HeaderLines []string
}

func ParseUseLocalDesc(r io.Reader) (*UseLocalDesc, error) {
	ud := &UseLocalDesc{
		Flags:       make(map[string]map[string]string),
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
			pkgFlag := strings.TrimSpace(parts[0])
			desc := strings.TrimSpace(parts[1])

			pkgFlagParts := strings.SplitN(pkgFlag, ":", 2)
			if len(pkgFlagParts) == 2 {
				pkg := strings.TrimSpace(pkgFlagParts[0])
				flag := strings.TrimSpace(pkgFlagParts[1])

				if ud.Flags[pkg] == nil {
					ud.Flags[pkg] = make(map[string]string)
				}
				ud.Flags[pkg][flag] = desc
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ud, nil
}

func ParseUseLocalDescFile(filename string) (*UseLocalDesc, error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &UseLocalDesc{
				Flags: make(map[string]map[string]string),
				HeaderLines: []string{
					"# Copyright 1999-2024 Gentoo Authors",
					"# Distributed under the terms of the GNU General Public License v2",
				},
			}, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseUseLocalDesc(file)
}

func (ud *UseLocalDesc) Write(w io.Writer) error {
	for _, line := range ud.HeaderLines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}

	var pkgs []string
	for pkg := range ud.Flags {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	for _, pkg := range pkgs {
		var flags []string
		for flag := range ud.Flags[pkg] {
			flags = append(flags, flag)
		}
		sort.Strings(flags)

		for _, flag := range flags {
			desc := ud.Flags[pkg][flag]
			if _, err := fmt.Fprintf(w, "%s:%s - %s\n", pkg, flag, desc); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ud *UseLocalDesc) WriteFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return ud.Write(file)
}
