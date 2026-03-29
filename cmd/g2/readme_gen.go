package main

import (
	"fmt"
	"os/exec"
	"strings"
	"sort"
	"os"
	"path/filepath"
	"text/template"
)

// Attempt to find the template, supporting execution from different directories
func findTemplate() (string, error) {
	candidates := []string{
		"readme.tmpl.md",
		"../../readme.tmpl.md",
		filepath.Join(os.Getenv("GOPATH"), "src/github.com/arran4/g2/readme.tmpl.md"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("could not find readme.tmpl.md")
}

func runCmdHelp(args []string) string {
	// Let's attempt to run the command via the most reliable cross-environment method: `go run .` if in cmd/g2, or `go run ./cmd/g2` if in root.
    var cmdArgs []string
    if _, err := os.Stat("main.go"); err == nil {
        cmdArgs = append([]string{"run", "."}, args...)
    } else {
        cmdArgs = append([]string{"run", "./cmd/g2"}, args...)
    }

	cmd := exec.Command("go", cmdArgs...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func parseSubcommands(output string) map[string]string {
	subs := make(map[string]string)
	lines := strings.Split(output, "\n")
	parsingSubs := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Usage:") {
			parsingSubs = true
			continue
		}
		if parsingSubs && strings.HasPrefix(line, "/") {
			continue
		}
		if parsingSubs {
			parts := strings.Split(line, "\t")
			var cleanParts []string
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cleanParts = append(cleanParts, p)
				}
			}
			if len(cleanParts) >= 2 {
				cmdName := cleanParts[0]
				cmdDesc := cleanParts[1]
				if cmdName != "" && !strings.HasPrefix(cmdName, "/") {
					subs[cmdName] = cmdDesc
				}
			}
		}
	}
	return subs
}

func parseFlags(output string) []string {
	var flags []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimLine, "-") {
			flags = append(flags, trimLine)
		}
	}
	return flags
}

// walkCommandTree recursively walks the help commands discovering nested subcommands
func walkCommandTree(path []string, desc string) string {
    var out string

    depth := len(path)
    heading := strings.Repeat("#", depth+2) // Depth 1 -> ###, Depth 2 -> ####, etc.
    cmdStr := "g2"
    if len(path) > 0 {
        cmdStr += " " + strings.Join(path, " ")
    }

    out += fmt.Sprintf("%s `%s`\n\n", heading, cmdStr)
    if desc != "" {
        out += fmt.Sprintf("%s\n\n", desc)
    }

    cmdHelp := runCmdHelp(append(path, "-help"))
    subCmdsMap := parseSubcommands(cmdHelp)

    // Sort to ensure stable output
    var subKeys []string
    for k := range subCmdsMap {
        subKeys = append(subKeys, k)
    }
    sort.Strings(subKeys)

    if len(subKeys) > 0 {
        // Only print flags for the parent if it has them and it's meaningful, but generally let's just do it for leaves.
        // Or do it for parents too if they have distinct flags.
        flags := parseFlags(cmdHelp)
        if len(flags) > 0 {
            out += fmt.Sprintf("**Flags:**\n```\n%s\n```\n\n", strings.Join(flags, "\n"))
        }

        for _, subK := range subKeys {
            newPath := append([]string{}, path...)
            newPath = append(newPath, subK)
            out += walkCommandTree(newPath, subCmdsMap[subK])
        }
    } else {
        flags := parseFlags(cmdHelp)
        if len(flags) > 0 {
            out += fmt.Sprintf("**Flags:**\n```\n%s\n```\n\n", strings.Join(flags, "\n"))
        }
    }

    return out
}

func (cfg *MainArgConfig) cmdReadmeGen(args []string) error {
	outPath := "readme.md"
	if len(args) > 0 {
		outPath = args[0]
	}

	tmplPath, err := findTemplate()
	if err != nil {
		return fmt.Errorf("finding template: %w", err)
	}

	tmplContent, err := os.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", tmplPath, err)
	}

	tmpl, err := template.New("readme").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

    // Recursively walk root commands
    var commandRef string
	rootHelp := runCmdHelp([]string{"-help"})
	rootCmdsMap := parseSubcommands(rootHelp)
	var rootKeys []string
	for k := range rootCmdsMap {
		rootKeys = append(rootKeys, k)
	}
	sort.Strings(rootKeys)

    for _, k := range rootKeys {
        commandRef += walkCommandTree([]string{k}, rootCmdsMap[k])
    }

	// Prepare template data
	data := map[string]string{
		"CommandReference": strings.TrimSpace(commandRef),
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file %s: %w", outPath, err)
	}
	defer func() { _ = f.Close() }()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	fmt.Printf("Written to %s\n", outPath)
	return nil
}
