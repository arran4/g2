package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func (cfg *MainArgConfig) cmdWorld(args []string) error {
	fs := flag.NewFlagSet("world", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "tui", "Open the terminal UI to manage the world file")
		fmt.Printf("\t\t %s \t\t %s\n", "list", "List all entries in the world file")
		fmt.Printf("\t\t %s \t\t %s\n", "query", "Query entries in the world file")
		fmt.Printf("\t\t %s \t\t %s\n", "delete", "Delete an entry from the world file")
		fmt.Printf("\t\t %s \t\t %s\n", "add", "Add an entry to the world file")
		fmt.Printf("\t\t %s \t\t %s\n", "enable", "Enable (uncomment) an entry in the world file")
		fmt.Printf("\t\t %s \t\t %s\n", "disable", "Disable (comment) an entry in the world file")
	}

	locationOpt := fs.String("location", "/var/lib/portage/world", "Path to world file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

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

	switch cmd {
	case "tui":
		return runWorldTUI(path, lines)
	case "list":
		for _, line := range lines {
			fmt.Println(line)
		}
		return nil
	case "query":
		queryArgs := fs.Args()[1:]
		if len(queryArgs) == 0 {
			return fmt.Errorf("missing query string")
		}
		q := queryArgs[0]
		for _, line := range lines {
			if strings.Contains(line, q) {
				fmt.Println(line)
			}
		}
		return nil
	case "delete":
		delArgs := fs.Args()[1:]
		if len(delArgs) == 0 {
			return fmt.Errorf("missing package to delete")
		}
		pkg := delArgs[0]
		newLines := []string{}
		for _, line := range lines {
			if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#")) != pkg {
				newLines = append(newLines, line)
			}
		}
		return writeWorldFile(path, newLines)
	case "add":
		addArgs := fs.Args()[1:]
		if len(addArgs) == 0 {
			return fmt.Errorf("missing package to add")
		}
		pkg := addArgs[0]
		lines = append(lines, pkg)
		return writeWorldFile(path, lines)
	case "enable":
		enableArgs := fs.Args()[1:]
		if len(enableArgs) == 0 {
			return fmt.Errorf("missing package to enable")
		}
		pkg := enableArgs[0]
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") && strings.TrimSpace(strings.TrimPrefix(trimmed, "#")) == pkg {
				lines[i] = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			}
		}
		return writeWorldFile(path, lines)
	case "disable":
		disableArgs := fs.Args()[1:]
		if len(disableArgs) == 0 {
			return fmt.Errorf("missing package to disable")
		}
		pkg := disableArgs[0]
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "#") && trimmed == pkg {
				lines[i] = "# " + line
			}
		}
		return writeWorldFile(path, lines)
	default:
		fs.Usage()
		return fmt.Errorf("unknown subcommand: %s", cmd)
	}
}

func readWorldFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeWorldFile(path string, lines []string) error {
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
