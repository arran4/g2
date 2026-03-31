package g2

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type ArchList struct {
	Arches []string
}

func ParseArchList(r io.Reader) (*ArchList, error) {
	al := &ArchList{}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		al.Arches = append(al.Arches, trimmed)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return al, nil
}

func ParseArchListFile(filename string) (*ArchList, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseArchList(file)
}

type ArchesDesc struct {
	Arches      map[string]string
	HeaderLines []string
}

func ParseArchesDesc(r io.Reader) (*ArchesDesc, error) {
	ad := &ArchesDesc{
		Arches:      make(map[string]string),
		HeaderLines: make([]string, 0),
	}

	scanner := bufio.NewScanner(r)
	inHeader := true

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if inHeader {
				ad.HeaderLines = append(ad.HeaderLines, line)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			if inHeader {
				ad.HeaderLines = append(ad.HeaderLines, line)
			}
			continue
		}

		inHeader = false

		parts := strings.Fields(trimmed)
		if len(parts) >= 2 {
			arch := strings.TrimSpace(parts[0])
			status := strings.TrimSpace(parts[1])
			ad.Arches[arch] = status
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return ad, nil
}

func ParseArchesDescFile(filename string) (*ArchesDesc, error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &ArchesDesc{
				Arches: make(map[string]string),
				HeaderLines: []string{
					"# Copyright 1999-2024 Gentoo Authors",
					"# Distributed under the terms of the GNU General Public License v2",
				},
			}, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseArchesDesc(file)
}

func (ad *ArchesDesc) Write(w io.Writer) error {
	for _, line := range ad.HeaderLines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}

	var arches []string
	for k := range ad.Arches {
		arches = append(arches, k)
	}
	sort.Strings(arches)

	for _, arch := range arches {
		status := ad.Arches[arch]
		if _, err := fmt.Fprintf(w, "%-15s %s\n", arch, status); err != nil {
			return err
		}
	}

	return nil
}

func (ad *ArchesDesc) WriteFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return ad.Write(file)
}
