package g2

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// UserList represents a list of users.
// Source: https://github.com/gentoo-mirror/gentoo/blob/stable/metadata/dtd/userinfo.dtd
type UserList struct {
	XMLName xml.Name `xml:"userlist"`
	Users   []User   `xml:"user"`
}

type User struct {
	Username string    `xml:"username,attr"`
	Realname Realname  `xml:"realname"`
	PGPKeys  []string  `xml:"pgpkey"`
	Aliases  []string  `xml:"alias"`
	Emails   []string  `xml:"email"`
	Joined   []string  `xml:"joined"`
	Retired  []string  `xml:"retired"`
	Status   string    `xml:"status,omitempty"`
	Roles    string    `xml:"roles,omitempty"`
	Location *Location `xml:"location"`
}

type Realname struct {
	Fullname   string     `xml:"fullname,attr,omitempty"`
	Firstname  string     `xml:"firstname"`
	Familyname Familyname `xml:"familyname"`
}

type Familyname struct {
	Text string `xml:",innerxml"`
	Sort string `xml:"sort,attr,omitempty"`
}

type Location struct {
	Text      string `xml:",innerxml"`
	Latitude  string `xml:"latitude,attr,omitempty"`
	Longitude string `xml:"longitude,attr,omitempty"`
}

// ParseUserInfo parses a userinfo.xml file and returns a UserList pointer, or an error.
func ParseUserInfo(path string) (*UserList, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return ParseUserInfoFromReader(f)
}

// ParseUserInfoBytes parses UserList from a byte slice.
func ParseUserInfoBytes(data []byte) (*UserList, error) {
	return ParseUserInfoFromReader(bytes.NewReader(data))
}

// ParseUserInfoFromReader parses UserList from an io.Reader.
func ParseUserInfoFromReader(r io.Reader) (*UserList, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var userList UserList
	if err := xml.Unmarshal(data, &userList); err != nil {
		return nil, err
	}
	if userList.XMLName.Local != "userlist" {
		return nil, fmt.Errorf("invalid XML: expected userlist, got %s", userList.XMLName.Local)
	}

	return &userList, nil
}
