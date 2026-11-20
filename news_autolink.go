package g2

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// NewsLinkContext provides site-relative path prefixes and known repository package/version
// inventory so news autolinking never emits broken or nondeterministic links.
type NewsLinkContext struct {
	BaseURL      string
	CurrentRepo  string
	PackageIndex map[string][]string          // "cat/pkg" -> sorted repo names
	VersionIndex map[string]map[string]bool // "repo|cat/pkg" -> set of PV strings
}

var (
	reNewsURLCandidate  = regexp.MustCompile(`https?://[^\s<>"']+`)
	reNewsAtomCandidate = regexp.MustCompile(`(?:>=|<=|>|<|=|~|!)?([a-z0-9]+-[a-z0-9]+|virtual|acct-user|acct-group)/([A-Za-z0-9+_.-]+)(?::([A-Za-z0-9+_./-]+))?(?:::([A-Za-z0-9+_.-]+))?`)
	reVersionSplit      = regexp.MustCompile(`^(.+?)-(\d+(?:\.\d+)*[a-z]?(?:_(?:alpha|beta|pre|rc|p)\d*)*(?:-r\d+)?)$`)
)

// ToHTMLWithLinks renders the NewsItem body into safe HTML with autolinked HTTP(S) URLs
// and verified package/version references from ctx.
func (n NewsItem) ToHTMLWithLinks(ctx NewsLinkContext) template.HTML {
	if n.NewsItemFormat == "2.0" {
		var out strings.Builder
		first := true
		for _, node := range n.BodyAST {
			switch node.Type {
			case NewsNodeText:
				for _, line := range node.Lines {
					if !first {
						out.WriteByte('\n')
					}
					first = false
					if strings.TrimSpace(line) == "" {
						out.WriteString("<br><br>")
					} else {
						out.WriteString(renderNewsLineWithAutolinks(line, ctx))
					}
				}
			case NewsNodeList:
				if !first {
					out.WriteByte('\n')
				}
				first = false
				out.WriteString("<ul>")
				for _, item := range node.Lines {
					out.WriteString("\n<li>")
					out.WriteString(renderNewsLineWithAutolinks(item, ctx))
					out.WriteString("</li>")
				}
				out.WriteString("\n</ul>")
			case NewsNodeCode:
				if !first {
					out.WriteByte('\n')
				}
				first = false
				out.WriteString("<pre><code>")
				for i, codeLine := range node.Lines {
					if i > 0 {
						out.WriteByte('\n')
					}
					out.WriteString(template.HTMLEscapeString(codeLine))
				}
				out.WriteString("</code></pre>")
			}
		}
		return template.HTML(out.String())
	}

	if n.Body == "" {
		return ""
	}
	lines := strings.Split(n.Body, "\n")
	var out strings.Builder
	for i, line := range lines {
		if i > 0 {
			out.WriteString("<br>")
		}
		out.WriteString(renderNewsLineWithAutolinks(line, ctx))
	}
	return template.HTML(out.String())
}

func isUnindentedShellLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "emerge ") ||
		strings.HasPrefix(trimmed, "# ") ||
		strings.HasPrefix(trimmed, "$ ")
}

func renderNewsLineWithAutolinks(line string, ctx NewsLinkContext) string {
	if isUnindentedShellLine(line) {
		return template.HTMLEscapeString(line)
	}
	// Split around inline backtick code spans; never autolink inside `...`.
	var b strings.Builder
	pos := 0
	for pos < len(line) {
		tickStart := strings.IndexByte(line[pos:], '`')
		if tickStart == -1 {
			b.WriteString(autolinkProseSegment(line[pos:], ctx))
			break
		}
		if tickStart > 0 {
			b.WriteString(autolinkProseSegment(line[pos:pos+tickStart], ctx))
		}
		codeStart := pos + tickStart + 1
		tickEnd := strings.IndexByte(line[codeStart:], '`')
		if tickEnd == -1 {
			b.WriteString(autolinkProseSegment(line[pos+tickStart:], ctx))
			break
		}
		b.WriteString("<code>")
		b.WriteString(template.HTMLEscapeString(line[codeStart : codeStart+tickEnd]))
		b.WriteString("</code>")
		pos = codeStart + tickEnd + 1
	}
	return b.String()
}

func autolinkProseSegment(seg string, ctx NewsLinkContext) string {
	if seg == "" {
		return ""
	}
	var b strings.Builder
	i := 0
	for i < len(seg) {
		// Check for URL match starting at or after i, and atom match starting at or after i.
		urlLoc := reNewsURLCandidate.FindStringIndex(seg[i:])
		var atomLoc []int
		if len(ctx.PackageIndex) > 0 {
			atomLoc = findNextValidAtomMatch(seg[i:], ctx)
		}

		// Pick earliest match.
		chooseURL := false
		chooseAtom := false
		if urlLoc != nil && (atomLoc == nil || urlLoc[0] <= atomLoc[0]) {
			chooseURL = true
		} else if atomLoc != nil {
			chooseAtom = true
		}

		if !chooseURL && !chooseAtom {
			b.WriteString(template.HTMLEscapeString(seg[i:]))
			break
		}

		if chooseURL {
			start := i + urlLoc[0]
			end := i + urlLoc[1]
			// Avoid matching URL inside an email or after non-boundary characters.
			if start > 0 && isIdentByte(seg[start-1]) {
				b.WriteString(template.HTMLEscapeString(seg[i:end]))
				i = end
				continue
			}
			b.WriteString(template.HTMLEscapeString(seg[i:start]))
			rawURL, trailing := splitURLTrailingPunctuation(seg[start:end])
			escURL := template.HTMLEscapeString(rawURL)
			fmt.Fprintf(&b, `<a href="%s" rel="noopener noreferrer">%s</a>`, escURL, escURL)
			b.WriteString(template.HTMLEscapeString(trailing))
			i = end
			continue
		}

		// chooseAtom
		start := i + atomLoc[0]
		end := i + atomLoc[1]
		b.WriteString(template.HTMLEscapeString(seg[i:start]))
		atomText := seg[start:end]
		if target, ok := resolveAtomTarget(atomText, ctx); ok {
			fmt.Fprintf(&b, `<a href="%s">%s</a>`, template.HTMLEscapeString(target), template.HTMLEscapeString(atomText))
		} else {
			b.WriteString(template.HTMLEscapeString(atomText))
		}
		i = end
	}
	return b.String()
}

