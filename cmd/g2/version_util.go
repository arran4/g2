package main

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reVersionRevGlobal = regexp.MustCompile(`-r(\d+)$`)
	reDigitsGlobal     = regexp.MustCompile(`\d+`)
)

func padVersionGlobal(ver string) string {
	if ver == "" {
		return ""
	}

	pVer := reVersionRevGlobal.ReplaceAllString(ver, "+r$1")

	return reDigitsGlobal.ReplaceAllStringFunc(pVer, func(s string) string {
		return fmt.Sprintf("%010s", s)
	})
}

func splitVersionOpGlobal(queryVersion string) (version string, op string) {
	op = "=="
	v := queryVersion
	if strings.HasPrefix(queryVersion, ">=") {
		op = ">="
		v = queryVersion[2:]
	} else if strings.HasPrefix(queryVersion, "<=") {
		op = "<="
		v = queryVersion[2:]
	} else if strings.HasPrefix(queryVersion, ">") {
		op = ">"
		v = queryVersion[1:]
	} else if strings.HasPrefix(queryVersion, "<") {
		op = "<"
		v = queryVersion[1:]
	} else if strings.HasPrefix(queryVersion, "~") {
		op = "~"
		v = queryVersion[1:]
	} else if strings.HasPrefix(queryVersion, "=") {
		op = "="
		v = queryVersion[1:]
	}
	return v, op
}
