package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdMakeConf(args []string) error {
	fs := flag.NewFlagSet("make-conf", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 make-conf <subcommand>|<key>\n")
		fmt.Printf("\ttui\t\t\tOpen terminal UI to manage make.conf\n")
		fmt.Printf("\t<key>\t\t\tGet the value of a key in make.conf\n")
	}

	locationOpt := fs.String("location", "/etc/portage/make.conf", "Path to make.conf file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing key or subcommand for make-conf")
	}

	subcmd := fs.Arg(0)

	if subcmd == "tui" {
		return cfg.cmdMakeConfTUI(*locationOpt)
	}

	key := subcmd

	vars, err := g2.ParseMakeConf(*locationOpt)
	if err != nil {
		return fmt.Errorf("parsing make.conf: %w", err)
	}

	if val, ok := vars[key]; ok {
		fmt.Println(val)
	} else {
		// Just be silent or empty output if not found, like typical get operations?
		// Portages `portageq envvar` just prints empty string if not found.
		fmt.Println("")
	}

	return nil
}

func (cfg *MainArgConfig) cmdMakeConfTUI(location string) error {
	content, err := os.ReadFile(location)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading make.conf: %w", err)
		}
		content = []byte{}
	}

	lines := strings.Split(string(content), "\n")
	return runConfTUI(location, lines, "make.conf")
}
