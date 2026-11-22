package g2

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// NewsDialect identifies the inferred presentation heading convention of a news document.
type NewsDialect string

const (
	DialectPlain           NewsDialect = "plain"
	DialectMarkdownHash    NewsDialect = "markdown_hash"
	DialectSetextUnderline NewsDialect = "setext_underline"
	DialectMediaWikiEquals NewsDialect = "mediawiki_equals"
)

// NewsDialectReport records explainable heuristic metadata for research/testing only.
// It is never published as public warnings on generated pages.
type NewsDialectReport struct {
	Dialect         NewsDialect
	Confidence      float64
	Evidence        []string
	OverriddenLines []int
}

// NewsRenderNodeType identifies a node in the derived immutable render AST.
type NewsRenderNodeType int

const (
	RenderNodeParagraph NewsRenderNodeType = iota
	RenderNodeHeading
	RenderNodeList
	RenderNodeCode
	RenderNodeFootnotes
)

// NewsFootnoteDef holds a single deduplicated footnote definition.
type NewsFootnoteDef struct {
	Number    string
	Body      string
	URL       string
	RefCount  int
	SourceIdx int
}

// NewsRenderNode represents a node in the transformed semantic/render AST.
type NewsRenderNode struct {
	Type            NewsRenderNodeType
	SourceStartLine int
	SourceEndLine   int
	HeadingLevel    int    // 1..4 relative level before mapping to <h3>..<h6>
	HeadingID       string // base slug before optional render scope prefix
	Lines           []string
	FootnoteDefs    []NewsFootnoteDef
}

// NewsRenderDoc holds the transformed semantic/render AST and diagnostic metadata.
type NewsRenderDoc struct {
	Nodes     []NewsRenderNode
	Footnotes []NewsFootnoteDef
	Report    NewsDialectReport
	IsFormat1 bool
}

var (
	reHashHeadingLine     = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	reMediaWikiHeading    = regexp.MustCompile(`^(={2,6})\s*(.+?)\s*(={2,6})\s*$`)
	reSetextUnderlineLine = regexp.MustCompile(`^([=-]){3,}\s*$`)
	reShellCommentLine    = regexp.MustCompile(`^(?i)(emerge|eselect|cd|cp|mv|rm|ln|sln|mkdir|chmod|chown|grep|sed|awk|cat|echo|nano|vim|git|systemctl|rc-update|rc-service|perl-cleaner|python-updater|etc-update|dispatch-conf|flatpak|usermod|gpasswd|mount|umount|equery|euse|FEATURES=|USE=|[A-Z_]+=|--?[a-zA-Z]|/etc/|/usr/|/var/|/bin/|/sbin/|https?://)`)
	reFootnoteDefLine     = regexp.MustCompile(`^\[(\d+)\]\s+(\S.*)$`)
	reInlineFootnoteRef   = regexp.MustCompile(`\[(\d+)\]`)
	reSlugNonAlnum        = regexp.MustCompile(`[^a-z0-9]+`)
)

// TransformNewsAST applies pure, composable passes over an immutable NewsItem
// to produce a derived semantic render tree without mutating the source AST or body.
func TransformNewsAST(item NewsItem) NewsRenderDoc {
	isFmt1 := item.NewsItemFormat != "2.0"
	srcNodes := item.BodyAST
	if isFmt1 {
		if item.Body != "" {
			srcNodes = []NewsNode{{
				Type:      NewsNodeText,
				Lines:     strings.Split(item.Body, "\n"),
				StartLine: 1,
				EndLine:   strings.Count(item.Body, "\n") + 1,
			}}
		}
	}

	report, headings := inferAndClassifyHeadings(srcNodes)
	nodes := applyHeadingTransform(srcNodes, headings)
	nodes, footnotes := extractAndLinkFootnotes(nodes)

	return NewsRenderDoc{
		Nodes:     nodes,
		Footnotes: footnotes,
		Report:    report,
		IsFormat1: isFmt1,
	}
}

