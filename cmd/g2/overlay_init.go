package main

import (
	"flag"
	"fmt"
	"os"
	"io"
	"net/http"
	"path/filepath"
)

type OverlayInitArgs struct {
	Name          string
	EapiVersion   string
	LayoutConfUrl string
}

// SimpleFS provides a minimal interface for file system operations needed by overlay init.
type SimpleFS interface {
	MkdirAll(path string, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
}

// osSimpleFS implements SimpleFS using the os package.
type osSimpleFS struct {
	baseDir string
}

func (fs *osSimpleFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(filepath.Join(fs.baseDir, path), perm)
}

func (fs *osSimpleFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(fs.baseDir, name))
}

func (fs *osSimpleFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filepath.Join(fs.baseDir, name), data, perm)
}

func (cfg *MainArgConfig) cmdOverlayInit(args []string) error {
	fsFlags := flag.NewFlagSet("overlay init", flag.ExitOnError)
	overlayName := fsFlags.String("overlay-name", "my-overlay", "Name of the overlay")
	repoName := fsFlags.String("repo-name", "", "Name of the repository (alias for overlay-name)")
	eapiVersion := fsFlags.String("eapi-version", "8", "EAPI version")
	layoutConfUrl := fsFlags.String("layout-conf-url", "https://raw.githubusercontent.com/gentoo/gentoo/master/metadata/layout.conf", "URL to fetch layout.conf from (if empty, a minimal one is created)")

	fsFlags.Usage = func() {
		fmt.Printf("Usage: g2 overlay init [flags]\n")
		fsFlags.PrintDefaults()
	}

	if err := fsFlags.Parse(args); err != nil {
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

	targetFs := &osSimpleFS{baseDir: cwd}

	return InitOverlay(targetFs, initArgs)
}

func InitOverlay(fs SimpleFS, args OverlayInitArgs) error {
	// Create profiles directory
	if err := fs.MkdirAll("profiles", 0755); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Create profiles/repo_name
	if _, err := fs.Stat("profiles/repo_name"); os.IsNotExist(err) {
		if err := fs.WriteFile("profiles/repo_name", []byte(args.Name + "\n"), 0644); err != nil {
			return fmt.Errorf("failed to write profiles/repo_name: %w", err)
		}
		fmt.Printf("Created profiles/repo_name with '%s'\n", args.Name)
	} else {
		fmt.Printf("profiles/repo_name already exists, skipping\n")
	}

	// Create profiles/eapi
	if _, err := fs.Stat("profiles/eapi"); os.IsNotExist(err) {
		if err := fs.WriteFile("profiles/eapi", []byte(args.EapiVersion + "\n"), 0644); err != nil {
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
			if err := fs.WriteFile("metadata/layout.conf", content, 0644); err != nil {
				return fmt.Errorf("failed to write metadata/layout.conf: %w", err)
			}
			fmt.Printf("Created metadata/layout.conf\n")
		}
	} else {
		fmt.Printf("metadata/layout.conf already exists, skipping\n")
	}

	return nil
}
