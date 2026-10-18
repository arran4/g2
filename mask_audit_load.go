package g2

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// MaskAuditOptions selects only input paths. Loading an audit creates no files,
// refreshes no caches, and never executes an ebuild or profile shell script.
type MaskAuditOptions struct {
	ConfigRoot   string
	ReposConf    string
	Profile      string
	Repositories []*RepoInfo
}

// readAuditRules uses the provenance parser, retaining unsupported active lines
// as problems rather than silently dropping them. Directory fragments are read
// recursively in lexical order, excluding dotfiles and ~ / .bak backups.
func readAuditRules(path string, user bool, repo string) ([]MaskRule, error) {
	var rules []MaskRule
	target := path
	visiting := map[string]bool{}
	seenFiles := map[string]bool{}
	var read func(string) error
	read = func(path string) error {
		info, err := os.Stat(path)
		if os.IsNotExist(err) && path == target {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect mask configuration %s: %w", path, err)
		}
		if info.IsDir() {
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("resolve configuration directory %s: %w", path, err)
			}
			if visiting[real] {
				return fmt.Errorf("configuration directory cycle at %s", path)
			}
			visiting[real] = true
			defer delete(visiting, real)
			entries, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("read mask directory %s: %w", path, err)
			}
			for _, e := range entries {
				if auditIgnoredFile(e.Name()) {
					continue
				}
				if err := read(filepath.Join(path, e.Name())); err != nil {
					return err
				}
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("mask configuration %s is not a regular file", path)
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve mask configuration %s: %w", path, err)
		}
		if seenFiles[real] {
			return fmt.Errorf("mask configuration %s is reached through multiple paths", path)
		}
		seenFiles[real] = true
		file, _, err := ParseUserConfigFile(path)
		if err != nil {
			return fmt.Errorf("parse mask configuration %s: %w", path, err)
		}
		if file == nil {
			return fmt.Errorf("mask configuration %s disappeared during the audit", path)
		}
		for i, raw := range file.Lines {
			line, _, _ := strings.Cut(raw, "#")
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			rule := MaskRule{Atom: fields[0], File: path, Line: i + 1, User: user, Repository: repo}
			if len(fields) != 1 {
				rule.Problem = "unsupported extra fields in mask rule"
			}
			rules = append(rules, rule)
		}
		return nil
	}
	if err := read(path); err != nil {
		return nil, err
	}
	return rules, nil
}

func auditIgnoredFile(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") || strings.HasSuffix(name, ".bak")
}

// stackAuditRules implements exact negative incrementals and -* resets. Scope
// has already been established by the caller. Keep positive duplicates so each
// physical user line can be audited independently.
func stackAuditRules(previous, next []MaskRule, problems *[]string, ignoreRepo bool) []MaskRule {
	for _, rule := range next {
		if !strings.HasPrefix(rule.Atom, "-") {
			previous = append(previous, rule)
			continue
		}
		atom := strings.TrimPrefix(rule.Atom, "-")
		if rule.Problem != "" {
			*problems = append(*problems, fmt.Sprintf("%s:%d: %s", rule.File, rule.Line, rule.Problem))
			continue
		}
		if atom == "*" {
			previous = nil
			continue
		}
		if _, err := MatchMaskAtom(atom, MaskCandidate{}); err != nil {
			*problems = append(*problems, fmt.Sprintf("%s:%d: unsupported removal: %v", rule.File, rule.Line, err))
			continue
		}
		kept := previous[:0]
		for _, old := range previous {
			key := old.Atom
			if ignoreRepo {
				a := ParsePackageAtom(old.Atom)
				if a.Repo == "" {
					a.Repo = old.Repository
				}
				if !strings.Contains(atom, "::") {
					a.Repo = ""
				}
				key = a.String()
			}
			if key != atom {
				kept = append(kept, old)
			}
		}
		previous = kept
	}
	return previous
}

