package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arran4/g2"
)

func TestOverlayEbuildMove_Regression_Issue478(t *testing.T) {
	// Create a temp directory for the test
	tmpDir := t.TempDir()

	// Change working directory to temp directory so getQuarterFile and updatesDir work correctly
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory to temp dir: %v", err)
	}

	// Create updates dir
	updatesDir := filepath.Join("profiles", "updates")
	if err := os.MkdirAll(updatesDir, 0755); err != nil {
		t.Fatalf("failed to create updates directory: %v", err)
	}

	// Create historical update files
	hist1 := "1Q-2023"
	hist1Content := "move hist/old hist/new\n"
	if err := os.WriteFile(filepath.Join(updatesDir, hist1), []byte(hist1Content), 0644); err != nil {
		t.Fatalf("failed to write historical file 1: %v", err)
	}

	hist2 := "2Q-2023"
	hist2Content := "slotmove hist/pkg 1 2\n"
	if err := os.WriteFile(filepath.Join(updatesDir, hist2), []byte(hist2Content), 0644); err != nil {
		t.Fatalf("failed to write historical file 2: %v", err)
	}

	// Determine the current quarter file name
	quarterFile := getQuarterFileForDate(time.Now())

	// Ensure the current quarter file does not match historical names
	if quarterFile == hist1 || quarterFile == hist2 {
		t.Skip("Skipping test because current quarter happens to match historical test files.")
	}

	// Create current quarter file with one existing record
	currContent := "move curr/old curr/new\n"
	if err := os.WriteFile(filepath.Join(updatesDir, quarterFile), []byte(currContent), 0644); err != nil {
		t.Fatalf("failed to write current quarter file: %v", err)
	}

	// Append a new move
	err = appendUpdateFile(time.Now(), &g2.PackageMove{Old: "new/old", New: "new/new"}, nil)
	if err != nil {
		t.Fatalf("appendUpdateFile move failed: %v", err)
	}

	// Append a new slotmove
	err = appendUpdateFile(time.Now(), nil, &g2.PackageSlotMove{Package: "new/pkg", Old: "1", New: "2"})
	if err != nil {
		t.Fatalf("appendUpdateFile slotmove failed: %v", err)
	}

	// Read back current quarter file
	parsedCurr, err := g2.ParseUpdatesFile(filepath.Join(updatesDir, quarterFile))
	if err != nil {
		t.Fatalf("failed to parse updated quarter file: %v", err)
	}

	// Assertions for current file
	if len(parsedCurr.Moves) != 2 {
		t.Fatalf("expected 2 moves in current quarter file, got %d", len(parsedCurr.Moves))
	}
	if parsedCurr.Moves[0].Old != "curr/old" || parsedCurr.Moves[0].New != "curr/new" {
		t.Errorf("unexpected first move: %+v", parsedCurr.Moves[0])
	}
	if parsedCurr.Moves[1].Old != "new/old" || parsedCurr.Moves[1].New != "new/new" {
		t.Errorf("unexpected second move: %+v", parsedCurr.Moves[1])
	}

	if len(parsedCurr.SlotMoves) != 1 {
		t.Fatalf("expected 1 slotmove in current quarter file, got %d", len(parsedCurr.SlotMoves))
	}
	if parsedCurr.SlotMoves[0].Package != "new/pkg" || parsedCurr.SlotMoves[0].Old != "1" || parsedCurr.SlotMoves[0].New != "2" {
		t.Errorf("unexpected slotmove: %+v", parsedCurr.SlotMoves[0])
	}

	// Check historical files to ensure they are untouched
	content, err := os.ReadFile(filepath.Join(updatesDir, hist1))
	if err != nil {
		t.Fatalf("failed to read hist1: %v", err)
	}
	if string(content) != hist1Content {
		t.Errorf("historical file 1 changed! expected %q, got %q", hist1Content, string(content))
	}

	content, err = os.ReadFile(filepath.Join(updatesDir, hist2))
	if err != nil {
		t.Fatalf("failed to read hist2: %v", err)
	}
	if string(content) != hist2Content {
		t.Errorf("historical file 2 changed! expected %q, got %q", hist2Content, string(content))
	}
}

