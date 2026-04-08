package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/arran4/g2"
	"github.com/arran4/g2/cmd/g2/internal/cacheconfig"
	"golang.org/x/tools/txtar"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type CmdPackageArgConfig struct {
	*MainArgConfig
}

func (cfg *MainArgConfig) cmdPackage(args []string) error {
	fs := flag.NewFlagSet("package", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fmt.Printf("\t\t %s \t\t %s\n", "search", "search packages")
		fmt.Printf("\t\t %s \t\t %s\n", "index-overlay", "index a single overlay")
		fmt.Printf("\t\t %s \t\t %s\n", "index-repositories", "index multiple repositories from an xml file")
		fmt.Printf("\t\t %s \t\t %s\n", "index", "index local repositories")
		fmt.Printf("\t\t %s \t\t %s\n", "update", "update the local index from a remote zip file")
		fmt.Printf("\t\t %s \t\t %s\n", "deprecated", "commands relating to deprecated packages")
		fmt.Printf("\t\t %s \t\t %s\n", "masked", "commands relating to masked packages")
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing subcommand")
	}

	cmd := fs.Arg(0)
	cfg.Args = append(cfg.Args, cmd)

	config := &CmdPackageArgConfig{
		MainArgConfig: cfg,
	}

	switch cmd {
	case "search":
		if err := config.cmdSearch(fs.Args()[1:]); err != nil {
			return err
		}
	case "index-overlay":
		if err := config.cmdIndexOverlay(fs.Args()[1:]); err != nil {
			return err
		}
	case "index-repositories":
		if err := config.cmdIndexRepositories(fs.Args()[1:]); err != nil {
			return err
		}
	case "index":
		if err := config.cmdIndex(fs.Args()[1:]); err != nil {
			return err
		}
	case "update":
		if err := config.cmdUpdate(fs.Args()[1:]); err != nil {
			return err
		}
	case "deprecated":
		if err := config.cmdDeprecated(fs.Args()[1:]); err != nil {
			return err
		}
	case "masked":
		if err := config.cmdMasked(fs.Args()[1:]); err != nil {
			return err
		}
	case "help", "-help", "--help":
		fs.Usage()
		os.Exit(-1)
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %s", cmd)
	}
	return nil
}