func configuredAuditRepos(path string, problems *[]string) ([]*RepoInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect audit repos.conf %s: %w", path, err)
	}
	paths := []string{path}
	if info.IsDir() {
		paths = nil
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("read audit repos.conf directory %s: %w", path, err)
		}
		for _, e := range entries {
			if !e.IsDir() && !auditIgnoredFile(e.Name()) {
				paths = append(paths, filepath.Join(path, e.Name()))
			}
		}
	}
	var repos []*RepoInfo
	for _, path := range paths {
		// Unlike ParseReposConf, do not swallow unreadable configuration files.
		config, err := ParseReposConfFile(path)
		if err != nil {
			return nil, fmt.Errorf("read audit repository configuration %s: %w", path, err)
		}
		for _, section := range config.Sections {
			if section.Disabled {
				continue
			}
			for _, line := range section.Lines {
				key, _, ok := strings.Cut(strings.TrimSpace(line), "=")
				if ok && strings.TrimSpace(key) == "masters" {
					*problems = append(*problems, fmt.Sprintf("repository masters override in %s [%s] is unsupported", path, section.Name))
				}
			}
		}
		found, err := ListConfiguredRepos(path)
		if err != nil {
			return nil, fmt.Errorf("discover audit repositories in %s: %w", path, err)
		}
		repos = append(repos, found...)
	}
	return repos, nil
}

