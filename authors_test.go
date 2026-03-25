package g2

import (
	_ "embed"
	"reflect"
	"strings"
	"testing"
)

//go:embed testdata/authors/authors.txt
var authorsTestInput string

func TestParseAuthors(t *testing.T) {
	expected := []Author{
		{Name: "Alice Schwartz", Email: "alice@example.com", Line: 16},
		{Name: "Bob Smith", Email: "bob@example.com", Line: 17},
		{Name: "Charlie Johnson", Email: "charlie@example.com", Line: 18},
		{Name: "Global Entertainment Inc.", Email: "", Line: 19},
		{Name: "Dana H. Williams", Email: "dana@example.com", Line: 22},
		{Name: "Example Libraries Cooperative 2009", Email: "", Line: 23},
		{Name: "Test Networks Inc", Email: "", Line: 24},
		{Name: "Sample Oy", Email: "", Line: 25},
		{Name: "Global Example Inc", Email: "", Line: 26},
		{Name: "Web Technologies Inc.", Email: "", Line: 27},
		{Name: "The Gallery Group", Email: "", Line: 28},
		{Name: "Business Solutions Inc", Email: "", Line: 29},
		{Name: "Fraser University", Email: "", Line: 30},
		{Name: "Technical University (TechBC)", Email: "", Line: 31},
	}

	authors, err := ParseAuthors(strings.NewReader(authorsTestInput))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(authors, expected) {
		t.Errorf("expected:\n%v\ngot:\n%v", expected, authors)
	}
}
