package g2

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type PackageMove struct {
	Old string
	New string
}

type PackageSlotMove struct {
	Package string
	Old     string
	New     string
}

type PackageUpdate struct {
	Moves     []PackageMove
	SlotMoves []PackageSlotMove
}

func ParseUpdatesDirFS(sysFS fs.FS, dir string) (*PackageUpdate, error) {
	entries, err := fs.ReadDir(sysFS, dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &PackageUpdate{}, nil
		}
		return nil, err
	}

	update := &PackageUpdate{}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		filePath := filepath.ToSlash(filepath.Join(dir, entry.Name()))
		err := parseUpdateFileFS(sysFS, filePath, update)
		if err != nil {
			return nil, err
		}
	}
	return update, nil
}

func parseUpdateFileFS(sysFS fs.FS, path string, update *PackageUpdate) error {
	f, err := sysFS.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 && parts[0] == "move" {
			update.Moves = append(update.Moves, PackageMove{
				Old: parts[1],
				New: parts[2],
			})
		} else if len(parts) >= 4 && parts[0] == "slotmove" {
			update.SlotMoves = append(update.SlotMoves, PackageSlotMove{
				Package: parts[1],
				Old:     parts[2],
				New:     parts[3],
			})
		}
	}
	return scanner.Err()
}

func ParseUpdatesDir(dir string) (*PackageUpdate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &PackageUpdate{}, nil
		}
		return nil, err
	}

	update := &PackageUpdate{}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		err := parseUpdateFile(filePath, update)
		if err != nil {
			return nil, err
		}
	}
	return update, nil
}

func parseUpdateFile(path string, update *PackageUpdate) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 && parts[0] == "move" {
			update.Moves = append(update.Moves, PackageMove{
				Old: parts[1],
				New: parts[2],
			})
		} else if len(parts) >= 4 && parts[0] == "slotmove" {
			update.SlotMoves = append(update.SlotMoves, PackageSlotMove{
				Package: parts[1],
				Old:     parts[2],
				New:     parts[3],
			})
		}
	}
	return scanner.Err()
}

func WriteUpdatesFile(path string, update *PackageUpdate) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for _, m := range update.Moves {
		if _, err := w.WriteString("move " + m.Old + " " + m.New + "\n"); err != nil {
			return err
		}
	}
	for _, m := range update.SlotMoves {
		if _, err := w.WriteString("slotmove " + m.Package + " " + m.Old + " " + m.New + "\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}
