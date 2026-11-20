package g2

import (
	"reflect"
	"strings"
	"testing"
)

func TestTransformNewsAST_ImmutabilityAndDialects(t *testing.T) {
	raw := `Title: Varnish Renamed
Author: Jane Doe <jane@example.com>
Posted: 2026-03-31
Revision: 1
News-Item-Format: 2.0

# Varnish Renamed to Vinyl-Cache

Package renamed[0][1][2].

# Changes

 varnishd -> vinyld

# Refs

[0] https://example.com/0
[1] https://example.com/1
[2] https://example.com/2
`
	item := ParseNewsItem(raw)
	origBody := item.Body
	origAST := make([]NewsNode, len(item.BodyAST))
	for i, n := range item.BodyAST {
		origAST[i] = NewsNode{
			Type:      n.Type,
			StartLine: n.StartLine,
			EndLine:   n.EndLine,
			Lines:     append([]string(nil), n.Lines...),
			RawLines:  append([]string(nil), n.RawLines...),
		}
	}

	doc := TransformNewsAST(item)
	if doc.Report.Dialect != DialectMarkdownHash {
		t.Fatalf("expected DialectMarkdownHash, got %s (%+v)", doc.Report.Dialect, doc.Report)
	}
	if !reflect.DeepEqual(item.Body, origBody) || !reflect.DeepEqual(item.BodyAST, origAST) {
		t.Fatalf("TransformNewsAST mutated source NewsItem or BodyAST")
	}

	htmlOut := string(doc.RenderHTML("varnish-scope"))
	for _, want := range []string{
		`<h3 id="varnish-scope-varnish-renamed-to-vinyl-cache">Varnish Renamed to Vinyl-Cache</h3>`,
		`<h3 id="varnish-scope-changes">Changes</h3>`,
		`<pre><code>varnishd -&gt; vinyld</code></pre>`,
		`href="#fn-varnish-scope-0"`,
		`id="fnref-varnish-scope-0-1"`,
		`<li id="fn-varnish-scope-0" value="0">`,
		`<section class="news-footnotes" aria-label="Footnotes">`,
	} {
		if !strings.Contains(htmlOut, want) {
			t.Fatalf("rendered HTML missing %q\ngot:\n%s", want, htmlOut)
		}
	}
}

func TestTransformNewsAST_ShellCommentsNotHeadings(t *testing.T) {
	raw := `Title: Mixed Comments
Author: Jane Doe <jane@example.com>
Posted: 2022-12-27
Revision: 1
News-Item-Format: 2.0

## Pre-upgrade requirements

Read before continuing.

## Configuration

# https://example.com/wiki/Parallel
# Make /bin/gzip a symlink
# emerge -v1 sys-apps/alternatives
`
	item := ParseNewsItem(raw)
	doc := TransformNewsAST(item)
	if doc.Report.Dialect != DialectMarkdownHash {
		t.Fatalf("expected DialectMarkdownHash, got %s", doc.Report.Dialect)
	}
	htmlOut := string(doc.RenderHTML("mixed"))
	if !strings.Contains(htmlOut, `<h3 id="mixed-pre-upgrade-requirements">Pre-upgrade requirements</h3>`) {
		t.Fatalf("missing expected heading in %s", htmlOut)
	}
	if strings.Contains(htmlOut, `<h3`+" id=\"mixed-emerge") || strings.Contains(htmlOut, `>https://example.com/wiki/Parallel</h3>`) || strings.Contains(htmlOut, `>Make /bin/gzip a symlink</h3>`) {
		t.Fatalf("shell/URL comments wrongly promoted to headings:\n%s", htmlOut)
	}
}

