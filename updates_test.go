package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseUpdatesDir(t *testing.T) {
	tmpDir := t.TempDir()

	content := `move dev-python/PyICU dev-python/pyicu
move dev-python/python-fastimport dev-python/fastimport
`
	err := os.WriteFile(filepath.Join(tmpDir, "2Q-2021"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	update, err := ParseUpdatesDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(update.Moves) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(update.Moves))
	}
	if update.Moves[0].Old != "dev-python/PyICU" || update.Moves[0].New != "dev-python/pyicu" {
		t.Fatalf("unexpected move 0: %+v", update.Moves[0])
	}
}
