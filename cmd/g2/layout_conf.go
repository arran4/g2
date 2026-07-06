package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdLayoutConf(args []string) error {
	fs := flag.NewFlagSet("layout-conf", flag.ExitOnError)
	repoDir := fs.String("repo", ".", "Path to the repository root")
	if err := fs.Parse(args); err != nil {
		return err
	}

	remainingArgs := fs.Args()
	if len(remainingArgs) < 1 {
		return fmt.Errorf("missing subcommand for layout-conf (e.g., get, set, unset)")
	}

	layoutConfPath := filepath.Join(*repoDir, "metadata", "layout.conf")
	var lc *g2.LayoutConf
	var err error

	if _, statErr := os.Stat(layoutConfPath); os.IsNotExist(statErr) {
		lc = &g2.LayoutConf{}
	} else {
		lc, err = g2.ParseLayoutConf(layoutConfPath)
		if err != nil {
			return fmt.Errorf("failed to parse layout.conf: %w", err)
		}
	}

	element := remainingArgs[0]

	if element == "list" {
		for _, entry := range lc.Entries {
			for _, comment := range entry.Comments {
				if comment != "" {
					fmt.Println(comment)
				}
			}
			fmt.Printf("%s = %s\n", entry.Key, entry.Value)
		}
		return nil
	}

	if len(remainingArgs) < 2 {
		return fmt.Errorf("missing subcommand for element %s (e.g., get, set, unset)", element)
	}

	subcmd := remainingArgs[1]
	switch subcmd {
	case "get":
		val := lc.GetValue(element)
		if val != "" {
			fmt.Println(val)
		}
	case "set":
		if len(remainingArgs) < 3 {
			return fmt.Errorf("missing value to set for %s", element)
		}
		value := strings.Join(remainingArgs[2:], " ")
		lc.SetValue(element, value)
		if err := os.MkdirAll(filepath.Dir(layoutConfPath), 0755); err != nil {
			return fmt.Errorf("creating metadata dir: %w", err)
		}
		if err := g2.WriteLayoutConf(lc, layoutConfPath); err != nil {
			return fmt.Errorf("writing layout.conf: %w", err)
		}
	case "unset":
		lc.UnsetValue(element)
		if err := g2.WriteLayoutConf(lc, layoutConfPath); err != nil {
			return fmt.Errorf("writing layout.conf: %w", err)
		}
	default:
		return fmt.Errorf("unknown action for %s: %s", element, subcmd)
	}

	return nil
}
