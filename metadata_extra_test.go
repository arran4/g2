package g2

import (
	"testing"
)

func TestParseGLSA(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE glsa SYSTEM "http://www.gentoo.org/dtd/glsa.dtd">
<glsa id="200311-01">
  <title>Buffer overflow vulnerability found in openssl-0.9.5</title>
  <synopsis>rsync has an exploitable buffer overflow that can lead to remote compromise</synopsis>
  <product type="ebuild">openssl</product>
  <announced>2003-11-20</announced>
  <revised count="02">2003-11-20</revised>
  <bug>34200</bug>
  <access>Remote</access>
  <affected>
    <package name="dev-libs/openssl" auto="yes" arch="*">
      <vulnerable range="lt">0.9.6k</vulnerable>
      <unaffected range="gt">0.9.6k</unaffected>
    </package>
  </affected>
  <background>
    <p>some background</p>
  </background>
  <description>
    <p>some desc</p>
  </description>
  <impact type="high">
    <p>some impact</p>
  </impact>
  <workaround>
    <p>some workaround</p>
  </workaround>
  <resolution>
    <p>some resolution</p>
  </resolution>
  <references>
    <uri link="http://www.cert.org">CERT</uri>
  </references>
  <license/>
  <metadata tag="approved">Level 1</metadata>
</glsa>`

	glsa, err := ParseGLSABytes([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseGLSABytes failed: %v", err)
	}

	if glsa.ID != "200311-01" {
		t.Errorf("expected ID '200311-01', got %s", glsa.ID)
	}
	if glsa.Title != "Buffer overflow vulnerability found in openssl-0.9.5" {
		t.Errorf("expected Title 'Buffer overflow vulnerability found in openssl-0.9.5', got %s", glsa.Title)
	}
	if glsa.Product.Type != "ebuild" {
		t.Errorf("expected Product Type 'ebuild', got %s", glsa.Product.Type)
	}
	if glsa.Revised.Count != "02" {
		t.Errorf("expected Revised Count '02', got %s", glsa.Revised.Count)
	}
	if len(glsa.Bugs) != 1 || glsa.Bugs[0] != "34200" {
		t.Errorf("expected Bug '34200', got %v", glsa.Bugs)
	}
	if len(glsa.Affected.Packages) != 1 {
		t.Fatalf("expected 1 Affected Package, got %d", len(glsa.Affected.Packages))
	}
	pkg := glsa.Affected.Packages[0]
	if pkg.Name != "dev-libs/openssl" || pkg.Auto != "yes" || pkg.Arch != "*" {
		t.Errorf("unexpected Package attributes: %#v", pkg)
	}
	if len(pkg.Vulnerable) != 1 || pkg.Vulnerable[0].Range != "lt" || pkg.Vulnerable[0].Text != "0.9.6k" {
		t.Errorf("unexpected Vulnerable: %#v", pkg.Vulnerable)
	}
	if len(glsa.Metadata) != 1 || glsa.Metadata[0].Tag != "approved" || glsa.Metadata[0].Text != "Level 1" {
		t.Errorf("unexpected Metadata: %#v", glsa.Metadata)
	}
}

func TestParseMirrors(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE mirrors SYSTEM "http://www.gentoo.org/dtd/mirrors.dtd">
<mirrors>
  <mirrorgroup region="North America" country="US" countryname="USA">
    <mirror city="Corvallis" coordinates="44.5646,-123.2620" gentoo-bug="12345">
      <name>OSU Open Source Lab</name>
      <uri protocol="http" ipv4="y" ipv6="n">http://gentoo.osuosl.org/</uri>
    </mirror>
  </mirrorgroup>
</mirrors>`

	mirrors, err := ParseMirrorsBytes([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseMirrorsBytes failed: %v", err)
	}

	if len(mirrors.MirrorGroups) != 1 {
		t.Fatalf("expected 1 MirrorGroup, got %d", len(mirrors.MirrorGroups))
	}
	group := mirrors.MirrorGroups[0]
	if group.Region != "North America" || group.Country != "US" {
		t.Errorf("unexpected group attributes: %#v", group)
	}
	if len(group.Mirrors) != 1 {
		t.Fatalf("expected 1 Mirror, got %d", len(group.Mirrors))
	}
	mirror := group.Mirrors[0]
	if mirror.City != "Corvallis" || mirror.GentooBug != "12345" {
		t.Errorf("unexpected mirror attributes: %#v", mirror)
	}
	if mirror.Name != "OSU Open Source Lab" {
		t.Errorf("expected Name 'OSU Open Source Lab', got %s", mirror.Name)
	}
	if len(mirror.URIs) != 1 {
		t.Fatalf("expected 1 URI, got %d", len(mirror.URIs))
	}
	uri := mirror.URIs[0]
	if uri.Protocol != "http" || uri.IPv4 != "y" || uri.IPv6 != "n" || uri.Text != "http://gentoo.osuosl.org/" {
		t.Errorf("unexpected URI: %#v", uri)
	}
}

func TestParseProjects(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE projects SYSTEM "http://www.gentoo.org/dtd/projects.dtd">
<projects>
  <project>
    <email>council@gentoo.org</email>
    <name>Gentoo Council</name>
    <url>https://wiki.gentoo.org/wiki/Project:Council</url>
    <description>The Gentoo Council is an elected body...</description>
    <subproject inherit-members="1" ref="base-system@gentoo.org"/>
    <member is-lead="1">
      <email>dev@gentoo.org</email>
      <name>John Doe</name>
      <role>Chair</role>
    </member>
  </project>
</projects>`

	projects, err := ParseProjectsBytes([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseProjectsBytes failed: %v", err)
	}

	if len(projects.Projects) != 1 {
		t.Fatalf("expected 1 Project, got %d", len(projects.Projects))
	}
	proj := projects.Projects[0]
	if proj.Email != "council@gentoo.org" || proj.Name != "Gentoo Council" {
		t.Errorf("unexpected project details: %#v", proj)
	}
	if len(proj.Subprojects) != 1 {
		t.Fatalf("expected 1 Subproject, got %d", len(proj.Subprojects))
	}
	if proj.Subprojects[0].Ref != "base-system@gentoo.org" {
		t.Errorf("expected subproject ref 'base-system@gentoo.org', got %s", proj.Subprojects[0].Ref)
	}
	if len(proj.Members) != 1 {
		t.Fatalf("expected 1 Member, got %d", len(proj.Members))
	}
	mem := proj.Members[0]
	if mem.IsLead != "1" || mem.Email != "dev@gentoo.org" || mem.Role != "Chair" {
		t.Errorf("unexpected member details: %#v", mem)
	}
}

func TestParseRepositories(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE repositories SYSTEM "http://www.gentoo.org/dtd/repositories.dtd">
<repositories version="1.0">
  <repo quality="experimental" status="unofficial">
    <name>foo-overlay</name>
    <description lang="en">Foo Overlay</description>
    <homepage>https://github.com/foo/foo-overlay</homepage>
    <owner type="person">
      <email>foo@example.com</email>
      <name>Foo Bar</name>
    </owner>
    <source type="git">https://github.com/foo/foo-overlay.git</source>
    <feed>https://github.com/foo/foo-overlay/commits/master.atom</feed>
  </repo>
</repositories>`

	repos, err := ParseRepositoriesBytes([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseRepositoriesBytes failed: %v", err)
	}

	if repos.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", repos.Version)
	}
	if len(repos.Repositories) != 1 {
		t.Fatalf("expected 1 Repository, got %d", len(repos.Repositories))
	}
	repo := repos.Repositories[0]
	if repo.Quality != "experimental" || repo.Status != "unofficial" || repo.Name != "foo-overlay" {
		t.Errorf("unexpected repo details: %#v", repo)
	}
	if len(repo.Owners) != 1 || repo.Owners[0].Email != "foo@example.com" {
		t.Errorf("unexpected repo owner: %#v", repo.Owners)
	}
	if len(repo.Sources) != 1 || repo.Sources[0].Type != "git" || repo.Sources[0].Text != "https://github.com/foo/foo-overlay.git" {
		t.Errorf("unexpected repo source: %#v", repo.Sources)
	}
}

func TestParseUserInfo(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE userlist SYSTEM "http://www.gentoo.org/dtd/userinfo.dtd">
<userlist>
  <user username="foo">
    <realname fullname="Foo Bar">
      <firstname>Foo</firstname>
      <familyname sort="Bar">Bar</familyname>
    </realname>
    <pgpkey>0x12345678</pgpkey>
    <email>foo@gentoo.org</email>
    <joined>2010-01-01</joined>
    <status>active</status>
    <roles>Developer</roles>
    <location latitude="50.0" longitude="10.0">Germany</location>
  </user>
</userlist>`

	users, err := ParseUserInfoBytes([]byte(xmlData))
	if err != nil {
		t.Fatalf("ParseUserInfoBytes failed: %v", err)
	}

	if len(users.Users) != 1 {
		t.Fatalf("expected 1 User, got %d", len(users.Users))
	}
	user := users.Users[0]
	if user.Username != "foo" {
		t.Errorf("expected username foo, got %s", user.Username)
	}
	if user.Realname.Firstname != "Foo" || user.Realname.Familyname.Text != "Bar" {
		t.Errorf("unexpected realname: %#v", user.Realname)
	}
	if len(user.PGPKeys) != 1 || user.PGPKeys[0] != "0x12345678" {
		t.Errorf("unexpected pgp keys: %#v", user.PGPKeys)
	}
	if user.Location == nil || user.Location.Latitude != "50.0" || user.Location.Text != "Germany" {
		t.Errorf("unexpected location: %#v", user.Location)
	}
}
