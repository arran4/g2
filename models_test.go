package g2

import (
	"encoding/xml"
	"reflect"
	"testing"
)

func assertSerializeStable(t *testing.T, val interface{}, name string) {
	b, err := xml.Marshal(val)
	if err != nil {
		t.Fatalf("%s Marshal failed: %v", name, err)
	}

	valType := reflect.TypeOf(val)
	if valType.Kind() == reflect.Ptr {
		valType = valType.Elem()
	}
	out := reflect.New(valType).Interface()

	if err := xml.Unmarshal(b, out); err != nil {
		t.Fatalf("%s Unmarshal failed: %v", name, err)
	}

	b2, err := xml.Marshal(out)
	if err != nil {
		t.Fatalf("%s Second Marshal failed: %v", name, err)
	}

	if string(b) != string(b2) {
		t.Fatalf("%s Not serialize stable.\nOriginal: %s\nReserialized: %s", name, string(b), string(b2))
	}
}

func TestModelsSerialization(t *testing.T) {
	t.Run("GLSA", func(t *testing.T) {
		glsa := &GLSA{
			XMLName:   xml.Name{Local: "glsa"},
			ID:        "200311-01",
			Title:     "Buffer overflow",
			Synopsis:  "Small description",
			Product:   Product{Type: "ebuild", Text: "openssl"},
			Announced: "2003-11-20",
			Revised:   Revised{Count: "01", Text: "2003-11-20"},
			Bugs:      []string{"12345"},
			Access:    "Remote",
			Affected: Affected{
				Packages: []Package{
					{
						Name: "dev-libs/openssl",
						Auto: "yes",
						Arch: "*",
						Vulnerable: []Vulnerable{
							{Range: "lt", Slot: "*", Text: "0.9.6k"},
						},
						Unaffected: []Unaffected{
							{Range: "ge", Slot: "*", Text: "0.9.6k"},
						},
					},
				},
			},
			Background:  &Background{Text: "Background desc"},
			Description: GLSADescription{Text: "Security issue"},
			Impact:      Impact{Type: "normal", Text: "Impact desc"},
			Workaround:  Workaround{Text: "Workaround desc"},
			Resolution:  Resolution{Text: "Resolution desc"},
			References: References{
				URIs: []URI{
					{Link: "http://example.com", Text: "Example"},
				},
			},
			License: &License{Text: "GPL-2"},
			Metadata: []GLSAMetadata{
				{
					Tag:  "approved",
					Text: `Level 1<metadata tag="nested">true</metadata>`,
				},
			},
		}
		assertSerializeStable(t, glsa, "GLSA")
	})

	t.Run("PkgMetadata", func(t *testing.T) {
		pmd := &PkgMetadata{
			XMLName: xml.Name{Local: "pkgmetadata"},
			Maintainers: []Maintainer{
				{Email: "foo@gentoo.org", Name: "Foo", Type: "person", Proxied: "no", Restrict: "test"},
			},
			LongDescription: []LongDescription{
				{Body: "This is a package.", Lang: "en", Restrict: "test"},
			},
			Slots: &Slots{
				Slot: []Slot{
					{Name: "0", Text: "Main slot"},
				},
				Subslots: "sub",
				Lang:     "en",
			},
			StabilizeAllArches: &StabilizeAllArches{Restrict: "test"},
			Use: []Use{
				{
					Flags: []Flag{
						{Name: "foo", Text: "Enable foo", Restrict: "test"},
					},
					Lang: "en",
				},
			},
			Upstream: &Upstream{
				Maintainers: []Maintainer{
					{Name: "Upstream Foo", Status: "active"},
				},
				Changelog: "http://example.com/changelog",
				Doc: []Doc{
					{URL: "http://example.com/doc", Lang: "en"},
				},
				BugsTo: "http://example.com/bugs",
				RemoteID: []RemoteID{
					{Type: "github", Text: "foo/bar"},
				},
			},
		}
		assertSerializeStable(t, pmd, "PkgMetadata")
	})

	t.Run("Mirrors", func(t *testing.T) {
		mirrors := &Mirrors{
			XMLName: xml.Name{Local: "mirrors"},
			MirrorGroups: []MirrorGroup{
				{
					Region:      "Europe",
					Country:     "DE",
					CountryName: "Germany",
					Mirrors: []Mirror{
						{
							City:        "Berlin",
							Coordinates: "52.5200, 13.4050",
							GentooBug:   "123",
							Name:        "Test Mirror",
							URIs: []MirrorURI{
								{Text: "http://mirror.example.com/", IPv4: "Y", IPv6: "N", Partial: "N", Protocol: "http"},
							},
						},
					},
				},
			},
		}
		assertSerializeStable(t, mirrors, "Mirrors")
	})

	t.Run("Projects", func(t *testing.T) {
		projects := &Projects{
			XMLName: xml.Name{Local: "projects"},
			Projects: []Project{
				{
					Email:       "test@gentoo.org",
					Name:        "Test Project",
					URL:         "https://wiki.gentoo.org/wiki/Test",
					Description: "A test project",
					Subprojects: []Subproject{
						{Text: "Sub", InheritMembers: "1", Ref: "sub@gentoo.org"},
					},
					Members: []Member{
						{Email: "member@gentoo.org", Name: "Member", Role: "Dev", IsLead: "1"},
					},
				},
			},
		}
		assertSerializeStable(t, projects, "Projects")
	})

	t.Run("Repositories", func(t *testing.T) {
		repos := &Repositories{
			XMLName: xml.Name{Local: "repositories"},
			Version: "1.0",
			Repositories: []Repository{
				{
					Priority: "10",
					Quality:  "testing",
					Status:   "unofficial",
					Name:     "test-repo",
					Descriptions: []RepositoryDescription{
						{Text: "A test repository", Lang: "en"},
					},
					Homepage: "https://example.com",
					Owners: []RepositoryOwner{
						{Email: "owner@example.com", Name: "Owner", Type: "person"},
					},
					Sources: []RepositorySource{
						{Text: "https://example.com/repo.git", Type: "git"},
					},
					Feeds: []string{"https://example.com/feed.xml"},
				},
			},
		}
		assertSerializeStable(t, repos, "Repositories")
	})

	t.Run("UserList", func(t *testing.T) {
		ul := &UserList{
			XMLName: xml.Name{Local: "userlist"},
			Users: []User{
				{
					Username: "testuser",
					Realname: Realname{
						Fullname:  "Test User",
						Firstname: "Test",
						Familyname: Familyname{
							Text: "User",
							Sort: "User, Test",
						},
					},
					PGPKeys: []string{"0x12345678"},
					Aliases: []string{"tester"},
					Emails:  []string{"testuser@gentoo.org"},
					Joined:  []string{"2020-01-01"},
					Retired: []string{"2025-01-01"},
					Status:  "active",
					Roles:   "Developer",
					Location: &Location{
						Text:      "Earth",
						Latitude:  "0.0",
						Longitude: "0.0",
					},
				},
			},
		}
		assertSerializeStable(t, ul, "UserList")
	})
}