func TestTransformNewsAST_SetextMediaWikiAndSeparator(t *testing.T) {
	setextRaw := `Title: Setext
Author: Jane Doe <jane@example.com>
Posted: 2024-05-09
Revision: 1
News-Item-Format: 2.0

Deferring the upgrade
=====================

Keep existing version.

The rest still applies.
---

Another paragraph.
`
	setextDoc := TransformNewsAST(ParseNewsItem(setextRaw))
	if setextDoc.Report.Dialect != DialectSetextUnderline {
		t.Fatalf("expected DialectSetextUnderline, got %s", setextDoc.Report.Dialect)
	}
	setextHTML := string(setextDoc.RenderHTML("s"))
	if !strings.Contains(setextHTML, `<h3 id="s-deferring-the-upgrade">Deferring the upgrade</h3>`) {
		t.Fatalf("missing setext heading: %s", setextHTML)
	}
	if strings.Contains(setextHTML, `The rest still applies.</h3>`) {
		t.Fatalf("sentence ending in period before --- was wrongly treated as heading: %s", setextHTML)
	}

	mwRaw := `Title: MW
Author: Jane Doe <jane@example.com>
Posted: 2026-02-11
Revision: 1
News-Item-Format: 2.0

== Systemd users ==

Stop service.

== OpenRC users ===

Restart service.
`
	mwDoc := TransformNewsAST(ParseNewsItem(mwRaw))
	if mwDoc.Report.Dialect != DialectMediaWikiEquals {
		t.Fatalf("expected DialectMediaWikiEquals, got %s", mwDoc.Report.Dialect)
	}
	mwHTML := string(mwDoc.RenderHTML("mw"))
	if !strings.Contains(mwHTML, `<h3 id="mw-systemd-users">Systemd users</h3>`) || !strings.Contains(mwHTML, `<h3 id="mw-openrc-users">OpenRC users</h3>`) {
		t.Fatalf("missing mediawiki headings: %s", mwHTML)
	}
}

func TestTransformNewsAST_FootnotesEdgeCasesAndXSS(t *testing.T) {
	raw := `Title: Edge Footnotes and XSS
Author: Jane Doe <jane@example.com>
Posted: 2025-09-10
Revision: 1
News-Item-Format: 2.0

See [1], repeat [1], unmatched [99], and code ` + "`arr[1]`" + `.
Raw <script>alert(1)</script> tag.

  code[1] = 42
  <img src=x onerror=alert(1)>

[1] https://example.com/first
[1] https://example.com/duplicate
[2] javascript:alert(1)
[3] Unused plain note
`
	item := ParseNewsItem(raw)
	out := string(item.ToHTMLWithScope("edge"))

	if strings.Contains(out, "<script>") || strings.Contains(out, "<img ") || strings.Contains(out, `href="javascript:`) {
		t.Fatalf("unsafe HTML or scheme leaked into output:\n%s", out)
	}
	if !strings.Contains(out, `id="fnref-edge-1-1"`) || !strings.Contains(out, `id="fnref-edge-1-2"`) {
		t.Fatalf("expected multiple ref anchors for repeated [1]:\n%s", out)
	}
	if strings.Contains(out, `href="#fn-edge-99"`) {
		t.Fatalf("unmatched [99] must remain literal text:\n%s", out)
	}
	if strings.Contains(out, "duplicate") {
		t.Fatalf("duplicate footnote definition should be ignored:\n%s", out)
	}
	if !strings.Contains(out, `<li id="fn-edge-3" value="3">Unused plain note</li>`) {
		t.Fatalf("expected unused footnote [3] without backref:\n%s", out)
	}
	if !strings.Contains(out, `<code>arr[1]</code>`) && !strings.Contains(out, "`arr[1]`") {
		t.Fatalf("expected backtick span arr[1] untouched:\n%s", out)
	}
}

func BenchmarkTransformAndRenderNews(b *testing.B) {
	raw := `Title: Varnish Renamed
Author: Jane Doe <jane@example.com>
Posted: 2026-03-31
Revision: 1
News-Item-Format: 2.0

# Overview

Package renamed[0][1].

## Details

 - Change one
 - Change two

  emerge -av www-servers/vinyl-cache

[0] https://example.com/0
[1] https://example.com/1
`
	item := ParseNewsItem(raw)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = item.ToHTMLWithScope("bench")
	}
}
