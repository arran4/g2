package g2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMakeConf(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "make.conf")
	confDDir := filepath.Join(dir, "make.conf.d")

	err := os.Mkdir(confDDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(confDDir, "01-custom.conf"), []byte(`VAR4="sourced"`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(confPath, []byte(`
VAR1="hello"
VAR2='world'
VAR3="${VAR1} ${VAR2}"
export VAR1 VAR2 VAR3
USE="foo"
USE+=" bar"
if true; then
    USE+=" baz"
fi
ARR=(a b c)
ARR2=(1 2)
ARR2+=(3)

source "`+filepath.Join(confDDir, "01-custom.conf")+`"
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

	if vars["USE"] != "foo bar baz" {
		t.Errorf("Unexpected USE: %q", vars["USE"])
	}

	if vars["ARR"] != "a b c" {
		t.Errorf("Unexpected ARR: %q", vars["ARR"])
	}

	if vars["ARR2"] != "1 2 3" {
		t.Errorf("Unexpected ARR2: %q", vars["ARR2"])
	}

	if vars["VAR4"] != "sourced" {
		t.Errorf("Unexpected VAR4 (source failed?): %q", vars["VAR4"])
	}
}
