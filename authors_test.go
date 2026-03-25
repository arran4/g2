package g2

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseAuthors(t *testing.T) {
	input := `# This is a partial list of copyright holders for Gentoo packages.
# It is opt-in and manually maintained, so it will be neither complete
# nor necessarily up to date. A more exhaustive list can be obtained
# by additionally extracting author information from the commit history
# of the Gentoo CVS and git repositories.
#
# Requests to be listed below can be filed at https://bugs.gentoo.org/
# under the "Gentoo Council" product. To be considered, an entity must
# have made a legally significant contribution, as determined by the
# Gentoo Council. As a reference, the following guide can be used:
# https://www.gnu.org/prep/maintain/html_node/Legally-Significant.html
#
# Entries are single lines and contain the entity's name and an optional
# e-mail address. Keep the list sorted (use "LC_ALL=en_US.UTF-8 sort").
#
Eli Schwartz <eschwartz@gentoo.org>
Michał Górny <mgorny@gentoo.org>
Sam James <sam@gentoo.org>
Sony Interactive Entertainment Inc.

# The following per https://bugs.gentoo.org/730200
Robin H. Johnson <robbat2@gentoo.org>
BC Libraries Cooperative 2009
Epik Networks Inc
Experq Oy
Global NetOptex Inc
IsoHunt Web Technologies Inc.
Merkle: The Gallery Group
Net-Conex Business Solutions Inc
Simon Fraser University
Technical University Of British Columbia (TechBC)`

	expected := []Author{
		{Name: "Eli Schwartz", Email: "eschwartz@gentoo.org"},
		{Name: "Michał Górny", Email: "mgorny@gentoo.org"},
		{Name: "Sam James", Email: "sam@gentoo.org"},
		{Name: "Sony Interactive Entertainment Inc.", Email: ""},
		{Name: "Robin H. Johnson", Email: "robbat2@gentoo.org"},
		{Name: "BC Libraries Cooperative 2009", Email: ""},
		{Name: "Epik Networks Inc", Email: ""},
		{Name: "Experq Oy", Email: ""},
		{Name: "Global NetOptex Inc", Email: ""},
		{Name: "IsoHunt Web Technologies Inc.", Email: ""},
		{Name: "Merkle: The Gallery Group", Email: ""},
		{Name: "Net-Conex Business Solutions Inc", Email: ""},
		{Name: "Simon Fraser University", Email: ""},
		{Name: "Technical University Of British Columbia (TechBC)", Email: ""},
	}

	authors, err := ParseAuthors(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(authors, expected) {
		t.Errorf("expected:\n%v\ngot:\n%v", expected, authors)
	}
}