type detectedHeading struct {
	nodeIdx    int
	startLine  int // index inside node.Lines
	endLine    int // inclusive index inside node.Lines (equals startLine except Setext)
	level      int
	text       string
	sourceLine int
}

func inferAndClassifyHeadings(srcNodes []NewsNode) (NewsDialectReport, []detectedHeading) {
	var mdHits, setextHits, mwHits []detectedHeading
	var rejectedLines []int

	for nIdx, node := range srcNodes {
		if node.Type != NewsNodeText {
			continue
		}
		lines := node.Lines
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Never promote indented lines (code/verbatim/continuation) as headings.
			if line != strings.TrimLeft(line, " \t") {
				if strings.HasPrefix(trimmed, "#") || reMediaWikiHeading.MatchString(trimmed) {
					if node.StartLine > 0 {
						rejectedLines = append(rejectedLines, node.StartLine+i)
					}
				}
				continue
			}

			// 1. Check Setext underline (requires i > 0 and non-blank previous line without sentence period).
			if i > 0 && reSetextUnderlineLine.MatchString(trimmed) {
				prev := lines[i-1]
				prevTrimmed := strings.TrimSpace(prev)
				prevUnindented := prev == strings.TrimLeft(prev, " \t")
				prevBeforeBlank := i == 1 || strings.TrimSpace(lines[i-2]) == ""
				if prevUnindented && prevTrimmed != "" && prevBeforeBlank && !strings.HasSuffix(prevTrimmed, ".") && !strings.HasSuffix(prevTrimmed, "!") && len(prevTrimmed) <= 90 {
					lvl := 1
					if strings.HasPrefix(trimmed, "-") {
						lvl = 2
					}
					srcL := 0
					if node.StartLine > 0 {
						srcL = node.StartLine + i - 1
					}
					setextHits = append(setextHits, detectedHeading{
						nodeIdx:    nIdx,
						startLine:  i - 1,
						endLine:    i,
						level:      lvl,
						text:       prevTrimmed,
						sourceLine: srcL,
					})
					continue
				}
			}

			// 2. Check MediaWiki == Heading == (allowing asymmetrical ===).
			if m := reMediaWikiHeading.FindStringSubmatch(trimmed); m != nil {
				inner := strings.TrimSpace(m[2])
				if inner != "" && len(inner) <= 90 {
					srcL := 0
					if node.StartLine > 0 {
						srcL = node.StartLine + i
					}
					mwHits = append(mwHits, detectedHeading{
						nodeIdx:    nIdx,
						startLine:  i,
						endLine:    i,
						level:      len(m[1]) - 1,
						text:       inner,
						sourceLine: srcL,
					})
					continue
				}
			}

			// 3. Check Markdown # / ## headings vs shell/config comments.
			if m := reHashHeadingLine.FindStringSubmatch(line); m != nil {
				lvl := len(m[1])
				text := strings.TrimSpace(m[2])
				prevBlank := i == 0 || strings.TrimSpace(lines[i-1]) == ""
				nextBlank := i == len(lines)-1 || strings.TrimSpace(lines[i+1]) == ""
				prevHash := i > 0 && strings.HasPrefix(strings.TrimSpace(lines[i-1]), "#")
				nextHash := i < len(lines)-1 && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "#")
				nextIsListNode := i == len(lines)-1 && nIdx+1 < len(srcNodes) && srcNodes[nIdx+1].Type == NewsNodeList
				isShell := reShellCommentLine.MatchString(text)
				endsSentence := strings.HasSuffix(text, ".") || strings.HasSuffix(text, "!")

				accepted := !isShell && !endsSentence && !prevHash && !nextHash && prevBlank && len(text) <= 80 && (lvl >= 2 || nextBlank || nextIsListNode)
				if accepted {
					srcL := 0
					if node.StartLine > 0 {
						srcL = node.StartLine + i
					}
					mdHits = append(mdHits, detectedHeading{
						nodeIdx:    nIdx,
						startLine:  i,
						endLine:    i,
						level:      lvl,
						text:       text,
						sourceLine: srcL,
					})
				} else if node.StartLine > 0 {
					rejectedLines = append(rejectedLines, node.StartLine+i)
				}
			}
		}
	}

	// Select winning document dialect from aggregate structural evidence.
	report := NewsDialectReport{
		Dialect:         DialectPlain,
		Confidence:      1.0,
		OverriddenLines: rejectedLines,
	}
	var chosen []detectedHeading

	switch {
	case len(mdHits) >= len(setextHits) && len(mdHits) >= len(mwHits) && len(mdHits) > 0:
		report.Dialect = DialectMarkdownHash
		report.Confidence = 0.95
		report.Evidence = append(report.Evidence, fmt.Sprintf("%d markdown hash heading(s)", len(mdHits)))
		chosen = mdHits
	case len(setextHits) >= len(mwHits) && len(setextHits) > 0:
		report.Dialect = DialectSetextUnderline
		report.Confidence = 0.95
		report.Evidence = append(report.Evidence, fmt.Sprintf("%d setext underline heading(s)", len(setextHits)))
		chosen = setextHits
	case len(mwHits) > 0:
		report.Dialect = DialectMediaWikiEquals
		report.Confidence = 0.90
		report.Evidence = append(report.Evidence, fmt.Sprintf("%d mediawiki heading(s)", len(mwHits)))
		chosen = mwHits
	default:
		if len(rejectedLines) > 0 {
			report.Evidence = append(report.Evidence, fmt.Sprintf("rejected %d shell/comment candidate(s)", len(rejectedLines)))
		}
	}

	// Normalize minimum heading level to 1 (renders as <h3> under page <h2>).
	if len(chosen) > 0 {
		minLvl := chosen[0].level
		for _, h := range chosen {
			if h.level < minLvl {
				minLvl = h.level
			}
		}
		for i := range chosen {
			chosen[i].level = chosen[i].level - minLvl + 1
			if chosen[i].level < 1 {
				chosen[i].level = 1
			}
			if chosen[i].level > 4 {
				chosen[i].level = 4
			}
		}
	}

	return report, chosen
}

