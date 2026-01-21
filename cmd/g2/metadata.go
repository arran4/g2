package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"log"
	"os"
	"strings"
)

// StringSliceFlag handles repeated flags for strings.
type StringSliceFlag []string

func (s *StringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *StringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// MaintainerFlag handles repeated flags for maintainers.
type MaintainerFlag []string

func (m *MaintainerFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *MaintainerFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// RemoteIDFlag handles repeated flags for upstream remote IDs.
type RemoteIDFlag []string

func (r *RemoteIDFlag) String() string {
	return strings.Join(*r, ",")
}

func (r *RemoteIDFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

// UseFlagFlag handles repeated flags for Use flags.
type UseFlagFlag []string

func (u *UseFlagFlag) String() string {
	return strings.Join(*u, ",")
}

func (u *UseFlagFlag) Set(value string) error {
	*u = append(*u, value)
	return nil
}

func (cfg *MainArgConfig) cmdMetadata(args []string) error {
	fs := flag.NewFlagSet("metadata", flag.ExitOnError)

	var maintainers MaintainerFlag
	fs.Var(&maintainers, "maintainer-add", "Add/Update maintainer (format: email[:name[:type]])")
	fs.Var(&maintainers, "maintainer", "Alias for maintainer-add")
	fs.Var(&maintainers, "m", "Alias for maintainer-add")

	var maintainerRemoves StringSliceFlag
	fs.Var(&maintainerRemoves, "maintainer-remove", "Remove maintainer (format: email)")

	longDesc := fs.String("longdescription", "", "Set long description")
	longDescShort := fs.String("l", "", "Set long description")

	var remoteIDs RemoteIDFlag
	fs.Var(&remoteIDs, "upstream-add", "Add upstream remote ID (format: type:id)")
	fs.Var(&remoteIDs, "upstream-id", "Alias for upstream-add")
	fs.Var(&remoteIDs, "u", "Alias for upstream-add")

	var remoteIDRemoves StringSliceFlag
	fs.Var(&remoteIDRemoves, "upstream-remove", "Remove upstream remote ID (format: type:id)")

	var useAdds UseFlagFlag
	fs.Var(&useAdds, "use-add", "Add/Update USE flag (format: name:description)")
	fs.Var(&useAdds, "use", "Alias for use-add")

	var useRemoves StringSliceFlag
	fs.Var(&useRemoves, "use-remove", "Remove USE flag (format: name)")

	force := fs.Bool("force", false, "Force overwrite if type mismatches or other errors")

	if err := fs.Parse(args); err != nil {
		return err
	}

	targetFile := "metadata.xml"
	if fs.NArg() > 0 {
		targetFile = fs.Arg(0)
	}

	// Load existing or create new
	var pkgMd *g2.PkgMetadata

	// Check if file exists
	if _, err := os.Stat(targetFile); err == nil {
		data, err := g2.ParseMetadata(targetFile)
		if err != nil {
			if !*force {
				return fmt.Errorf("parsing existing metadata: %w", err)
			}
			log.Printf("Error parsing existing metadata, starting fresh: %v", err)
			pkgMd = &g2.PkgMetadata{XMLName: xml.Name{Local: "pkgmetadata"}}
		} else {
			var ok bool
			pkgMd, ok = data.(*g2.PkgMetadata)
			if !ok {
				// It might be CatMetadata
				return fmt.Errorf("existing file is not PkgMetadata (maybe CatMetadata?). Editing CatMetadata not supported yet.")
			}
		}
	} else if os.IsNotExist(err) {
		pkgMd = &g2.PkgMetadata{XMLName: xml.Name{Local: "pkgmetadata"}}
	} else {
		return fmt.Errorf("stat file: %w", err)
	}

	// Apply Maintainer Removes
	for _, email := range maintainerRemoves {
		var newMaintainers []g2.Maintainer
		for _, m := range pkgMd.Maintainers {
			if m.Email != email {
				newMaintainers = append(newMaintainers, m)
			}
		}
		pkgMd.Maintainers = newMaintainers
	}

	// Apply Maintainer Adds
	for _, mStr := range maintainers {
		parts := strings.Split(mStr, ":")
		email := parts[0]
		var name, mType string
		if len(parts) > 1 {
			name = parts[1]
		}
		if len(parts) > 2 {
			mType = parts[2]
		}

		// Find if maintainer exists by email
		found := false
		for i, m := range pkgMd.Maintainers {
			if m.Email == email {
				pkgMd.Maintainers[i].Name = name
				if mType != "" {
					pkgMd.Maintainers[i].Type = mType
				}
				found = true
				break
			}
		}
		if !found {
			if mType == "" {
				mType = "person" // Default
			}
			pkgMd.Maintainers = append(pkgMd.Maintainers, g2.Maintainer{
				Email: email,
				Name:  name,
				Type:  mType,
			})
		}
	}

	// Apply LongDescription
	ldVal := *longDesc
	if ldVal == "" {
		ldVal = *longDescShort
	}
	if ldVal != "" {
		targetLang := "en"
		found := false
		for i, ld := range pkgMd.LongDescription {
			if ld.Lang == targetLang || (ld.Lang == "" && targetLang == "en") {
				pkgMd.LongDescription[i].Body = ldVal
				if ld.Lang == "" {
					pkgMd.LongDescription[i].Lang = targetLang
				}
				found = true
				break
			}
		}
		if !found {
			pkgMd.LongDescription = append(pkgMd.LongDescription, g2.LongDescription{
				Body: ldVal,
				Lang: targetLang,
			})
		}
	}

	// Apply RemoteID Removes
	if pkgMd.Upstream != nil {
		for _, rStr := range remoteIDRemoves {
			parts := strings.SplitN(rStr, ":", 2)
			if len(parts) != 2 {
				log.Printf("Invalid remote ID remove format: %s (expected type:id)", rStr)
				continue
			}
			rType := parts[0]
			rText := parts[1]

			var newRemotes []g2.RemoteID
			for _, rid := range pkgMd.Upstream.RemoteID {
				if rid.Type == rType && rid.Text == rText {
					continue
				}
				newRemotes = append(newRemotes, rid)
			}
			pkgMd.Upstream.RemoteID = newRemotes
		}
	}

	// Apply RemoteID Adds
	for _, rStr := range remoteIDs {
		parts := strings.SplitN(rStr, ":", 2)
		if len(parts) != 2 {
			log.Printf("Invalid remote ID format: %s (expected type:id)", rStr)
			continue
		}
		rType := parts[0]
		rText := parts[1]

		if pkgMd.Upstream == nil {
			pkgMd.Upstream = &g2.Upstream{}
		}

		// Check if exists
		found := false
		for _, rid := range pkgMd.Upstream.RemoteID {
			if rid.Type == rType && rid.Text == rText {
				found = true
				break
			}
		}
		if !found {
			pkgMd.Upstream.RemoteID = append(pkgMd.Upstream.RemoteID, g2.RemoteID{
				Type: rType,
				Text: rText,
			})
		}
	}

	// Apply Use Removes
	for _, name := range useRemoves {
		for i := range pkgMd.Use {
			var newFlags []g2.Flag
			for _, f := range pkgMd.Use[i].Flags {
				if f.Name != name {
					newFlags = append(newFlags, f)
				}
			}
			pkgMd.Use[i].Flags = newFlags
		}
	}

	// Apply Use Adds
	for _, uStr := range useAdds {
		parts := strings.SplitN(uStr, ":", 2)
		if len(parts) != 2 {
			log.Printf("Invalid use flag format: %s (expected name:description)", uStr)
			continue
		}
		uName := parts[0]
		uDesc := parts[1]

		targetLang := "en" // Default to english block

		// Find or create correct use block
		var useBlockIdx = -1
		for i, u := range pkgMd.Use {
			if u.Lang == targetLang || (u.Lang == "" && targetLang == "en") {
				useBlockIdx = i
				break
			}
		}

		if useBlockIdx == -1 {
			pkgMd.Use = append(pkgMd.Use, g2.Use{Lang: targetLang})
			useBlockIdx = len(pkgMd.Use) - 1
		}

		// Update or Add Flag
		found := false
		for j, f := range pkgMd.Use[useBlockIdx].Flags {
			if f.Name == uName {
				pkgMd.Use[useBlockIdx].Flags[j].Text = uDesc
				found = true
				break
			}
		}
		if !found {
			pkgMd.Use[useBlockIdx].Flags = append(pkgMd.Use[useBlockIdx].Flags, g2.Flag{
				Name: uName,
				Text: uDesc,
			})
		}
	}

	// Write
	if err := g2.WriteMetadata(targetFile, pkgMd); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}
