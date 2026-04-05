package g2

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseArchList(t *testing.T) {
	r := strings.NewReader("amd64\n# a comment\n\n  x86  \narm64")
	al, err := ParseArchList(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"amd64", "x86", "arm64"}
	if !reflect.DeepEqual(al.Arches, expected) {
		t.Errorf("expected %v, got %v", expected, al.Arches)
	}
}
