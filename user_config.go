package g2

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// UserConfigEntry represents a single configuration rule parsed from a Portage user config file.
type UserConfigEntry struct {
	RawLine    string
	AtomString string
	Atom       PackageAtom
	FilePath   string
	LineNumber int
}

// UserConfigFile represents a parsed Portage user config file.
type UserConfigFile struct {
	Path  string
	Lines []string
	Mode  os.FileMode
}

// ParseUserConfigFile reads and parses a user configuration file (e.g. package.mask, package.unmask).
// If the file does not exist, it returns nil, nil, nil.
func ParseUserConfigFile(path string) (*UserConfigFile, []UserConfigEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading file info %s: %w", path, err)
	}

	if info.IsDir() {
		return nil, nil, fmt.Errorf("%s is a directory, not a file", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Split lines preserving content structure
	rawLines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	hasTrailingNewline := strings.HasSuffix(string(content), "\n")
	if hasTrailingNewline && len(rawLines) > 0 && rawLines[len(rawLines)-1] == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	ucf := &UserConfigFile{
		Path:  path,
		Lines: rawLines,
		Mode:  info.Mode().Perm(),
	}

	var entries []UserConfigEntry
	for i, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Extract atom portion before inline comment
		candidate := trimmed
		if idx := strings.Index(candidate, "#"); idx != -1 {
			candidate = strings.TrimSpace(candidate[:idx])
		}
		fields := strings.Fields(candidate)
		if len(fields) == 0 {
			continue
		}
		atomCandidate := fields[0]
		atom := ParsePackageAtom(atomCandidate)
		if atom.Name != "" {
			entries = append(entries, UserConfigEntry{
				RawLine:    line,
				AtomString: atomCandidate,
				Atom:       atom,
				FilePath:   path,
				LineNumber: i + 1,
			})
		}
	}

	return ucf, entries, nil
}

// ReadUserConfigEntries reads and parses all entries from a Portage configuration target
// (either a single file or a directory of files). Non-existent targets return an empty slice and nil error.
func ReadUserConfigEntries(path string) ([]UserConfigEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	if !info.IsDir() {
		_, entries, err := ParseUserConfigFile(path)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		return entries, nil
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", path, err)
	}

	// Sort directory entries by name for deterministic ordering
	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	var allEntries []UserConfigEntry
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".bak") {
			continue
		}
		filePath := filepath.Join(path, name)
		_, entries, err := ParseUserConfigFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filePath, err)
		}
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