func applyHeadingTransform(srcNodes []NewsNode, headings []detectedHeading) []NewsRenderNode {
	byNode := make(map[int][]detectedHeading)
	for _, h := range headings {
		byNode[h.nodeIdx] = append(byNode[h.nodeIdx], h)
	}

	slugCounts := make(map[string]int)
	var out []NewsRenderNode

	for nIdx, node := range srcNodes {
		switch node.Type {
		case NewsNodeList:
			linesCopy := append([]string(nil), node.Lines...)
			out = append(out, NewsRenderNode{
				Type:            RenderNodeList,
				SourceStartLine: node.StartLine,
				SourceEndLine:   node.EndLine,
				Lines:           linesCopy,
			})
		case NewsNodeCode:
			linesCopy := append([]string(nil), node.Lines...)
			if node.StartLine > 0 {
				for len(linesCopy) > 1 && strings.TrimSpace(linesCopy[len(linesCopy)-1]) == "" {
					linesCopy = linesCopy[:len(linesCopy)-1]
				}
			}
			out = append(out, NewsRenderNode{
				Type:            RenderNodeCode,
				SourceStartLine: node.StartLine,
				SourceEndLine:   node.EndLine,
				Lines:           linesCopy,
			})
		case NewsNodeText:
			hs := byNode[nIdx]
			if len(hs) == 0 {
				linesCopy := append([]string(nil), node.Lines...)
				out = append(out, NewsRenderNode{
					Type:            RenderNodeParagraph,
					SourceStartLine: node.StartLine,
					SourceEndLine:   node.EndLine,
					Lines:           linesCopy,
				})
				continue
			}

			cursor := 0
			for _, h := range hs {
				if h.startLine > cursor {
					seg := trimTrailingBlankEdges(node.Lines[cursor:h.startLine], cursor == 0)
					if len(seg) > 0 {
						out = append(out, NewsRenderNode{
							Type:            RenderNodeParagraph,
							SourceStartLine: offsetSourceLine(node.StartLine, cursor),
							SourceEndLine:   offsetSourceLine(node.StartLine, h.startLine-1),
							Lines:           append([]string(nil), seg...),
						})
					}
				}
				slug := slugifyHeading(h.text)
				slugCounts[slug]++
				if slugCounts[slug] > 1 {
					slug = fmt.Sprintf("%s-%d", slug, slugCounts[slug])
				}
				out = append(out, NewsRenderNode{
					Type:            RenderNodeHeading,
					SourceStartLine: offsetSourceLine(node.StartLine, h.startLine),
					SourceEndLine:   offsetSourceLine(node.StartLine, h.endLine),
					HeadingLevel:    h.level,
					HeadingID:       slug,
					Lines:           []string{h.text},
				})
				cursor = h.endLine + 1
			}
			if cursor < len(node.Lines) {
				seg := trimLeadingBlankEdges(node.Lines[cursor:])
				if len(seg) > 0 {
					out = append(out, NewsRenderNode{
						Type:            RenderNodeParagraph,
						SourceStartLine: offsetSourceLine(node.StartLine, cursor),
						SourceEndLine:   node.EndLine,
						Lines:           append([]string(nil), seg...),
					})
				}
			}
		}
	}
	return out
}

