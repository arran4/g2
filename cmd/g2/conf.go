package main

import (
	"fmt"
)

func (cfg *MainArgConfig) cmdConf(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing subcommand for conf")
	}

	subcmd := args[0]
	switch subcmd {
	case "all":
		return cfg.cmdConfAll(args[1:])
	default:
		return fmt.Errorf("unknown conf subcommand: %s", subcmd)
	}
}