func findNextValidAtomMatch(s string, ctx NewsLinkContext) []int {
	offset := 0
	for offset < len(s) {
		loc := reNewsAtomCandidate.FindStringSubmatchIndex(s[offset:])
		if loc == nil {
			return nil
		}
		absStart := offset + loc[0]
		absEnd := offset + loc[1]

		// Boundary check before match (avoid matching mid-word or email/URL path).
		if absStart > 0 {
			prev := s[absStart-1]
			if isIdentByte(prev) || prev == '/' || prev == '@' || prev == ':' {
				offset = absStart + 1
				continue
			}
		}
		// Strip trailing sentence punctuation if captured by pkg/slot/repo pattern.
		atomSlice := s[absStart:absEnd]
		trimmedAtom := strings.TrimRight(atomSlice, ".,;:!?)")
		if trimmedAtom != atomSlice {
			absEnd = absStart + len(trimmedAtom)
			atomSlice = trimmedAtom
		}
		if _, ok := resolveAtomTarget(atomSlice, ctx); ok {
			return []int{absStart, absEnd}
		}
		offset = absStart + 1
	}
	return nil
}

func isIdentByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.'
}

func splitURLTrailingPunctuation(raw string) (urlPart, trailing string) {
	end := len(raw)
	for end > 0 {
		ch := raw[end-1]
		switch ch {
		case '.', ',', ';', ':', '!', '?':
			end--
		case ')':
			open := strings.Count(raw[:end], "(")
			close := strings.Count(raw[:end], ")")
			if close > open {
				end--
			} else {
				return raw[:end], raw[end:]
			}
		case ']':
			open := strings.Count(raw[:end], "[")
			close := strings.Count(raw[:end], "]")
			if close > open {
				end--
			} else {
				return raw[:end], raw[end:]
			}
		default:
			return raw[:end], raw[end:]
		}
	}
	return raw[:end], raw[end:]
}

func resolveAtomTarget(atom string, ctx NewsLinkContext) (string, bool) {
	if len(ctx.PackageIndex) == 0 {
		return "", false
	}
	core := strings.TrimLeft(atom, "<>=~!")
	var repoQual string
	if idx := strings.Index(core, "::"); idx != -1 {
		repoQual = core[idx+2:]
		core = core[:idx]
	}
	if idx := strings.IndexByte(core, ':'); idx != -1 {
		core = core[:idx]
	}
	slashIdx := strings.IndexByte(core, '/')
	if slashIdx <= 0 || slashIdx >= len(core)-1 {
		return "", false
	}
	cat := core[:slashIdx]
	pkgPart := core[slashIdx+1:]

	pkgName := ""
	ver := ""
	fullKey := cat + "/" + pkgPart
	if _, ok := ctx.PackageIndex[fullKey]; ok {
		pkgName = pkgPart
	} else if m := reVersionSplit.FindStringSubmatch(pkgPart); m != nil {
		candKey := cat + "/" + m[1]
		if _, ok := ctx.PackageIndex[candKey]; ok {
			pkgName = m[1]
			ver = m[2]
		}
	}
	if pkgName == "" {
		return "", false
	}

	catPkg := cat + "/" + pkgName
	repos := ctx.PackageIndex[catPkg]
	if len(repos) == 0 {
		return "", false
	}

	if repoQual != "" {
		if !containsStr(repos, repoQual) {
			return "", false
		}
		if ver != "" && ctx.VersionIndex[repoQual+"|"+catPkg][ver] {
			return ctx.BaseURL + "repos/" + repoQual + "/categories/" + cat + "/packages/" + pkgName + "/ebuild/" + ver + "/", true
		}
		return ctx.BaseURL + "repos/" + repoQual + "/categories/" + cat + "/packages/" + pkgName + "/", true
	}

	if ver != "" {
		if ctx.CurrentRepo != "" && ctx.VersionIndex[ctx.CurrentRepo+"|"+catPkg][ver] {
			return ctx.BaseURL + "repos/" + ctx.CurrentRepo + "/categories/" + cat + "/packages/" + pkgName + "/ebuild/" + ver + "/", true
		}
		var matchingRepos []string
		for _, r := range repos {
			if ctx.VersionIndex[r+"|"+catPkg][ver] {
				matchingRepos = append(matchingRepos, r)
			}
		}
		if len(matchingRepos) == 1 {
			r := matchingRepos[0]
			return ctx.BaseURL + "repos/" + r + "/categories/" + cat + "/packages/" + pkgName + "/ebuild/" + ver + "/", true
		}
		return ctx.BaseURL + "packages/" + cat + "/" + pkgName + "/", true
	}

	if ctx.CurrentRepo != "" && containsStr(repos, ctx.CurrentRepo) {
		return ctx.BaseURL + "repos/" + ctx.CurrentRepo + "/categories/" + cat + "/packages/" + pkgName + "/", true
	}
	return ctx.BaseURL + "packages/" + cat + "/" + pkgName + "/", true
}

func containsStr(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