// LoadMaskAuditInput discovers repository versions and mask policy. Missing or
// ambiguous semantic inputs are reported as Problems, never as an empty policy.
func LoadMaskAuditInput(opts MaskAuditOptions) (MaskAuditInput, error) {
	var in MaskAuditInput
	if opts.ConfigRoot == "" {
		opts.ConfigRoot = "/etc/portage"
	}
	if opts.ReposConf == "" {
		opts.ReposConf = filepath.Join(opts.ConfigRoot, "repos.conf")
	}
	repos := opts.Repositories
	if repos == nil {
		var err error
		repos, err = configuredAuditRepos(opts.ReposConf, &in.Problems)
		if err != nil {
			return in, err
		}
	}
	if len(repos) == 0 {
		in.Problems = append(in.Problems, "no configured repositories; inventory is incomplete")
	}
	repos = append([]*RepoInfo(nil), repos...)
	for i, repo := range repos {
		if repo == nil {
			return in, fmt.Errorf("nil repository in audit configuration")
		}
		normalized := *repo
		if normalized.Location != "" {
			location, err := filepath.Abs(normalized.Location)
			if err != nil {
				return in, fmt.Errorf("resolve audit repository location %s: %w", normalized.Location, err)
			}
			normalized.Location = location
		}
		repos[i] = &normalized
	}
	repoByName := map[string]*RepoInfo{}
	layouts := map[string]*LayoutConf{}
	for _, repo := range repos {
		if repo.Location == "" || repo.RepoName == "" || repoByName[repo.RepoName] != nil {
			return in, fmt.Errorf("missing or ambiguous repository identity/location for %q", repo.RepoName)
		}
		if err := ValidateRepoName(repo.RepoName); err != nil {
			return in, fmt.Errorf("invalid audit repository identity: %w", err)
		}
		if identity, err := os.ReadFile(filepath.Join(repo.Location, "profiles", "repo_name")); err == nil {
			if strings.TrimSpace(string(identity)) != repo.RepoName {
				return in, fmt.Errorf("repository identity %q disagrees with profiles/repo_name in %s", repo.RepoName, repo.Location)
			}
		} else if !os.IsNotExist(err) {
			return in, fmt.Errorf("read audit repository identity: %w", err)
		}
		repoByName[repo.RepoName] = repo
		layout, err := ParseLayoutConf(filepath.Join(repo.Location, "metadata", "layout.conf"))
		if err != nil && !os.IsNotExist(err) {
			return in, fmt.Errorf("read audit layout for %s: %w", repo.RepoName, err)
		}
		if layout == nil {
			in.Problems = append(in.Problems, fmt.Sprintf("repository %s has no layout.conf; master inheritance is unknown", repo.RepoName))
			layout = &LayoutConf{}
		}
		layouts[repo.RepoName] = layout
		if eapi, err := os.ReadFile(filepath.Join(repo.Location, "profiles", "eapi")); err == nil {
			switch strings.TrimSpace(string(eapi)) {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8":
			default:
				in.Problems = append(in.Problems, fmt.Sprintf("unsupported repository profile EAPI for %s", repo.RepoName))
			}
		} else if !os.IsNotExist(err) {
			return in, fmt.Errorf("read repository profile EAPI for %s: %w", repo.RepoName, err)
		}
		hasMasters := false
		for _, entry := range layout.Entries {
			hasMasters = hasMasters || entry.Key == "masters"
		}
		if !hasMasters {
			in.Problems = append(in.Problems, fmt.Sprintf("repository %s has no explicit masters policy", repo.RepoName))
		}
		versions, err := inventoryMaskCandidates(repo)
		if err != nil {
			return in, err
		}
		in.Candidates = append(in.Candidates, versions...)
	}
	// Repository master masks apply to the child repository, not globally.
	for _, repo := range repos {
		var masks []MaskRule
		if len(layouts[repo.RepoName].Masters()) > 1 {
			in.Problems = append(in.Problems, fmt.Sprintf("multiple-master mask stacking for %s is unsupported", repo.RepoName))
		}
		for _, master := range layouts[repo.RepoName].Masters() {
			parent := repoByName[master]
			if parent == nil {
				in.Problems = append(in.Problems, fmt.Sprintf("unavailable master %s of %s", master, repo.RepoName))
				continue
			}
			rules, err := readAuditRules(filepath.Join(parent.Location, "profiles", "package.mask"), false, repo.RepoName)
			if err != nil {
				return in, err
			}
			masks = stackAuditRules(masks, rules, &in.Problems, false)
		}
		rules, err := readAuditRules(filepath.Join(repo.Location, "profiles", "package.mask"), false, repo.RepoName)
		if err != nil {
			return in, err
		}
		masks = stackAuditRules(masks, rules, &in.Problems, false)
		in.Masks = append(in.Masks, masks...)
		unmasks, err := readAuditRules(filepath.Join(repo.Location, "profiles", "package.unmask"), false, repo.RepoName)
		if err != nil {
			return in, err
		}
		if len(unmasks) > 0 && !strings.Contains(strings.Join(layouts[repo.RepoName].ProfileFormats(), " "), "portage-1") {
			in.Problems = append(in.Problems, fmt.Sprintf("repository %s has package.unmask without an explicit portage-1 profile format", repo.RepoName))
		}
		in.Unmasks = append(in.Unmasks, stackAuditRules(nil, unmasks, &in.Problems, false)...)
	}
	profile := opts.Profile
	if profile == "" {
		profile = filepath.Join(opts.ConfigRoot, "make.profile")
	}
	profiles, err := auditProfileCascade(profile, repoByName)
	if err != nil {
		in.Problems = append(in.Problems, err.Error())
	}
	// Portage stacks profile incrementals separately before combining them
	// with repository-wide rules. A profile -atom cannot erase a repo mask.
	var profileMasks, profileUnmasks []MaskRule
	for _, p := range profiles {
		// Profile package.unmask and directory fragments are Portage extensions.
		// Without an explicit format declaration, do not assume they are active.
		portageFormat := false
		for _, repo := range repos {
			rel, err := filepath.Rel(filepath.Join(repo.Location, "profiles"), p)
			if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
				for _, format := range layouts[repo.RepoName].ProfileFormats() {
					portageFormat = portageFormat || format == "portage-1"
				}
			}
		}
		for _, target := range []struct {
			name string
			dst  *[]MaskRule
		}{{"package.mask", &profileMasks}, {"package.unmask", &profileUnmasks}} {
			info, statErr := os.Stat(filepath.Join(p, target.name))
			if statErr == nil && !portageFormat && (info.IsDir() || target.name == "package.unmask") {
				in.Problems = append(in.Problems, fmt.Sprintf("profile extension %s/%s requires an explicit portage-1 format", p, target.name))
			}
			rules, err := readAuditRules(filepath.Join(p, target.name), false, "")
			if err != nil {
				return in, err
			}
			*target.dst = stackAuditRules(*target.dst, rules, &in.Problems, false)
		}
	}
	// The optional user profile participates after the selected profile.
	userProfile := filepath.Join(opts.ConfigRoot, "profile")
	if info, err := os.Stat(userProfile); err == nil && info.IsDir() {
		if parents, err := os.ReadFile(filepath.Join(userProfile, "parent")); err == nil && strings.TrimSpace(string(parents)) != "" {
			in.Problems = append(in.Problems, "user profile parent inheritance is unsupported")
		} else if err != nil && !os.IsNotExist(err) {
			return in, fmt.Errorf("read user profile parents: %w", err)
		}
		for _, name := range []string{"package.mask", "package.unmask"} {
			rules, err := readAuditRules(filepath.Join(userProfile, name), false, "")
			if err != nil {
				return in, err
			}
			if name == "package.mask" {
				profileMasks = stackAuditRules(profileMasks, rules, &in.Problems, false)
			} else {
				profileUnmasks = stackAuditRules(profileUnmasks, rules, &in.Problems, false)
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return in, fmt.Errorf("inspect user profile %s: %w", userProfile, err)
	}
	in.Masks = append(in.Masks, profileMasks...)
	in.Unmasks = append(in.Unmasks, profileUnmasks...)
	userMasks, err := readAuditRules(filepath.Join(opts.ConfigRoot, "package.mask"), false, "")
	if err != nil {
		return in, err
	}
	in.Masks = stackAuditRules(in.Masks, userMasks, &in.Problems, true)
	userUnmasks, err := readAuditRules(filepath.Join(opts.ConfigRoot, "package.unmask"), true, "")
	if err != nil {
		return in, err
	}
	in.Unmasks = stackAuditRules(in.Unmasks, userUnmasks, &in.Problems, true)
	// Only read ebuilds for packages referenced by a slot-constrained rule.
	// Slot values obtained by shell execution or inheritance remain unknown.
	slotPackages := map[string]bool{}
	for _, rules := range [][]MaskRule{in.Masks, in.Unmasks} {
		for _, rule := range rules {
			atom := ParsePackageAtom(rule.Atom)
			if atom.Slot != "" {
				slotPackages[atom.Category+"/"+atom.Name] = true
			}
		}
	}
	for i := range in.Candidates {
		c := &in.Candidates[i]
		if slotPackages[c.Category+"/"+c.Package] {
			path := filepath.Join(repoByName[c.Repository].Location, c.Category, c.Package, c.Package+"-"+c.Version+".ebuild")
			c.Slot = staticAuditSlot(path)
		}
	}
	sort.Slice(in.Candidates, func(i, j int) bool { return in.Candidates[i].String() < in.Candidates[j].String() })
	return in, nil
}

func auditProfileCascade(profile string, repos map[string]*RepoInfo) ([]string, error) {
	var ordered []string
	visiting := map[string]bool{}
	seen := map[string]bool{}
	var visit func(string) error
	visit = func(path string) error {
		path, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve active profile %s: %w", profile, err)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve profile path: %w", err)
		}
		if visiting[path] {
			return fmt.Errorf("profile parent cycle at %s", path)
		}
		if seen[path] {
			return fmt.Errorf("shared profile ancestry at %s is unsupported", path)
		}
		seen[path] = true
		visiting[path] = true
		defer delete(visiting, path)
		if eapi, err := os.ReadFile(filepath.Join(path, "eapi")); err == nil {
			switch strings.TrimSpace(string(eapi)) {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8":
			default:
				return fmt.Errorf("unsupported profile EAPI in %s", path)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("read profile EAPI in %s: %w", path, err)
		}
		parents, err := os.ReadFile(filepath.Join(path, "parent"))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read profile parents in %s: %w", path, err)
		}
		for _, line := range strings.Split(string(parents), "\n") {
			line, _, _ = strings.Cut(line, "#")
			parent := strings.TrimSpace(line)
			if parent == "" {
				continue
			}
			next := filepath.Join(path, parent)
			if name, suffix, qualified := strings.Cut(parent, ":"); qualified {
				repo := repos[name]
				if repo == nil || name == "" {
					return fmt.Errorf("unsupported or unavailable profile parent %q in %s", parent, path)
				}
				next = filepath.Join(repo.Location, "profiles", suffix)
			} else if filepath.IsAbs(parent) {
				next = parent
			}
			if err := visit(next); err != nil {
				return err
			}
		}
		ordered = append(ordered, path)
		return nil
	}
	err := visit(profile)
	return ordered, err
}

