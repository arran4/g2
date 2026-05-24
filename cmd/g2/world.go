package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

func (cfg *MainArgConfig) cmdWorld(args []string) error {
	fs := flag.NewFlagSet("world", flag.ExitOnError)
	locationOpt := fs.String("location", "/var/lib/portage/world", "Path to world file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	path := *locationOpt

	// Read world file
	lines, err := readWorldFile(path)
	if err != nil {
		// If file doesn't exist, we can start empty
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading world file: %w", err)
		}
		lines = []string{}
	}

	return runWorldTUI(path, lines)
}

func readWorldFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeWorldFile(path string, lines []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}
