package main

import (
	"flag"
	"fmt"
	"os"
	"io"
	"net/http"
)

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

	// Create profiles directory
	if err := os.MkdirAll("profiles", 0755); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Create profiles/repo_name
	if _, err := os.Stat("profiles/repo_name"); os.IsNotExist(err) {
		if err := os.WriteFile("profiles/repo_name", []byte(name+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write profiles/repo_name: %w", err)
		}
		fmt.Printf("Created profiles/repo_name with '%s'\n", name)
	} else {
		fmt.Printf("profiles/repo_name already exists, skipping\n")
	}

	// Create profiles/eapi
	if _, err := os.Stat("profiles/eapi"); os.IsNotExist(err) {
		if err := os.WriteFile("profiles/eapi", []byte(*eapiVersion+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write profiles/eapi: %w", err)
		}
		fmt.Printf("Created profiles/eapi with '%s'\n", *eapiVersion)
	} else {
		fmt.Printf("profiles/eapi already exists, skipping\n")
	}

	// Create metadata directory
	if err := os.MkdirAll("metadata", 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Create metadata/layout.conf
	if _, err := os.Stat("metadata/layout.conf"); os.IsNotExist(err) {
		var content []byte
		if *layoutConfUrl != "" {
			fmt.Printf("Downloading layout.conf from %s...\n", *layoutConfUrl)
			resp, err := http.Get(*layoutConfUrl)
			if err != nil {
				fmt.Printf("Warning: failed to download layout.conf: %v\n", err)
			} else {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					content, _ = io.ReadAll(resp.Body)
				} else {
					fmt.Printf("Warning: failed to download layout.conf, status: %s\n", resp.Status)
				}
			}
		}
		if len(content) > 0 {
			if err := os.WriteFile("metadata/layout.conf", content, 0644); err != nil {
				return fmt.Errorf("failed to write metadata/layout.conf: %w", err)
			}
			fmt.Printf("Created metadata/layout.conf\n")
		}
	} else {
		fmt.Printf("metadata/layout.conf already exists, skipping\n")
	}

	return nil
}
