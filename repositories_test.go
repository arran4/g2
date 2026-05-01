package g2

import (
	_ "embed"
	"encoding/xml"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

//go:embed testdata/repositories/repositories.xml
var validRepositoriesXML []byte

func TestParseRepositories(t *testing.T) {
	expected := &Repositories{
		XMLName: xml.Name{Local: "repositories"},
		Version: "1.0",
		Repositories: []Repository{
			{
				Priority: "50",
				Quality:  "experimental",
				Status:   "unofficial",
				Name:     "foo-overlay",
				Descriptions: []RepositoryDescription{
					{Text: "Foo Overlay", Lang: "en"},
					{Text: "Overlay Foo", Lang: "fr"},
				},
				LongDescriptions: []RepositoryDescription{
					{Text: "This is the foo overlay, providing various foo packages.", Lang: "en"},
				},
				Homepage: "https://example.com/foo",
				Owners: []RepositoryOwner{
					{Type: "project", Email: "foo@example.com", Name: "Foo Project"},
					{Type: "person", Email: "bar@example.com", Name: "Bar Person"},
				},
				Sources: []RepositorySource{
					{Type: "git", Text: "https://git.example.com/foo.git"},
					{Type: "rsync", Text: "rsync://rsync.example.com/foo"},
				},
				Feeds: []string{
					"https://example.com/foo/atom.xml",
				},
			},
			{
				Priority: "",
				Quality:  "core",
				Status:   "official",
				Name:     "gentoo",
				Descriptions: []RepositoryDescription{
					{Text: "Gentoo Linux repository", Lang: "en"},
				},
				Owners: []RepositoryOwner{
					{Type: "project", Email: "bug-wranglers@gentoo.org"},
				},
				Sources: []RepositorySource{
					{Type: "rsync", Text: "rsync://rsync.gentoo.org/gentoo-portage"},
				},
			},
		},
	}

	reposPath := filepath.Join("testdata", "repositories", "repositories.xml")
	repos, err := ParseRepositories(reposPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(repos, expected) {
		t.Errorf("expected:\n%+v\ngot:\n%+v", expected, repos)
	}
}

func TestParseRepositoriesBytes(t *testing.T) {
	expectedName := "foo-overlay"

	repos, err := ParseRepositoriesBytes(validRepositoriesXML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos.Repositories) == 0 || repos.Repositories[0].Name != expectedName {
		t.Errorf("expected first repository name to be %q, got %q", expectedName, repos.Repositories[0].Name)
	}
}

func TestParseRepositoriesFromReader(t *testing.T) {
	expectedName := "foo-overlay"

	repos, err := ParseRepositoriesFromReader(strings.NewReader(string(validRepositoriesXML)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos.Repositories) == 0 || repos.Repositories[0].Name != expectedName {
		t.Errorf("expected first repository name to be %q, got %q", expectedName, repos.Repositories[0].Name)
	}
}

func TestParseRepositories_ErrorHandling(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		_, err := ParseRepositories("testdata/repositories/does-not-exist.xml")
		if err == nil {
			t.Fatal("expected error for non-existent file, got nil")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected os.ErrNotExist, got: %v", err)
		}
	})

	t.Run("invalid xml", func(t *testing.T) {
		invalidPath := filepath.Join("testdata", "repositories", "invalid.xml")
		_, err := ParseRepositories(invalidPath)
		if err == nil {
			t.Fatal("expected error for invalid XML, got nil")
		}
	})

	t.Run("wrong root element", func(t *testing.T) {
		wrongRootPath := filepath.Join("testdata", "repositories", "wrong-root.xml")
		_, err := ParseRepositories(wrongRootPath)
		if err == nil {
			t.Fatal("expected error for wrong root element, got nil")
		}
		expectedErrPrefix := "expected element type <repositories> but have"
		if !strings.HasPrefix(err.Error(), expectedErrPrefix) {
			t.Errorf("expected error to start with %q, got: %q", expectedErrPrefix, err.Error())
		}
	})
}
