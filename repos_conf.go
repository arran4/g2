package g2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ReposConf struct {
	Path  string
	IsDir bool
	Files []*ReposConfFile
}

type ReposConfFile struct {
	Path        string
	HeaderLines []string
	Sections    []*ReposConfSection
}

type ReposConfSection struct {
	Name     string
	Disabled bool
	Lines    []string
}

func ParseReposConf(path string) (*ReposConf, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// If it doesn't exist, we can create an empty ReposConf
			// but we need to know if it's meant to be a dir or file.
			// Let's assume file by default, or the caller handles it.
			return &ReposConf{Path: path, IsDir: false}, nil
		}
		return nil, err
	}

	rc := &ReposConf{Path: path, IsDir: info.IsDir()}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fPath := filepath.Join(path, entry.Name())
			f, err := ParseReposConfFile(fPath)
			if err == nil {
				rc.Files = append(rc.Files, f)
			}
		}
	} else {
		f, err := ParseReposConfFile(path)
		if err == nil {
			rc.Files = append(rc.Files, f)
		}
	}
	return rc, nil
}

func ParseReposConfFile(path string) (*ReposConfFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// split by newline, handle both \r\n and \n
	lines := strings.Split(strings.ReplaceAll(string(bytes), "\r\n", "\n"), "\n")

	file := &ReposConfFile{Path: path}
	var currentSection *ReposConfSection

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		isHeader := false
		var name string
		disabled := false

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			isHeader = true
			name = trimmed[1 : len(trimmed)-1]
		} else if strings.HasPrefix(trimmed, "#") {
			inner := strings.TrimSpace(trimmed[1:])
			if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
				isHeader = true
				name = inner[1 : len(inner)-1]
				disabled = true
			}
		}

		if isHeader {
			if currentSection != nil {
				file.Sections = append(file.Sections, currentSection)
			}
			currentSection = &ReposConfSection{Name: name, Disabled: disabled}
			continue
		}

		if currentSection == nil {
			file.HeaderLines = append(file.HeaderLines, line)
		} else {
			currentSection.Lines = append(currentSection.Lines, line)
		}
	}
	if currentSection != nil {
		file.Sections = append(file.Sections, currentSection)
	}

	return file, nil
}

func (f *ReposConfFile) Write() error {
	var out []string
	out = append(out, f.HeaderLines...)
	for _, sec := range f.Sections {
		prefix := ""
		if sec.Disabled {
			prefix = "# "
		}
		out = append(out, prefix+"["+sec.Name+"]")
		out = append(out, sec.Lines...)
	}

	// Avoid trailing newline duplication
	content := strings.Join(out, "\n")
	if len(out) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return os.WriteFile(f.Path, []byte(content), 0644)
}

func (s *ReposConfSection) Disable() {
	if s.Disabled {
		return
	}
	s.Disabled = true
	for i, line := range s.Lines {
		s.Lines[i] = "# " + line
	}
}

func (s *ReposConfSection) Enable() {
	if !s.Disabled {
		return
	}
	s.Disabled = false
	for i, line := range s.Lines {
		if strings.HasPrefix(line, "# ") {
			s.Lines[i] = line[2:]
		} else if strings.HasPrefix(line, "#") {
			s.Lines[i] = line[1:]
		}
	}
}

func (f *ReposConfFile) Disable() error {
	base := filepath.Base(f.Path)
	if strings.HasPrefix(base, ".") {
		return nil
	}
	newPath := filepath.Join(filepath.Dir(f.Path), "."+base)
	if err := os.Rename(f.Path, newPath); err != nil {
		return err
	}
	f.Path = newPath
	return nil
}

func (f *ReposConfFile) Enable() error {
	base := filepath.Base(f.Path)
	if !strings.HasPrefix(base, ".") {
		return nil
	}
	newPath := filepath.Join(filepath.Dir(f.Path), strings.TrimPrefix(base, "."))
	if err := os.Rename(f.Path, newPath); err != nil {
		return err
	}
	f.Path = newPath
	return nil
}

