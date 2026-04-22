package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMakeConf(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "make.conf")

	err := os.WriteFile(confPath, []byte(`
VAR1="hello"
VAR2='world'
VAR3="${VAR1} ${VAR2}"
export VAR1 VAR2 VAR3
	`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	vars, err := ParseMakeConf(confPath)
	if err != nil {
		t.Fatal(err)
	}

	if vars["VAR1"] != "hello" || vars["VAR2"] != "world" || vars["VAR3"] != "hello world" {
		t.Errorf("Unexpected vars: %v", vars)
	}
}
