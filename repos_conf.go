package g2

import (

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
