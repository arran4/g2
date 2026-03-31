package g2_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"os"
	"path/filepath"

	"golang.org/x/tools/txtar"
)

var diffEbuildsArchive = []byte(`
-- testpkg-1.0.ebuild --
EAPI=8

DESCRIPTION="Test ebuild"
HOMEPAGE="https://example.com"
SRC_URI="https://example.com/${P}.tar.gz"

LICENSE="GPL-2"
SLOT="0"
KEYWORDS="amd64 ~x86"

IUSE="test"

DEPEND="dev-libs/foo"
RDEPEND="${DEPEND}"

if [[ ${PV} == "9999" ]]; then
    PROPERTIES="live"
fi

src_prepare() {
    default
    eapply_user
}
-- testpkg-1.0.json --
[]
`)

func TestEbuildDiffJsonCommand(t *testing.T) {
	archive := txtar.Parse(diffEbuildsArchive)

	tmpDir := t.TempDir()

	for _, f := range archive.Files {
		path := filepath.Join(tmpDir, f.Name)
		if err := os.WriteFile(path, f.Data, 0644); err != nil {
			t.Fatalf("writing file %s: %v", f.Name, err)
		}
	}

	for _, f := range archive.Files {
		if strings.HasSuffix(f.Name, ".ebuild") {
			t.Run(f.Name, func(t *testing.T) {
				jsonName := strings.TrimSuffix(f.Name, ".ebuild") + ".json"
				expectedJSON, err := os.ReadFile(filepath.Join(tmpDir, jsonName))
				if err != nil {
					t.Fatalf("reading expected JSON: %v", err)
				}

				// Build the CLI
				cliPath := filepath.Join(tmpDir, "g2")
				cmd := exec.Command("go", "build", "-o", cliPath, "./cmd/g2")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("building g2: %v", err)
				}

				// Run diff-json
				cmd = exec.Command(cliPath, "ebuild", "diff-json", filepath.Join(tmpDir, f.Name))
				var out bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("running g2 ebuild diff-json: %v", err)
				}

				// Compare JSON
				var expected, actual []map[string]interface{}
				if err := json.Unmarshal(expectedJSON, &expected); err != nil {
					t.Fatalf("parsing expected JSON: %v", err)
				}
				if err := json.Unmarshal(out.Bytes(), &actual); err != nil {
					t.Fatalf("parsing actual JSON output: %v", err)
				}

				// Normalize and compare
				eBytes, _ := json.MarshalIndent(expected, "", "\t")
				aBytes, _ := json.MarshalIndent(actual, "", "\t")

				if string(eBytes) != string(aBytes) {
					t.Errorf("JSON mismatch:\nExpected:\n%s\n\nActual:\n%s", string(eBytes), string(aBytes))
				}
			})
		}
	}
}