func offsetSourceLine(base, rel int) int {
	if base <= 0 {
		return 0
	}
	return base + rel
}

func trimTrailingBlankEdges(lines []string, keepLeading bool) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	start := 0
	if !keepLeading {
		for start < end && strings.TrimSpace(lines[start]) == "" {
			start++
		}
	}
	return lines[start:end]
}

func trimLeadingBlankEdges(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	return lines[start:]
}

func slugifyHeading(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	slug := reSlugNonAlnum.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "section"
	}
	return slug
}

func extractAndLinkFootnotes(nodes []NewsRenderNode) ([]NewsRenderNode, []NewsFootnoteDef) {
	seen := make(map[string]int)
	var defs []NewsFootnoteDef
	var filtered []NewsRenderNode

	for _, node := range nodes {
		if node.Type != RenderNodeParagraph {
			filtered = append(filtered, node)
			continue
		}
		var kept []string
		strippedFromNode := false
		for i := 0; i < len(node.Lines); i++ {
			trimmed := strings.TrimSpace(node.Lines[i])
			if m := reFootnoteDefLine.FindStringSubmatch(trimmed); m != nil && node.Lines[i] == strings.TrimLeft(node.Lines[i], " \t") {
				num := m[1]
				body := strings.TrimSpace(m[2])
				if _, exists := seen[num]; !exists {
					urlStr := extractSafeFootnoteURL(body)
					seen[num] = len(defs)
					defs = append(defs, NewsFootnoteDef{
						Number:    num,
						Body:      body,
						URL:       urlStr,
						SourceIdx: len(defs),
					})
				}
				strippedFromNode = true
				continue
			}
			kept = append(kept, node.Lines[i])
		}
		if strippedFromNode {
			for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
				kept = kept[:len(kept)-1]
			}
		}
		if len(kept) > 0 {
			node.Lines = kept
			filtered = append(filtered, node)
		}
	}

	if len(defs) > 0 {
		// Drop trailing heading like "# Refs" only if empty? No, preserve explicit headings and append footnote block.
		filtered = append(filtered, NewsRenderNode{
			Type:         RenderNodeFootnotes,
			FootnoteDefs: defs,
		})
	}
	return filtered, defs
}

