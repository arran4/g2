package main

import (
	"fmt"
	"strings"

	"github.com/arran4/g2/lints"
)

func escapeGithubActions(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

func printGithubActionsFormat(warnings []lints.LintResult) {
	for _, w := range warnings {
		sev := "notice"
		switch w.RuleMetadata.Severity {
		case lints.SeverityError:
			sev = "error"
		case lints.SeverityWarning:
			sev = "warning"
		}

		parts := []string{}
		if w.File != "" {
			parts = append(parts, fmt.Sprintf("file=%s", w.File))
		}
		if w.Line > 0 {
			parts = append(parts, fmt.Sprintf("line=%d", w.Line))
		}
		if w.RuleMetadata.Title != "" {
			parts = append(parts, fmt.Sprintf("title=%s", escapeGithubActions(w.RuleMetadata.Title)))
		}

		msg := escapeGithubActions(w.Message)

		if len(parts) > 0 {
			fmt.Printf("::%s %s::%s\n", sev, strings.Join(parts, ","), msg)
		} else {
			fmt.Printf("::%s::%s\n", sev, msg)
		}
	}
}