// SafeWriteFileAtomic safely writes data to targetPath by writing to a temporary file
// in the same directory and atomically renaming it into place. Existing permissions are preserved.
func SafeWriteFileAtomic(targetPath string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if mode == 0 {
		mode = 0644
	}

	tmpFile, err := os.CreateTemp(dir, ".g2-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	cleanedUp := false
	defer func() {
		if !cleanedUp {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(mode); err != nil {
		return fmt.Errorf("chmod %s: %w", tmpPath, err)
	}

	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("writing temp file %s: %w", tmpPath, err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("syncing temp file %s: %w", tmpPath, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpPath, targetPath, err)
	}

	cleanedUp = true
	return nil
}

// AddUserConfigAtom appends atomStr to the appropriate Portage user configuration file under configPath.
// If the atom already exists anywhere within configPath, no change is made and it returns false, targetFile, nil.
// If configPath is a directory, it targets configPath/g2.conf.
// If configPath is an existing file, it targets that file.
// If configPath does not exist, it creates configPath as a directory containing g2.conf.
// Returns true, targetFile, nil if successfully added.
func AddUserConfigAtom(configPath string, atomStr string) (bool, string, error) {
	// Check existing entries across all fragments in configPath
	existing, err := ReadUserConfigEntries(configPath)
	if err != nil {
		return false, "", fmt.Errorf("checking existing entries in %s: %w", configPath, err)
	}

	// Idempotency: check if exact atom exists
	for _, entry := range existing {
		if entry.AtomString == atomStr {
			return false, entry.FilePath, nil
		}
	}

	// Determine target file
	var targetFile string
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// By convention, create configPath as a directory containing g2.conf
			if err := os.MkdirAll(configPath, 0755); err != nil {
				return false, "", fmt.Errorf("creating directory %s: %w", configPath, err)
			}
			targetFile = filepath.Join(configPath, "g2.conf")
		} else {
			return false, "", fmt.Errorf("stat %s: %w", configPath, err)
		}
	} else if info.IsDir() {
		targetFile = filepath.Join(configPath, "g2.conf")
	} else {
		targetFile = configPath
	}

	var mode os.FileMode = 0644
	var existingContent []byte
	tInfo, err := os.Stat(targetFile)
	if err == nil {
		mode = tInfo.Mode().Perm()
		existingContent, err = os.ReadFile(targetFile)
		if err != nil {
			return false, "", fmt.Errorf("reading %s: %w", targetFile, err)
		}
	} else if !os.IsNotExist(err) {
		return false, "", fmt.Errorf("stat target %s: %w", targetFile, err)
	}

	var newContent []byte
	if len(existingContent) > 0 {
		newContent = append(newContent, existingContent...)
		if !strings.HasSuffix(string(existingContent), "\n") {
			newContent = append(newContent, '\n')
		}
	}
	newContent = append(newContent, []byte(atomStr+"\n")...)

	if err := SafeWriteFileAtomic(targetFile, newContent, mode); err != nil {
		return false, "", fmt.Errorf("writing %s: %w", targetFile, err)
	}

	return true, targetFile, nil
}

type lineByteRange struct {
	start    int
	end      int
	delimEnd int
	delimLen int
	isMatch  bool
}

func scanLineByteRanges(data []byte, matchAtom string) ([]lineByteRange, int) {
	var ranges []lineByteRange
	matchCount := 0
	n := len(data)
	start := 0

	for start < n {
		end := start
		for end < n && data[end] != '\n' && data[end] != '\r' {
			end++
		}
		delimEnd := end
		delimLen := 0
		if delimEnd < n {
			if data[delimEnd] == '\r' && delimEnd+1 < n && data[delimEnd+1] == '\n' {
				delimEnd += 2
				delimLen = 2
			} else if data[delimEnd] == '\n' {
				delimEnd += 1
				delimLen = 1
			} else if data[delimEnd] == '\r' {
				delimEnd += 1
				delimLen = 1
			}
		}

		lineStr := string(data[start:end])
		trimmed := strings.TrimSpace(lineStr)
		isMatch := false
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			candidate := trimmed
			if idx := strings.Index(candidate, "#"); idx != -1 {
				candidate = strings.TrimSpace(candidate[:idx])
			}
			fields := strings.Fields(candidate)
			if len(fields) > 0 && fields[0] == matchAtom {
				isMatch = true
				matchCount++
			}
		}

		ranges = append(ranges, lineByteRange{
			start:    start,
			end:      end,
			delimEnd: delimEnd,
			delimLen: delimLen,
			isMatch:  isMatch,
		})

		start = delimEnd
	}

	return ranges, matchCount
}

func removeMatchingLinesBytes(data []byte, matchAtom string) ([]byte, int) {
	ranges, matchCount := scanLineByteRanges(data, matchAtom)
	if matchCount == 0 {
		return data, 0
	}

	isLastOriginalLineWithoutDelim := len(ranges) > 0 && ranges[len(ranges)-1].delimLen == 0

	var buf bytes.Buffer
	for i, r := range ranges {
		if r.isMatch {
			continue
		}

		isLastKeptLine := true
		for j := i + 1; j < len(ranges); j++ {
			if !ranges[j].isMatch {
				isLastKeptLine = false
				break
			}
		}

		if isLastKeptLine && isLastOriginalLineWithoutDelim {
			buf.Write(data[r.start:r.end])
		} else {
			buf.Write(data[r.start:r.delimEnd])
		}
	}

	return buf.Bytes(), matchCount
}

// RemoveUserConfigAtom removes all occurrences of atomStr from all files under configPath,
// preserving untouched bytes (such as CRLF line endings and absence of trailing newlines).
// Returns the count of removed occurrences and error if any.
func RemoveUserConfigAtom(configPath string, atomStr string) (int, error) {
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat %s: %w", configPath, err)
	}

	var filesToProcess []string
	if !info.IsDir() {
		filesToProcess = []string{configPath}
	} else {
		entries, err := os.ReadDir(configPath)
		if err != nil {
			return 0, fmt.Errorf("reading directory %s: %w", configPath, err)
		}
		for _, de := range entries {
			if de.IsDir() {
				continue
			}
			name := de.Name()
			if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".bak") {
				continue
			}
			filesToProcess = append(filesToProcess, filepath.Join(configPath, name))
		}
	}

	totalRemoved := 0
	for _, filePath := range filesToProcess {
		fInfo, err := os.Stat(filePath)
		if err != nil {
			return totalRemoved, fmt.Errorf("stat %s: %w", filePath, err)
		}
		mode := fInfo.Mode().Perm()

		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			return totalRemoved, fmt.Errorf("reading %s: %w", filePath, err)
		}

		newBytes, removed := removeMatchingLinesBytes(contentBytes, atomStr)
		if removed > 0 {
			totalRemoved += removed
			if err := SafeWriteFileAtomic(filePath, newBytes, mode); err != nil {
				return totalRemoved, fmt.Errorf("writing %s: %w", filePath, err)
			}
		}
	}

	return totalRemoved, nil
}