func TestGetQuarterFileForDate(t *testing.T) {
	tests := []struct {
		dateStr string
		want    string
	}{
		{"2024-03-31", "1Q-2024"},
		{"2024-04-01", "2Q-2024"},
		{"2024-06-30", "2Q-2024"},
		{"2024-07-01", "3Q-2024"},
		{"2024-09-30", "3Q-2024"},
		{"2024-10-01", "4Q-2024"},
		{"2024-12-31", "4Q-2024"},
		{"2025-01-01", "1Q-2025"},
		// leap year
		{"2024-02-29", "1Q-2024"},
	}

	for _, tc := range tests {
		date, err := time.Parse(time.DateOnly, tc.dateStr)
		if err != nil {
			t.Fatalf("failed to parse date %s: %v", tc.dateStr, err)
		}
		got := getQuarterFileForDate(date)
		if got != tc.want {
			t.Errorf("getQuarterFileForDate(%s) = %s, want %s", tc.dateStr, got, tc.want)
		}
	}
}

func TestAppendUpdateFile_ConservativeAppend(t *testing.T) {
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()
	_ = os.Chdir(tmpDir)

	updatesDir := filepath.Join("profiles", "updates")
	_ = os.MkdirAll(updatesDir, 0755)

	// Create a file with missing newline at the end and comments
	content := `# historical move
move old/1 new/1

# no newline next`
	date, _ := time.Parse(time.DateOnly, "2024-09-21")
	quarterFile := getQuarterFileForDate(date)
	err := os.WriteFile(filepath.Join(updatesDir, quarterFile), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}

	// Append a new move
	err = appendUpdateFile(date, &g2.PackageMove{Old: "old/2", New: "new/2"}, nil)
	if err != nil {
		t.Fatalf("appendUpdateFile failed: %v", err)
	}

	// Append duplicate move (should be ignored)
	err = appendUpdateFile(date, &g2.PackageMove{Old: "old/2", New: "new/2"}, nil)
	if err != nil {
		t.Fatalf("appendUpdateFile duplicate move failed: %v", err)
	}

	// Append new slotmove
	err = appendUpdateFile(date, nil, &g2.PackageSlotMove{Package: "pkg/1", Old: "1", New: "2"})
	if err != nil {
		t.Fatalf("appendUpdateFile failed: %v", err)
	}

	// Append duplicate slotmove (should be ignored)
	err = appendUpdateFile(date, nil, &g2.PackageSlotMove{Package: "pkg/1", Old: "1", New: "2"})
	if err != nil {
		t.Fatalf("appendUpdateFile duplicate slotmove failed: %v", err)
	}

	finalContent, _ := os.ReadFile(filepath.Join(updatesDir, quarterFile))
	expected := `# historical move
move old/1 new/1

# no newline next
move old/2 new/2
slotmove pkg/1 1 2
`
	if string(finalContent) != expected {
		t.Errorf("File content mismatch.\nExpected:\n%s\nGot:\n%s", expected, string(finalContent))
	}
}

func TestCmdOverlayEbuildMove_InvalidDate(t *testing.T) {
	cfg := &MainArgConfig{}
	err := cfg.cmdOverlayEbuildMove([]string{"--date", "not-a-date", "old/pkg", "new/pkg"})
	if err == nil {
		t.Errorf("Expected error for invalid date, got nil")
	} else if err.Error() != "invalid date format \"not-a-date\", expected YYYY-MM-DD: parsing time \"not-a-date\" as \"2006-01-02\": cannot parse \"not-a-date\" as \"2006\"" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestCmdOverlayEbuildSlotmove_InvalidDate(t *testing.T) {
	cfg := &MainArgConfig{}
	err := cfg.cmdOverlayEbuildSlotmove([]string{"--date", "2024/09/21", "pkg/a", "1", "2"})
	if err == nil {
		t.Errorf("Expected error for invalid date, got nil")
	}
}

func TestCmdOverlayEbuild_Help(t *testing.T) {
	cfg := &MainArgConfig{}
	// ensure no error is returned when help is requested
	err := cfg.cmdOverlayEbuildMove([]string{"-h"})
	if err != nil {
		t.Errorf("Expected nil error for -h, got %v", err)
	}
}