func (cfg *CmdPackageArgConfig) cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	path := fs.String("path", "", "Path to the search index directory, zip, tar, txtar, or URL")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s <query>\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing query")
	}

	query := strings.Join(fs.Args(), " ")
	searchPath := *path

	if searchPath == "" {
		searchPath = fmt.Sprintf("%s/g2/search", cacheconfig.GetCacheDir())
	}

	engine := NewSearchEngine()

	// Handle loading logic based on type
	if strings.HasPrefix(searchPath, "http://") || strings.HasPrefix(searchPath, "https://") {
		// Load from URL
		// For simplicity we try to fetch manifest.json then data files

		manifestURL := fmt.Sprintf("%s/data/manifest.json", strings.TrimRight(searchPath, "/"))
		resp, err := http.Get(manifestURL)
		if err != nil {
			return fmt.Errorf("fetching manifest: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		var manifest SearchManifest
		if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
			return fmt.Errorf("decoding manifest: %w", err)
		}

		for _, file := range manifest.DataFiles {
			dataURL := fmt.Sprintf("%s/data/%s", strings.TrimRight(searchPath, "/"), file)
			dataResp, err := http.Get(dataURL)
			if err != nil {
				return fmt.Errorf("fetching data file %s: %w", file, err)
			}
			defer func() { _ = dataResp.Body.Close() }()

			var docs []SearchDocument
			if err := json.NewDecoder(dataResp.Body).Decode(&docs); err != nil {
				return fmt.Errorf("decoding data file %s: %w", file, err)
			}
			engine.LoadDocuments(docs)
		}
	} else if strings.HasSuffix(searchPath, ".zip") {
		z, err := zip.OpenReader(searchPath)
		if err != nil {
			return fmt.Errorf("opening zip file: %w", err)
		}
		defer func() { _ = z.Close() }()

		var manifest SearchManifest
		var manifestFound bool

		for _, f := range z.File {
			if strings.HasSuffix(f.Name, "manifest.json") {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				err = json.NewDecoder(rc).Decode(&manifest)
				_ = rc.Close()
				if err != nil {
					return fmt.Errorf("decoding manifest from zip: %w", err)
				}
				manifestFound = true
				break
			}
		}

		if !manifestFound {
			return fmt.Errorf("manifest.json not found in zip")
		}

		for _, f := range z.File {
			for _, mf := range manifest.DataFiles {
				if strings.HasSuffix(f.Name, mf) {
					rc, err := f.Open()
					if err != nil {
						return err
					}
					var docs []SearchDocument
					err = json.NewDecoder(rc).Decode(&docs)
					_ = rc.Close()
					if err != nil {
						return fmt.Errorf("decoding data file %s from zip: %w", mf, err)
					}
					engine.LoadDocuments(docs)
				}
			}
		}
	} else if strings.HasSuffix(searchPath, ".tar") || strings.HasSuffix(searchPath, ".tar.gz") {
		f, err := os.Open(searchPath)
		if err != nil {
			return fmt.Errorf("opening tar file: %w", err)
		}
		defer func() { _ = f.Close() }()

		var tr *tar.Reader
		if strings.HasSuffix(searchPath, ".tar.gz") {
			gzr, err := gzip.NewReader(f)
			if err != nil {
				return fmt.Errorf("opening gzip reader: %w", err)
			}
			defer func() { _ = gzr.Close() }()
			tr = tar.NewReader(gzr)
		} else {
			tr = tar.NewReader(f)
		}

		var manifest SearchManifest
		var manifestFound bool
		var filesData = make(map[string][]byte)

		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("reading tar: %w", err)
			}
			if hdr.Typeflag != tar.TypeReg {
				continue
			}
			data, err := io.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("reading file %s from tar: %w", hdr.Name, err)
			}
			filesData[hdr.Name] = data

			if strings.HasSuffix(hdr.Name, "manifest.json") {
				if err := json.Unmarshal(data, &manifest); err != nil {
					return fmt.Errorf("decoding manifest from tar: %w", err)
				}
				manifestFound = true
			}
		}

		if !manifestFound {
			return fmt.Errorf("manifest.json not found in tar")
		}

		for name, data := range filesData {
			for _, mf := range manifest.DataFiles {
				if strings.HasSuffix(name, mf) {
					var docs []SearchDocument
					if err := json.Unmarshal(data, &docs); err != nil {
						return fmt.Errorf("decoding data file %s from tar: %w", mf, err)
					}
					engine.LoadDocuments(docs)
				}
			}
		}
	} else if strings.HasSuffix(searchPath, ".txtar") {
		archive, err := txtar.ParseFile(searchPath)
		if err != nil {
			return fmt.Errorf("parsing txtar file: %w", err)
		}

		var manifest SearchManifest
		var manifestFound bool

		for _, f := range archive.Files {
			if strings.HasSuffix(f.Name, "manifest.json") {
				if err := json.Unmarshal(f.Data, &manifest); err != nil {
					return fmt.Errorf("decoding manifest from txtar: %w", err)
				}
				manifestFound = true
				break
			}
		}

		if !manifestFound {
			return fmt.Errorf("manifest.json not found in txtar")
		}

		for _, f := range archive.Files {
			for _, mf := range manifest.DataFiles {
				if strings.HasSuffix(f.Name, mf) {
					var docs []SearchDocument
					if err := json.Unmarshal(f.Data, &docs); err != nil {
						return fmt.Errorf("decoding data file %s from txtar: %w", mf, err)
					}
					engine.LoadDocuments(docs)
				}
			}
		}
	} else {
		// Directory

		dataDir := filepath.Join(searchPath, "data")
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			// maybe it's the data dir itself
			dataDir = searchPath
		}

		manifestPath := filepath.Join(dataDir, "manifest.json")
		mf, err := os.Open(manifestPath)
		if err != nil {
			return fmt.Errorf("opening manifest: %w", err)
		}
		defer func() { _ = mf.Close() }()

		var manifest SearchManifest
		if err := json.NewDecoder(mf).Decode(&manifest); err != nil {
			return fmt.Errorf("decoding manifest: %w", err)
		}

		for _, file := range manifest.DataFiles {
			dataPath := filepath.Join(dataDir, file)
			df, err := os.Open(dataPath)
			if err != nil {
				return fmt.Errorf("opening data file %s: %w", file, err)
			}
			var docs []SearchDocument
			if err := json.NewDecoder(df).Decode(&docs); err != nil {
				_ = df.Close()
				return fmt.Errorf("decoding data file %s: %w", file, err)
			}
			_ = df.Close()
			engine.LoadDocuments(docs)
		}
	}

	results := engine.Search(query)

	for _, res := range results {
		fmt.Printf("%s\n", res.FullName)
		if res.Description != "" {
			fmt.Printf("  %s\n", res.Description)
		}
		if res.Version != "" {
			fmt.Printf("  Version: %s\n", res.Version)
		}
	}

	log.Printf("Found %d results for '%s'", len(results), query)
	return nil
}