func staticAuditSlot(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(string(data)), path)
	if err != nil {
		return ""
	}
	var slot string
	var literal func([]syntax.WordPart) (string, bool)
	literal = func(parts []syntax.WordPart) (string, bool) {
		var value strings.Builder
		for _, part := range parts {
			switch p := part.(type) {
			case *syntax.Lit:
				value.WriteString(p.Value)
			case *syntax.SglQuoted:
				value.WriteString(p.Value)
			case *syntax.DblQuoted:
				s, ok := literal(p.Parts)
				if !ok {
					return "", false
				}
				value.WriteString(s)
			default:
				return "", false
			}
		}
		return value.String(), true
	}
	for _, stmt := range file.Stmts {
		if _, ok := stmt.Cmd.(*syntax.FuncDecl); ok {
			continue
		}
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) > 0 || stmt.Background || stmt.Negated || len(stmt.Redirs) > 0 {
			return "" // conditionals, inherit, source, eval, and shell commands
		}
		for _, assignment := range call.Assigns {
			if assignment.Value == nil {
				return ""
			}
			value, ok := literal(assignment.Value.Parts)
			if !ok {
				return "" // expansions can mutate SLOT as a side effect
			}
			if assignment.Name == nil || assignment.Name.Value != "SLOT" {
				continue
			}
			if assignment.Append || assignment.Index != nil {
				return ""
			}
			for _, component := range strings.Split(value, "/") {
				if !maskAtomComponent.MatchString(component) {
					return ""
				}
			}
			if strings.Count(value, "/") > 1 {
				return ""
			}
			slot = value
		}
	}
	return slot
}

