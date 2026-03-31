package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/arran4/g2"
)

func (cfg *CmdEbuildArgConfig) cmdEbuildDiffJson(args []string) error {
	fs := flag.NewFlagSet("diff-json", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: g2 ebuild diff-json <ebuild file>")
	}
	filename := fs.Arg(0)

	dir := filepath.Dir(filename)
	base := filepath.Base(filename)

	// parse with sh-parse-to-json
	shEbuild, err := g2.ParseEbuild(os.DirFS(dir), base, g2.ParseVariables)
	if err != nil {
		return fmt.Errorf("parsing ebuild with shell parser %s: %w", filename, err)
	}

	// parse with as-json
	nativeEbuild, err := g2.ParseEbuild(os.DirFS(dir), base, g2.ParseFull)
	if err != nil {
		return fmt.Errorf("parsing ebuild with native parser %s: %w", filename, err)
	}

	// Let's create a map of keys that are present in either parsing method
	allKeys := make(map[string]bool)
	for k := range shEbuild.Vars {
		allKeys[k] = true
	}
	for k := range nativeEbuild.Vars {
		allKeys[k] = true
	}

	type DiffEntry struct {
		Key       string `json:"key"`
		ShVal     string `json:"sh_val,omitempty"`
		NativeVal string `json:"native_val,omitempty"`
		Matches   bool   `json:"matches"`
	}

	var diffs []DiffEntry

	var sortedKeys []string
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}

	// Sort keys to ensure deterministic output
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		shVal := shEbuild.Vars[k]
		nativeVal := nativeEbuild.Vars[k]

		matches := shVal == nativeVal
		diffs = append(diffs, DiffEntry{
			Key:       k,
			ShVal:     shVal,
			NativeVal: nativeVal,
			Matches:   matches,
		})
	}

	jsonBytes, err := json.MarshalIndent(diffs, "", "\t")
	if err != nil {
		return fmt.Errorf("serializing diff to json: %w", err)
	}

	fmt.Println(string(jsonBytes))

	return nil
}