func (cfg *CmdPackageArgConfig) cmdIndexOverlay(args []string) error {
	fs := flag.NewFlagSet("index-overlay", flag.ExitOnError)
	outDir := fs.String("out-dir", "", "Directory to write the search index")
	fs.StringVar(outDir, "o", "", "Directory to write the search index (shorthand)")
	outZip := fs.String("out-zip", "", "Zip file to write the search index")
	fs.StringVar(outZip, "z", "", "Zip file to write the search index (shorthand)")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s <overlay-path>\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing overlay path")
	}

	overlayPath := fs.Arg(0)

	if *outDir == "" && *outZip == "" {
		*outDir = fmt.Sprintf("%s/g2/search", cacheconfig.GetCacheDir())
	}

	log.Printf("Indexing overlay: %s", overlayPath)

	sysFS := os.DirFS(overlayPath)
	repo, err := parseRepo(sysFS, overlayPath, "local", false, nil)
	if err != nil {
		return fmt.Errorf("failed to parse repo: %w", err)
	}

	sites := []*g2.SiteData{repo}

	if err := generateSearchData(*outDir, *outZip, sites); err != nil {
		return fmt.Errorf("generating search data: %w", err)
	}

	log.Printf("Done generating search data.")
	return nil
}

func (cfg *CmdPackageArgConfig) cmdIndexRepositories(args []string) error {
	fs := flag.NewFlagSet("index-repositories", flag.ExitOnError)
	outDir := fs.String("out-dir", "", "Directory to write the search index")
	fs.StringVar(outDir, "o", "", "Directory to write the search index (shorthand)")
	outZip := fs.String("out-zip", "", "Zip file to write the search index")
	fs.StringVar(outZip, "z", "", "Zip file to write the search index (shorthand)")
	repoFilter := fs.String("repo-filter", "", "Comma separated list of repos to filter by")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s <repositories.xml>\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("missing repositories.xml path or url")
	}

	reposFile := fs.Arg(0)

	if *outDir == "" && *outZip == "" {
		*outDir = fmt.Sprintf("%s/g2/search", cacheconfig.GetCacheDir())
	}

	var allowRepos map[string]bool
	if *repoFilter != "" {
		allowRepos = make(map[string]bool)
		for _, repo := range strings.Split(*repoFilter, ",") {
			allowRepos[strings.TrimSpace(repo)] = true
		}
	}

	log.Printf("Indexing repositories from: %s", reposFile)

	var repos []g2.Repository
	reposXml, err := os.ReadFile(reposFile)
	if err != nil {
		return fmt.Errorf("reading repos.xml: %w", err)
	}
	reposObj, err := g2.ParseRepositoriesBytes(reposXml)
	if err != nil {
		return fmt.Errorf("parsing repos.xml: %w", err)
	}
	repos = reposObj.Repositories

	var sites []*g2.SiteData

	for _, rep := range repos {
		if allowRepos != nil && !allowRepos[rep.Name] {
			continue
		}
		if len(rep.Sources) == 0 {
			continue
		}
		overlayPath := rep.Sources[0].Text
		sysFS := os.DirFS(overlayPath)
		repCopy := rep
		r, err := parseRepo(sysFS, overlayPath, rep.Name, false, &repCopy)
		if err != nil {
			log.Printf("failed to parse repo %s: %v", overlayPath, err)
			continue
		}
		sites = append(sites, r)
	}

	if err := generateSearchData(*outDir, *outZip, sites); err != nil {
		return fmt.Errorf("generating search data: %w", err)
	}

	log.Printf("Done generating search data.")
	return nil
}

