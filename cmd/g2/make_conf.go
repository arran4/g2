package main

import (
	"flag"
	"fmt"

	"github.com/arran4/g2"
)

func (cfg *MainArgConfig) cmdMakeConf(args []string) error {
	fs := flag.NewFlagSet("make-conf", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: g2 make-conf <key>\n")
	}

	locationOpt := fs.String("location", "/etc/portage/make.conf", "Path to make.conf file")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing key for make-conf")
	}

	key := fs.Arg(0)

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