func extractSafeFootnoteURL(body string) string {
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return ""
	}
	cand := strings.Trim(fields[0], "<>")
	lower := strings.ToLower(cand)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return cand
	}
	return ""
}

// RenderHTML converts the derived render AST into escaped, accessible HTML using scope for unique IDs.
func (d NewsRenderDoc) RenderHTML(scope string) template.HTML {
	if d.IsFormat1 && d.Report.Dialect == DialectPlain && len(d.Footnotes) == 0 {
		if len(d.Nodes) == 0 {
			return ""
		}
		escaped := template.HTMLEscapeString(strings.Join(d.Nodes[0].Lines, "\n"))
		return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
	}

	scopeSlug := sanitizeAnchorScope(scope)
	footnotes := make([]NewsFootnoteDef, len(d.Footnotes))
	copy(footnotes, d.Footnotes)
	fnByNum := make(map[string]*NewsFootnoteDef, len(footnotes))
	for i := range footnotes {
		footnotes[i].RefCount = 0
		fnByNum[footnotes[i].Number] = &footnotes[i]
	}

	var out strings.Builder
	first := true
	writeSep := func() {
		if !first {
			out.WriteByte('\n')
		}
		first = false
	}

	for _, node := range d.Nodes {
		switch node.Type {
		case RenderNodeHeading:
			writeSep()
			tagLevel := node.HeadingLevel + 2
			if tagLevel < 3 {
				tagLevel = 3
			}
			if tagLevel > 6 {
				tagLevel = 6
			}
			idAttr := node.HeadingID
			if scopeSlug != "" {
				idAttr = scopeSlug + "-" + idAttr
			}
			fmt.Fprintf(&out, `<h%d id="%s">%s</h%d>`, tagLevel, template.HTMLEscapeString(idAttr), template.HTMLEscapeString(strings.Join(node.Lines, " ")), tagLevel)

		case RenderNodeParagraph:
			for _, line := range node.Lines {
				writeSep()
				if strings.TrimSpace(line) == "" {
					out.WriteString("<br><br>")
				} else {
					out.WriteString(renderProseWithFootnotes(line, scopeSlug, fnByNum))
				}
			}

		case RenderNodeList:
			writeSep()
			out.WriteString("<ul>")
			for _, item := range node.Lines {
				out.WriteString("\n<li>")
				out.WriteString(renderProseWithFootnotes(item, scopeSlug, fnByNum))
				out.WriteString("</li>")
			}
			out.WriteString("\n</ul>")

		case RenderNodeCode:
			writeSep()
			out.WriteString("<pre><code>")
			for idx, codeLine := range node.Lines {
				if idx > 0 {
					out.WriteByte('\n')
				}
				out.WriteString(template.HTMLEscapeString(codeLine))
			}
			out.WriteString("</code></pre>")

		case RenderNodeFootnotes:
			if len(footnotes) == 0 {
				continue
			}
			writeSep()
			out.WriteString(`<section class="news-footnotes" aria-label="Footnotes"><ol>`)
			for _, fn := range footnotes {
				fnID := footnoteAnchorID(scopeSlug, fn.Number)
				fmt.Fprintf(&out, "\n<li id=\"%s\" value=\"%s\">", template.HTMLEscapeString(fnID), template.HTMLEscapeString(fn.Number))
				if fn.URL != "" && strings.TrimSpace(fn.Body) == fn.URL {
					escURL := template.HTMLEscapeString(fn.URL)
					fmt.Fprintf(&out, `<a href="%s" rel="noopener noreferrer">%s</a>`, escURL, escURL)
				} else if fn.URL != "" && strings.HasPrefix(fn.Body, fn.URL) {
					escURL := template.HTMLEscapeString(fn.URL)
					rest := strings.TrimPrefix(fn.Body, fn.URL)
					fmt.Fprintf(&out, `<a href="%s" rel="noopener noreferrer">%s</a>%s`, escURL, escURL, template.HTMLEscapeString(rest))
				} else {
					out.WriteString(template.HTMLEscapeString(fn.Body))
				}
				if fn.RefCount > 0 {
					refID := footnoteRefAnchorID(scopeSlug, fn.Number, 1)
					fmt.Fprintf(&out, ` <a href="#%s" class="footnote-backref" aria-label="Back to reference %s">↩</a>`, template.HTMLEscapeString(refID), template.HTMLEscapeString(fn.Number))
				}
				out.WriteString("</li>")
			}
			out.WriteString("\n</ol></section>")
		}
	}

	return template.HTML(out.String())
}