func (cfg *CmdPackageArgConfig) cmdIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	outDir := fs.String("out-dir", "", "Directory to write the search index")
	fs.StringVar(outDir, "o", "", "Directory to write the search index (shorthand)")
	outZip := fs.String("out-zip", "", "Zip file to write the search index")
	fs.StringVar(outZip, "z", "", "Zip file to write the search index (shorthand)")
	repoFilter := fs.String("repo-filter", "", "Comma separated list of repos to filter by")
	reposConfOpt := fs.String("repos-conf", "", "Path to repos.conf file or directory")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if *outDir == "" && *outZip == "" {
		*outDir = fmt.Sprintf("%s/g2/search", cacheconfig.GetCacheDir())
	}

	var allowRepos map[string]bool
	if *repoFilter != "" {
		allowRepos = make(map[string]bool)
		for _, repo := range strings.Split(*repoFilter, ",") {
			allowRepos[strings.TrimSpace(repo)] = true
		}
	}

	var overlayPaths []string

	if *reposConfOpt != "" {
		rc, err := g2.ParseReposConf(*reposConfOpt)
		if err != nil {
			return fmt.Errorf("parsing repos.conf: %w", err)
		}
		for _, f := range rc.Files {
			for _, s := range f.Sections {
				if s.Name == "DEFAULT" || s.Disabled {
					continue
				}
				if allowRepos != nil && !allowRepos[s.Name] {
					continue
				}
				loc := s.Get("location")
				if loc != "" {
					overlayPaths = append(overlayPaths, loc)
				}
			}
		}
		log.Printf("Indexing %d repositories from repos.conf %s", len(overlayPaths), *reposConfOpt)
	} else {
		reposDir := "/var/db/repos"
		log.Printf("Indexing repositories from: %s", reposDir)

		entries, err := os.ReadDir(reposDir)
		if err != nil {
			return fmt.Errorf("reading repos directory %s: %w", reposDir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			repoName := entry.Name()
			if allowRepos != nil && !allowRepos[repoName] {
				continue
			}
			overlayPaths = append(overlayPaths, fmt.Sprintf("%s/%s", reposDir, repoName))
		}
	}

	var sites []*g2.SiteData
	for _, overlayPath := range overlayPaths {
		sysFS := os.DirFS(overlayPath)
		repo, err := parseRepo(sysFS, overlayPath, "local", false, nil)
		if err != nil {
			log.Printf("failed to parse repo %s: %v", overlayPath, err)
			continue
		}
		sites = append(sites, repo)
	}

	if err := generateSearchData(*outDir, *outZip, sites); err != nil {
		return fmt.Errorf("generating search data: %w", err)
	}

	log.Printf("Done generating search data.")
	return nil
}

func (cfg *CmdPackageArgConfig) cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	outDir := fs.String("out-dir", "", "Directory to write the search index (default: standard cache dir)")
	fs.StringVar(outDir, "o", "", "Directory to write the search index (shorthand)")
	urlOpt := fs.String("url", "https://github.com/arran4/gentoo-packages/releases/latest/download/search-data.zip", "URL to the search data zip file")

	fs.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("\t%s\n", strings.Join(cfg.Args, " "))
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	if *outDir == "" {
		*outDir = fmt.Sprintf("%s/g2/search", cacheconfig.GetCacheDir())
	}

	log.Printf("Updating search index from %s to %s", *urlOpt, *outDir)

	resp, err := http.Get(*urlOpt)
	if err != nil {
		return fmt.Errorf("downloading search data: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download search data, status code: %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "search-data-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("writing to temp file: %w", err)
	}

	_ = tmpFile.Close()

	z, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("opening zip file: %w", err)
	}
	defer func() { _ = z.Close() }()

	dataDir := filepath.Join(*outDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating search data directory: %w", err)
	}

	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("opening file %s from zip: %w", f.Name, err)
		}

		destPath := filepath.Join(dataDir, filepath.Base(f.Name))
		destFile, err := os.Create(destPath)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("creating file %s: %w", destPath, err)
		}

		if _, err := io.Copy(destFile, rc); err != nil {
			_ = destFile.Close()
			_ = rc.Close()
			return fmt.Errorf("writing file %s: %w", destPath, err)
		}

		_ = destFile.Close()
		_ = rc.Close()

		log.Printf("Extracted %s", destPath)
	}

	log.Printf("Done updating search index.")
	return nil
}
