package g2

import (
	"io/fs"
	"strings"

	"golang.org/x/tools/txtar"
	"testing/fstest"
)

func WalkFiles(root fs.FS, dir string) ([]string, error) {
	var files []string
	err := fs.WalkDir(root, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func SplitInputExpected(ar *txtar.Archive) (input, expected fstest.MapFS) {
	input = fstest.MapFS{}
	expected = fstest.MapFS{}

	for _, f := range ar.Files {
		switch {
		case strings.HasPrefix(f.Name, "input/"):
			input[strings.TrimPrefix(f.Name, "input/")] = &fstest.MapFile{Data: f.Data}
		case strings.HasPrefix(f.Name, "expected/"):
			expected[strings.TrimPrefix(f.Name, "expected/")] = &fstest.MapFile{Data: f.Data}
		}
	}
	return input, expected
}
