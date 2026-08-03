package g2

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestMetadataComments_MarshalXML(t *testing.T) {
	pkgMd := &PkgMetadata{
		XMLName:  xml.Name{Local: "pkgmetadata"},
		Comments: MetadataComments{"maintainer-needed", "another comment"},
		Maintainers: []Maintainer{
			{Email: "foo@gentoo.org"},
		},
	}

	b, err := xml.MarshalIndent(pkgMd, "", "\t")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}

	expected := `<pkgmetadata><!-- maintainer-needed -->
&#x9;<!-- another comment -->
	<maintainer>
		<email>foo@gentoo.org</email>
	</maintainer>
</pkgmetadata>`

	if strings.TrimSpace(string(b)) != strings.TrimSpace(expected) {
		t.Errorf("Mismatch in Comments serialization.\nGot:\n%s\nWant:\n%s", string(b), expected)
	}

	catMd := &CatMetadata{
		XMLName:  xml.Name{Local: "catmetadata"},
		Comments: MetadataComments{"category comment"},
	}

	b, err = xml.MarshalIndent(catMd, "", "\t")
	if err != nil {
		t.Fatalf("MarshalIndent error: %v", err)
	}

	expectedCat := `<catmetadata><!-- category comment --></catmetadata>`

	if strings.TrimSpace(string(b)) != strings.TrimSpace(expectedCat) {
		t.Errorf("Mismatch in CatMetadata Comments serialization.\nGot:\n%s\nWant:\n%s", string(b), expectedCat)
	}
}
