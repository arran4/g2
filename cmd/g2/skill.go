package main

import (
	"flag"
	"fmt"
)

func (c *MainArgConfig) cmdSkill(args []string) error {
	fs := flag.NewFlagSet("skill", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: %s skill <subcommand> [flags]\n", c.Args[0])
		fmt.Printf("Subcommands:\n")
		fmt.Printf("\tinstall\t\tInstall a new agent skill\n")
		fmt.Printf("\tupdate\t\tUpdate an installed skill\n")
		fmt.Printf("\tremove\t\tRemove an installed skill\n")
		fmt.Printf("\tlist\t\tList installed skills\n")
		fmt.Printf("\tinspect\t\tInspect an installed skill\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return nil
	}

	subCmd := fs.Arg(0)
	subArgs := fs.Args()[1:]

	switch subCmd {
	case "install":
		return c.cmdSkillInstall(subArgs)
	case "update":
		return c.cmdSkillUpdate(subArgs)
	case "remove":
		return c.cmdSkillRemove(subArgs)
	case "list":
		return c.cmdSkillList(subArgs)
	case "inspect":
		return c.cmdSkillInspect(subArgs)
	default:
		return fmt.Errorf("unknown skill subcommand: %s", subCmd)
	}
}