func inventoryMaskCandidates(repo *RepoInfo) ([]MaskCandidate, error) {
	categories, err := os.ReadDir(repo.Location)
	if err != nil {
		return nil, fmt.Errorf("inventory repository %s at %s: %w", repo.RepoName, repo.Location, err)
	}
	var candidates []MaskCandidate
	for _, category := range categories {
		if strings.HasPrefix(category.Name(), ".") {
			continue
		}
		switch category.Name() {
		case "metadata", "profiles", "eclass", "licenses":
			continue
		}
		info, err := os.Stat(filepath.Join(repo.Location, category.Name()))
		if err != nil {
			return nil, fmt.Errorf("inspect inventory category %s: %w", category.Name(), err)
		}
		if !info.IsDir() {
			continue
		}
		packages, err := os.ReadDir(filepath.Join(repo.Location, category.Name()))
		if err != nil {
			return nil, fmt.Errorf("inventory category %s: %w", category.Name(), err)
		}
		for _, pkg := range packages {
			if strings.HasPrefix(pkg.Name(), ".") {
				continue
			}
			info, err := os.Stat(filepath.Join(repo.Location, category.Name(), pkg.Name()))
			if err != nil {
				return nil, fmt.Errorf("inspect inventory package %s/%s: %w", category.Name(), pkg.Name(), err)
			}
			if !info.IsDir() {
				continue
			}
			entries, err := os.ReadDir(filepath.Join(repo.Location, category.Name(), pkg.Name()))
			if err != nil {
				return nil, fmt.Errorf("inventory package %s/%s: %w", category.Name(), pkg.Name(), err)
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".ebuild") || !strings.HasPrefix(e.Name(), pkg.Name()+"-") {
					continue
				}
				path := filepath.Join(repo.Location, category.Name(), pkg.Name(), e.Name())
				info, err := os.Stat(path)
				if err != nil {
					return nil, fmt.Errorf("inspect available ebuild %s: %w", path, err)
				}
				if !info.Mode().IsRegular() {
					return nil, fmt.Errorf("available ebuild %s is not a regular file", path)
				}
				version := strings.TrimSuffix(strings.TrimPrefix(e.Name(), pkg.Name()+"-"), ".ebuild")
				if !ParseGentooVersion(version).IsValid {
					return nil, fmt.Errorf("inventory invalid ebuild version in %s/%s/%s", category.Name(), pkg.Name(), e.Name())
				}
				candidates = append(candidates, MaskCandidate{Category: category.Name(), Package: pkg.Name(), Version: version, Repository: repo.RepoName})
			}
		}
	}
	return candidates, nil
}
