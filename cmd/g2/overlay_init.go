package main

import (
	"flag"
	"fmt"
	"os"
	"io"
	"net/http"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
)

type OverlayInitArgs struct {
	Name          string
	EapiVersion   string
	LayoutConfUrl string
}

func (cfg *MainArgConfig) cmdOverlayInit(args []string) error {
	fs := flag.NewFlagSet("overlay init", flag.ExitOnError)
	overlayName := fs.String("overlay-name", "my-overlay", "Name of the overlay")
	repoName := fs.String("repo-name", "", "Name of the repository (alias for overlay-name)")
	eapiVersion := fs.String("eapi-version", "8", "EAPI version")
	layoutConfUrl := fs.String("layout-conf-url", "https://raw.githubusercontent.com/gentoo/gentoo/master/metadata/layout.conf", "URL to fetch layout.conf from (if empty, a minimal one is created)")

	fs.Usage = func() {
		fmt.Printf("Usage: g2 overlay init [flags]\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	name := *overlayName
	if *repoName != "" {
		name = *repoName
	}

	initArgs := OverlayInitArgs{
		Name:          name,
		EapiVersion:   *eapiVersion,
		LayoutConfUrl: *layoutConfUrl,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	targetFs := osfs.New(cwd)

	return InitOverlay(targetFs, initArgs)
}

func InitOverlay(fs billy.Filesystem, args OverlayInitArgs) error {
	// Create profiles directory
	if err := fs.MkdirAll("profiles", 0755); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Create profiles/repo_name
	if _, err := fs.Stat("profiles/repo_name"); os.IsNotExist(err) {
		f, err := fs.Create("profiles/repo_name")
		if err != nil {
			return fmt.Errorf("failed to write profiles/repo_name: %w", err)
		}
		_, err = f.Write([]byte(args.Name + "\n"))
		f.Close()
		if err != nil {
			return fmt.Errorf("failed to write profiles/repo_name: %w", err)
		}
		fmt.Printf("Created profiles/repo_name with '%s'\n", args.Name)
	} else {
		fmt.Printf("profiles/repo_name already exists, skipping\n")
	}

	// Create profiles/eapi
	if _, err := fs.Stat("profiles/eapi"); os.IsNotExist(err) {
		f, err := fs.Create("profiles/eapi")
		if err != nil {
			return fmt.Errorf("failed to write profiles/eapi: %w", err)
		}
		_, err = f.Write([]byte(args.EapiVersion + "\n"))
		f.Close()
		if err != nil {
			return fmt.Errorf("failed to write profiles/eapi: %w", err)
		}
		fmt.Printf("Created profiles/eapi with '%s'\n", args.EapiVersion)
	} else {
		fmt.Printf("profiles/eapi already exists, skipping\n")
	}

	// Create metadata directory
	if err := fs.MkdirAll("metadata", 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Create metadata/layout.conf
	if _, err := fs.Stat("metadata/layout.conf"); os.IsNotExist(err) {
		var content []byte
		if args.LayoutConfUrl != "" {
			fmt.Printf("Downloading layout.conf from %s...\n", args.LayoutConfUrl)
			resp, err := http.Get(args.LayoutConfUrl)
			if err != nil {
				fmt.Printf("Warning: failed to download layout.conf: %v\n", err)
			} else {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					content, _ = io.ReadAll(resp.Body)
				} else {
					fmt.Printf("Warning: failed to download layout.conf, status: %s\n", resp.Status)
				}
				_ = resp.Body.Close() // Explicitly ignore error check as per comment
			}
		}
		if len(content) > 0 {
			f, err := fs.Create("metadata/layout.conf")
			if err != nil {
				return fmt.Errorf("failed to write metadata/layout.conf: %w", err)
			}
			_, err = f.Write(content)
			f.Close()
			if err != nil {
				return fmt.Errorf("failed to write metadata/layout.conf: %w", err)
			}
			fmt.Printf("Created metadata/layout.conf\n")
		}
	} else {
		fmt.Printf("metadata/layout.conf already exists, skipping\n")
	}

	return nil
}