func (s *ReposConfSection) Get(key string) string {
	for _, line := range s.Lines {
		trimmed := strings.TrimSpace(line)
		if s.Disabled {
			if strings.HasPrefix(trimmed, "#") {
				trimmed = strings.TrimSpace(trimmed[1:])
			}
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func (s *ReposConfSection) Set(key, value string) {
	found := false
	for i, line := range s.Lines {
		trimmed := strings.TrimSpace(line)
		prefix := ""
		if s.Disabled && strings.HasPrefix(trimmed, "#") {
			prefix = "# "
			trimmed = strings.TrimSpace(trimmed[1:])
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			s.Lines[i] = prefix + key + " = " + value
			found = true
			break
		}
	}
	if !found {
		prefix := ""
		if s.Disabled {
			prefix = "# "
		}
		s.Lines = append(s.Lines, prefix+key+" = "+value)
	}
}

func (s *ReposConfSection) Unset(key string) {
	var newLines []string
	for _, line := range s.Lines {
		trimmed := strings.TrimSpace(line)
		if s.Disabled && strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimSpace(trimmed[1:])
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			continue
		}
		newLines = append(newLines, line)
	}
	s.Lines = newLines
}

// ValidateRepoName validates a repository name according to Gentoo PMS rules.
// Repository names must:
// - be non-empty;
// - contain only [A-Za-z0-9_-];
// - not begin with '-';
// - not begin with '+' or '.';
// - contain no whitespace, CR, LF, NUL, or other delimiters.
func ValidateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("repository name cannot be empty")
	}
	if strings.ContainsAny(name, " \t\r\n\x00") {
		return fmt.Errorf("invalid repository name %q: contains whitespace or control characters", name)
	}
	if name[0] == '-' || name[0] == '+' || name[0] == '.' {
		return fmt.Errorf("invalid repository name %q: must not begin with hyphen, plus, or dot", name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return fmt.Errorf("invalid repository name %q: contains invalid character %q", name, string(c))
	}
	return nil
}

// RepoInfo describes a configured repository found in repos.conf.
type RepoInfo struct {
	Name       string // Section name in repos.conf
	RepoName   string // Actual repository identity (from profiles/repo_name if present, else section name)
	Location   string // Repository location directory
	Disabled   bool   // True if disabled in repos.conf
	ConfigFile string // Path to repos.conf file containing this section
}

// ListConfiguredRepos returns all active (enabled) repositories from repos.conf at location.
func ListConfiguredRepos(location string) ([]*RepoInfo, error) {
	rc, err := ParseReposConf(location)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("parsing repos.conf at %s: %w", location, err)
	}

	var repos []*RepoInfo
	for _, f := range rc.Files {
		for _, s := range f.Sections {
			if strings.EqualFold(s.Name, "DEFAULT") {
				continue
			}
			disabled := s.Disabled ||
				strings.EqualFold(s.Get("disabled"), "true") ||
				strings.EqualFold(s.Get("disabled"), "yes") ||
				strings.EqualFold(s.Get("enabled"), "false") ||
				strings.EqualFold(s.Get("enabled"), "no")
			if disabled {
				continue
			}
			loc := s.Get("location")
			repoIdentity := s.Name
			if loc != "" {
				repoNameFile := filepath.Join(loc, "profiles", "repo_name")
				data, err := os.ReadFile(repoNameFile)
				if err != nil {
					if !os.IsNotExist(err) {
						return nil, fmt.Errorf("reading repository identity at %s: %w", repoNameFile, err)
					}
				} else {
					content := string(data)
					lines := strings.Split(strings.TrimRight(content, "\r\n"), "\n")
					if len(lines) == 0 || (len(lines) == 1 && strings.TrimSpace(lines[0]) == "") {
						return nil, fmt.Errorf("invalid repository identity in %s: file is empty", repoNameFile)
					}
					if len(lines) > 1 {
						return nil, fmt.Errorf("invalid repository identity in %s: contains multiple lines", repoNameFile)
					}
					trimmed := strings.TrimSpace(lines[0])
					if err := ValidateRepoName(trimmed); err != nil {
						return nil, fmt.Errorf("invalid repository identity in %s: %w", repoNameFile, err)
					}
					repoIdentity = trimmed
				}
			}
			if err := ValidateRepoName(repoIdentity); err != nil {
				return nil, fmt.Errorf("invalid repository identity for section [%s]: %w", s.Name, err)
			}
			repos = append(repos, &RepoInfo{
				Name:       s.Name,
				RepoName:   repoIdentity,
				Location:   loc,
				Disabled:   disabled,
				ConfigFile: f.Path,
			})
		}
	}
	return repos, nil
}

// ResolveRepo finds a single configured repository by section name or Portage repository identity.
func ResolveRepo(repoName string, reposConfPath string) (*RepoInfo, error) {
	repos, err := ListConfiguredRepos(reposConfPath)
	if err != nil {
		return nil, err
	}

	var matches []*RepoInfo
	for _, r := range repos {
		if r.Name == repoName || r.RepoName == repoName {
			matches = append(matches, r)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("repository %q not found or not enabled in %s", repoName, reposConfPath)
	}

	if len(matches) > 1 {
		// Check if they are identical in identity and location
		first := matches[0]
		for _, m := range matches[1:] {
			if m.RepoName != first.RepoName || m.Location != first.Location {
				return nil, fmt.Errorf("ambiguous repository %q matches multiple configured repositories in %s", repoName, reposConfPath)
			}
		}
	}

	return matches[0], nil
}
