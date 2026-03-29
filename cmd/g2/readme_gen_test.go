package main

import (
	"os"
	"strings"
	"testing"
)

func TestReadmeGen(t *testing.T) {
	// Let's output to a temporary file instead of stdout to avoid pipe blocking
	tmpDir := t.TempDir()
	outPath := tmpDir + "/readme.md"

	cfg := &MainArgConfig{}
	err := cfg.cmdReadmeGen([]string{outPath})
	if err != nil {
		t.Fatalf("cmdReadmeGen failed: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("Failed to read generated readme: %v", err)
	}
	output := string(content)

	requiredStrings := []string{
		"# g2",
		"## Modules & Commands",
		"### `manifest`",
		"### `ebuild`",
		"### `metadata`",
		"### `overlay site generate`",
		"### `use`",
		"### `cache`",
		"### `package`",
		"go run ./cmd/g2 readme-gen", // maintainer note
	}

	for _, req := range requiredStrings {
		if !strings.Contains(output, req) {
			t.Errorf("Generated readme is missing required string: %q", req)
		}
	}
}

func TestRenderModel(t *testing.T) {
	model := DocModel{
		Name:        "testcmd",
		Description: "A test command.",
		Usage:       "g2 testcmd [flags]",
		Args:        []string{"`arg1`: description"},
		Flags:       []string{"`-flag1`: A flag"},
		Subcommands: []DocModel{
			{Name: "sub", Description: "sub desc"},
		},
		Examples: []string{"g2 testcmd\n"},
	}

	output := renderModel(model, 3)

	requiredStrings := []string{
		"### `testcmd`",
		"A test command.",
		"**Usage:**\n\n```bash\ng2 testcmd [flags]\n```\n",
		"**Arguments:**\n\n* `arg1`: description\n",
		"**Flags:**\n\n* `-flag1`: A flag\n",
		"**Subcommands:**\n\n* `sub`: sub desc\n",
		"**Example:**\n\ng2 testcmd\n",
	}

	for _, req := range requiredStrings {
		if !strings.Contains(output, req) {
			t.Errorf("renderModel is missing required string: %q\nOutput: %s", req, output)
		}
	}
}
