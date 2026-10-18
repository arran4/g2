package g2

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestMatchMaskAtom(t *testing.T) {
	data, err := os.ReadFile("testdata/mask_audit/atoms.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Atom, Version, Slot string
		Match, Error        bool
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Atom+"/"+tc.Version+"/"+tc.Slot, func(t *testing.T) {
			got, err := MatchMaskAtom(tc.Atom, MaskCandidate{Category: "cat", Package: "pkg", Version: tc.Version, Repository: "test", Slot: tc.Slot})
			if (err != nil) != tc.Error || got != tc.Match {
				t.Fatalf("match = %v, %v; want match=%v error=%v", got, err, tc.Match, tc.Error)
			}
		})
	}
}

func TestAuditUnmasks(t *testing.T) {
	data, err := os.ReadFile("testdata/mask_audit/classifications.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name     string
		Input    MaskAuditInput
		Want     []UnmaskClassification
		Affected []string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			before, _ := json.Marshal(tc.Input)
			report := AuditUnmasks(tc.Input)
			var got []UnmaskClassification
			var affected []string
			for _, entry := range report.Entries {
				got = append(got, entry.Classification)
				if entry.Source.File == "" || entry.Source.Line < 1 || entry.Explanation == "" {
					t.Fatalf("missing provenance/explanation: %+v", entry)
				}
				for _, a := range entry.Affected {
					affected = append(affected, a.Candidate.String())
					if len(a.Masks) == 0 || a.Masks[0].File == "" || a.Masks[0].Line < 1 {
						t.Fatalf("missing mask provenance: %+v", a)
					}
				}
			}
			if !reflect.DeepEqual(got, tc.Want) || !reflect.DeepEqual(affected, tc.Affected) {
				t.Fatalf("classifications=%v affected=%v; want %v %v", got, affected, tc.Want, tc.Affected)
			}
			after, _ := json.Marshal(tc.Input)
			if string(before) != string(after) {
				t.Fatal("audit mutated its input")
			}
			again := AuditUnmasks(tc.Input)
			if !reflect.DeepEqual(report, again) {
				t.Fatal("audit result or summary is nondeterministic")
			}
			if report.Summary.Shadowed > 0 && len(report.Warnings) == 0 {
				t.Fatal("shadowed entries must not imply a simultaneous removal plan")
			}
		})
	}
}

func materializeMaskAudit(t *testing.T) (string, []UnmaskClassification) {
	t.Helper()
	return materializeMaskAuditFixture(t, "overlay.txtar")
}

func materializeMaskAuditFixture(t *testing.T, name string) (string, []UnmaskClassification) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata/mask_audit", name))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	var want []UnmaskClassification
	for _, f := range txtar.Parse(data).Files {
		if f.Name == "expected.json" {
			if err := json.Unmarshal(f.Data, &want); err != nil {
				t.Fatal(err)
			}
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(f.Data), "@ROOT@", filepath.ToSlash(root))), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root, want
}

func TestMaskAuditInheritanceAndIncrementals(t *testing.T) {
	root, want := materializeMaskAuditFixture(t, "inheritance.txtar")
	in, err := LoadMaskAuditInput(MaskAuditOptions{
		ConfigRoot: filepath.Join(root, "config"), Profile: filepath.Join(root, "child/profiles/selected"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Problems) > 0 {
		t.Fatalf("unexpected discovery problems: %v", in.Problems)
	}
	report := AuditUnmasks(in)
	var got []UnmaskClassification
	for _, entry := range report.Entries {
		got = append(got, entry.Classification)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classifications=%v, want %v; entries=%+v", got, want, report.Entries)
	}
}

func auditTreeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String() + info.ModTime().String()
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += string(data)
		}
		snapshot[path] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestLoadMaskAuditInput(t *testing.T) {
	root, want := materializeMaskAudit(t)
	before := auditTreeSnapshot(t, root)
	in, err := LoadMaskAuditInput(MaskAuditOptions{
		ConfigRoot: filepath.Join(root, "config"), Profile: filepath.Join(root, "repo/profiles/selected"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Problems) > 0 {
		t.Fatalf("unexpected discovery problems: %v", in.Problems)
	}
	report := AuditUnmasks(in)
	var got []UnmaskClassification
	for _, entry := range report.Entries {
		got = append(got, entry.Classification)
		if entry.Source.File != filepath.Join(root, "config/package.unmask/10-main") {
			t.Fatalf("wrong source file: %+v", entry.Source)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classifications=%v, want %v; entries=%+v", got, want, report.Entries)
	}
	if report.Entries[0].Source.Line != 3 {
		t.Fatalf("lost source line: %+v", report.Entries[0].Source)
	}
	if after := auditTreeSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("loading/auditing modified the filesystem")
	}
}

func TestMissingAuditProfileIsIndeterminate(t *testing.T) {
	root, _ := materializeMaskAudit(t)
	in, err := LoadMaskAuditInput(MaskAuditOptions{ConfigRoot: filepath.Join(root, "config")})
	if err != nil {
		t.Fatal(err)
	}
	report := AuditUnmasks(in)
	if report.Summary.Indeterminate != len(report.Entries) || len(report.Warnings) == 0 {
		t.Fatalf("missing profile must prevent redundancy claims: %+v", report)
	}
}

func TestMaskAuditDefaultProfile(t *testing.T) {
	root, want := materializeMaskAudit(t)
	if err := os.Symlink(filepath.Join(root, "repo/profiles/selected"), filepath.Join(root, "config/make.profile")); err != nil {
		t.Skipf("profile symlink is unavailable: %v", err)
	}
	in, err := LoadMaskAuditInput(MaskAuditOptions{ConfigRoot: filepath.Join(root, "config")})
	if err != nil {
		t.Fatal(err)
	}
	report := AuditUnmasks(in)
	var got []UnmaskClassification
	for _, entry := range report.Entries {
		got = append(got, entry.Classification)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default make.profile classifications=%v, want %v", got, want)
	}
}

func TestMaskAuditUncertainDiscovery(t *testing.T) {
	data, err := os.ReadFile("testdata/mask_audit/uncertain.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, Path, Content string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			root, _ := materializeMaskAudit(t)
			path := filepath.Join(root, filepath.FromSlash(tc.Path))
			content := strings.ReplaceAll(tc.Content, "@ROOT@", filepath.ToSlash(root))
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			before := auditTreeSnapshot(t, root)
			in, err := LoadMaskAuditInput(MaskAuditOptions{
				ConfigRoot: filepath.Join(root, "config"), Profile: filepath.Join(root, "repo/profiles/selected"),
			})
			if err != nil {
				t.Fatal(err)
			}
			report := AuditUnmasks(in)
			if report.Summary.Indeterminate != len(report.Entries) || len(report.Warnings) == 0 {
				t.Fatalf("uncertain inputs must prevent redundancy claims: %+v", report)
			}
			if !reflect.DeepEqual(before, auditTreeSnapshot(t, root)) {
				t.Fatal("audit modified uncertain inputs")
			}
		})
	}
}
