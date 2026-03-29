package main

import (
    "testing"
    "os"
    "path/filepath"
    "strings"
)

func TestReadmeGen(t *testing.T) {
    cfg := &MainArgConfig{}

    tmpDir := t.TempDir()
    outPath := filepath.Join(tmpDir, "test_readme.md")

    // We expect the generator to execute cleanly
    err := cfg.cmdReadmeGen([]string{outPath})
    if err != nil {
        t.Fatalf("readme-gen failed: %v", err)
    }

    // Test if file exists and has content
    content, err := os.ReadFile(outPath)
    if err != nil {
        t.Fatalf("Failed to read generated readme: %v", err)
    }

    if len(content) == 0 {
        t.Fatalf("Generated readme is empty")
    }

    // Ensure template wasn't just completely blank or just had minimal text.
    // E.g. make sure our template rendered in the real text like `## Quick Start`
    contentStr := string(content)
    if !strings.Contains(contentStr, "## Quick Start") {
        t.Fatalf("Generated readme is missing template contents (e.g. Quick Start block)")
    }

    // Ensure command ref generated successfully by looking for root commands.
    if !strings.Contains(contentStr, "### `g2 overlay`") {
        t.Fatalf("Generated readme is missing the generated command references.")
    }
}
