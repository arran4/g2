package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func (cfg *MainArgConfig) CmdVersions(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: versions <subcommand>")
	}

	subcmd := args[0]
	switch subcmd {
	case "convert":
		if len(args) < 3 {
			return fmt.Errorf("usage: versions convert <semantic-to-gentoo|gentoo-to-semantic> <version string | ->")
		}
		mode := args[1]
		input := args[2]

		var processFunc func(string) string
		switch mode {
		case "semantic-to-gentoo":
			processFunc = SemanticToGentoo
		case "gentoo-to-semantic":
			processFunc = GentooToSemantic
		default:
			return fmt.Errorf("unknown convert mode: %s, must be semantic-to-gentoo or gentoo-to-semantic", mode)
		}

		if input == "-" {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				fmt.Println(processFunc(scanner.Text()))
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error reading stdin: %w", err)
			}
		} else {
			fmt.Println(processFunc(input))
		}
	default:
		return fmt.Errorf("unknown versions subcommand: %s", subcmd)
	}

	return nil
}

// SemanticToGentoo converts a semantic version string to a Gentoo version string.
func SemanticToGentoo(v string) string {
	// Pattern 1: Base -rX pre-release
	rePrerelease := regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)*(?:[a-z])?)(?:-r([0-9]+))?(?:[-_](alpha|beta|pre|rc|p)([0-9]*))?$`)
	// Pattern 2: Base pre-release -rX
	rePrerelease2 := regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)*(?:[a-z])?)(?:[-_](alpha|beta|pre|rc|p)([0-9]*))?(?:-r([0-9]+))?$`)

	match := rePrerelease.FindStringSubmatch(v)
	if match != nil && (match[2] != "" || match[3] != "") {
		// Used Pattern 1
	} else {
		match = rePrerelease2.FindStringSubmatch(v)
		if match != nil {
			// Rearrange to match Pattern 1 indexes: [all, base, rev, preType, preNum]
			match = []string{match[0], match[1], match[4], match[2], match[3]}
		}
	}

	if match != nil {
		base := match[1]
		rev := match[2]
		preType := match[3]
		preNum := match[4]

		res := base
		if preType != "" {
			res += "_" + preType + preNum
		}
		if rev != "" {
			res += "-r" + rev
		}
		return res
	}

	// Fallback, return as is
	return v
}

// GentooToSemantic converts a Gentoo version string to a semantic version string.
func GentooToSemantic(v string) string {
	// Pattern 1: Base pre-release -rX
	reGentoo := regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)*(?:[a-z])?)(?:_([a-z]+)([0-9]*))?(?:-r([0-9]+))?$`)
	// Pattern 2: Base -rX pre-release
	reGentoo2 := regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)*(?:[a-z])?)(?:-r([0-9]+))?(?:_([a-z]+)([0-9]*))?$`)

	match := reGentoo.FindStringSubmatch(v)
	if match != nil && (match[2] != "" || match[4] != "") {
		// Used Pattern 1
	} else {
		match = reGentoo2.FindStringSubmatch(v)
		if match != nil {
			// Rearrange to match Pattern 1 indexes: [all, base, preType, preNum, rev]
			match = []string{match[0], match[1], match[3], match[4], match[2]}
		}
	}

	if match != nil {
		base := match[1]
		preType := match[2]
		preNum := match[3]
		rev := match[4]

		res := base
		if rev != "" {
			res += "-r" + rev
		}
		if preType != "" {
			res += "-" + preType + preNum
		}
		return res
	}

	// Fallback, return as is
	return strings.ReplaceAll(v, "_", "-")
}
