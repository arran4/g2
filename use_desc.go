package g2

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type UseDesc struct {
	Flags       map[string]string
	HeaderLines []string
}

func ParseUseDesc(r io.Reader) (*UseDesc, error) {
	ud := &UseDesc{
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

func ParseUseDescFile(filename string) (*UseDesc, error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &UseDesc{
				Flags: make(map[string]string),
				HeaderLines: []string{
					"# Copyright 1999-2024 Gentoo Authors",
					"# Distributed under the terms of the GNU General Public License v2",
				},
			}, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseUseDesc(file)
}

func (ud *UseDesc) Write(w io.Writer) error {
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

func (ud *UseDesc) WriteFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return ud.Write(file)
}
