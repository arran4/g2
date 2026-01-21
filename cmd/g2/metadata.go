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

func (cfg *MainArgConfig) cmdMetadata(args []string) error {
	fs := flag.NewFlagSet("metadata", flag.ExitOnError)

	var maintainers MaintainerFlag
	fs.Var(&maintainers, "maintainer", "Add/Update maintainer (format: email[:name[:type]])")
	fs.Var(&maintainers, "m", "Add/Update maintainer (format: email[:name[:type]])")

	longDesc := fs.String("longdescription", "", "Set long description")
	longDescShort := fs.String("l", "", "Set long description")

	var remoteIDs RemoteIDFlag
	fs.Var(&remoteIDs, "upstream-id", "Add upstream remote ID (format: type:id)")
	fs.Var(&remoteIDs, "u", "Add upstream remote ID (format: type:id)")

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

	// Apply Maintainers
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
				// Default type if not specified and creating?
				// The user might want to keep existing type if mType is empty.
				// If mType is empty, we keep existing.
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

	// Apply RemoteIDs
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

	// Write
	if err := g2.WriteMetadata(targetFile, pkgMd); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}