// ToHTMLWithScope transforms and renders the NewsItem into safe HTML with scoped anchor IDs.
func (n NewsItem) ToHTMLWithScope(scope string) template.HTML {
	return TransformNewsAST(n).RenderHTML(scope)
}

func renderProseWithFootnotes(line, scopeSlug string, fnByNum map[string]*NewsFootnoteDef) string {
	if len(fnByNum) == 0 || !strings.Contains(line, "[") {
		return template.HTMLEscapeString(line)
	}

	// Never interpret [N] inside inline backtick code spans.
	var b strings.Builder
	pos := 0
	for pos < len(line) {
		tickStart := strings.IndexByte(line[pos:], '`')
		if tickStart == -1 {
			b.WriteString(replaceInlineFootnotes(line[pos:], scopeSlug, fnByNum))
			break
		}
		if tickStart > 0 {
			b.WriteString(replaceInlineFootnotes(line[pos:pos+tickStart], scopeSlug, fnByNum))
		}
		codeStart := pos + tickStart + 1
		tickEnd := strings.IndexByte(line[codeStart:], '`')
		if tickEnd == -1 {
			b.WriteString(replaceInlineFootnotes(line[pos+tickStart:], scopeSlug, fnByNum))
			break
		}
		b.WriteString(template.HTMLEscapeString(line[pos+tickStart : codeStart+tickEnd+1]))
		pos = codeStart + tickEnd + 1
	}
	return b.String()
}

func replaceInlineFootnotes(seg, scopeSlug string, fnByNum map[string]*NewsFootnoteDef) string {
	matches := reInlineFootnoteRef.FindAllStringSubmatchIndex(seg, -1)
	if len(matches) == 0 {
		return template.HTMLEscapeString(seg)
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(template.HTMLEscapeString(seg[last:m[0]]))
		num := seg[m[2]:m[3]]
		if fn, ok := fnByNum[num]; ok {
			fn.RefCount++
			refID := footnoteRefAnchorID(scopeSlug, num, fn.RefCount)
			fnID := footnoteAnchorID(scopeSlug, num)
			fmt.Fprintf(&b, `<sup><a id="%s" href="#%s" aria-label="Footnote %s">[%s]</a></sup>`,
				template.HTMLEscapeString(refID),
				template.HTMLEscapeString(fnID),
				template.HTMLEscapeString(num),
				template.HTMLEscapeString(num),
			)
		} else {
			b.WriteString(template.HTMLEscapeString(seg[m[0]:m[1]]))
		}
		last = m[1]
	}
	b.WriteString(template.HTMLEscapeString(seg[last:]))
	return b.String()
}

func sanitizeAnchorScope(scope string) string {
	if scope == "" {
		return ""
	}
	s := reSlugNonAlnum.ReplaceAllString(strings.ToLower(scope), "-")
	return strings.Trim(s, "-")
}

func footnoteAnchorID(scopeSlug, num string) string {
	if scopeSlug == "" {
		return "fn-" + num
	}
	return "fn-" + scopeSlug + "-" + num
}

func footnoteRefAnchorID(scopeSlug, num string, occ int) string {
	if scopeSlug == "" {
		return fmt.Sprintf("fnref-%s-%d", num, occ)
	}
	return fmt.Sprintf("fnref-%s-%s-%d", scopeSlug, num, occ)
}
