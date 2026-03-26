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
slotmove dev-build/autoconf-dickey 2.52_p20210509 2.52
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
	if len(update.SlotMoves) != 1 {
		t.Fatalf("expected 1 slotmove, got %d", len(update.SlotMoves))
	}
	if update.SlotMoves[0].Package != "dev-build/autoconf-dickey" || update.SlotMoves[0].Old != "2.52_p20210509" || update.SlotMoves[0].New != "2.52" {
		t.Fatalf("unexpected slotmove 0: %+v", update.SlotMoves[0])
	}
}

func TestWriteUpdatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "3Q-2024")

	update := &PackageUpdate{
		Moves: []PackageMove{
			{Old: "old/pkg", New: "new/pkg"},
		},
		SlotMoves: []PackageSlotMove{
			{Package: "my/pkg", Old: "1", New: "2"},
		},
	}

	err := WriteUpdatesFile(path, update)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	expected := "move old/pkg new/pkg\nslotmove my/pkg 1 2\n"
	if string(content) != expected {
		t.Fatalf("expected %q, got %q", expected, string(content))
	}
}
