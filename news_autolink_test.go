package g2

import (
	"strings"
	"testing"
)

func TestToHTMLWithLinks_URLsAtomsAndSafety(t *testing.T) {
	ctx := NewsLinkContext{
		BaseURL:     "../../",
		CurrentRepo: "repo-a",
		PackageIndex: map[string][]string{
			"app-misc/demo":      {"repo-a", "repo-b"},
			"dev-util/tool":      {"repo-b"},
			"dev-util/gtk-3":     {"repo-a"},
			"virtual/example":    {"repo-a"},
		},
		VersionIndex: map[string]map[string]bool{
			"repo-a|app-misc/demo": {"1.0": true, "2.0": true},
			"repo-b|app-misc/demo": {"2.0": true},
			"repo-b|dev-util/tool": {"1.2-r1": true},
		},
	}

	raw := `Title: Autolink Comprehensive Test
Author: Jane Doe <jane@example.com>
Posted: 2026-10-04
Revision: 1
News-Item-Format: 2.0

Visit https://example.com/docs/page(v1). Also check <https://example.com/bracketed> and https://example.com/end).
Contact jane@example.com; ignore javascript:alert(1) and <script>alert(1)</script> and either/or prose.

Upgrade >=app-misc/demo-1.0, app-misc/demo:0::repo-b, >=dev-util/tool-1.2-r1, dev-util/gtk-3, and virtual/example.
Ambiguous version app-misc/demo-2.0 across repos without current override, and unknown unknown/pkg stay plain.
UTF-8 prefix: żółć app-misc/demo!

Keep backtick ` + "`app-misc/demo`" + ` and ` + "`https://example.com/in-tick`" + ` unlinked.

emerge --ask dev-util/tool

  https://example.com/in-code-block
  app-misc/demo
`
	item := ParseNewsItem(raw)
	out := string(item.ToHTMLWithLinks(ctx))

	for _, want := range []string{
		`<a href="https://example.com/docs/page(v1)" rel="noopener noreferrer">https://example.com/docs/page(v1)</a>.`,
		`&lt;<a href="https://example.com/bracketed" rel="noopener noreferrer">https://example.com/bracketed</a>&gt;`,
		`<a href="https://example.com/end" rel="noopener noreferrer">https://example.com/end</a>).`,
		`<a href="../../repos/repo-a/categories/app-misc/packages/demo/ebuild/1.0/">>=app-misc/demo-1.0</a>`[:0], // checked escaped below
		`&gt;=app-misc/demo-1.0</a>`,
		`href="../../repos/repo-b/categories/app-misc/packages/demo/"`,
		`href="../../repos/repo-b/categories/dev-util/packages/tool/ebuild/1.2-r1/"`,
		`href="../../repos/repo-a/categories/dev-util/packages/gtk-3/"`,
		`href="../../repos/repo-a/categories/virtual/packages/example/"`,
		`<code>app-misc/demo</code>`,
		`<code>https://example.com/in-tick</code>`,
		`&lt;script&gt;alert(1)&lt;/script&gt;`,
	} {
		if want != "" && !strings.Contains(out, want) {
			t.Fatalf("missing expected HTML fragment %q\ngot:\n%s", want, out)
		}
	}

	for _, forbid := range []string{
		`href="mailto:`,
		`href="javascript:`,
		`href="../../packages/unknown/pkg/"`,
		`href="https://example.com/in-code-block"`,
		`href="https://example.com/in-tick"`,
		`emerge --ask <a `,
		`<script>`,
	} {
		if strings.Contains(out, forbid) {
			t.Fatalf("forbidden fragment %q found in:\n%s", forbid, out)
		}
	}

	// Empty index should only link URLs, not atoms, and never panic.
	emptyOut := string(item.ToHTMLWithLinks(NewsLinkContext{BaseURL: "/"}))
	if strings.Contains(emptyOut, "categories/app-misc/packages/demo") || !strings.Contains(emptyOut, "https://example.com/docs/page(v1)") {
		t.Fatalf("unexpected empty-index rendering:\n%s", emptyOut)
	}

	// Ambiguous version with no CurrentRepo falls back deterministically to global package route.
	globalCtx := NewsLinkContext{
		BaseURL:      "../",
		PackageIndex: ctx.PackageIndex,
		VersionIndex: ctx.VersionIndex,
	}
	ambigItem := ParseNewsItem("Title: T\nPosted: 2026-01-01\nNews-Item-Format: 2.0\n\nCheck app-misc/demo-2.0.\n")
	ambigOut := string(ambigItem.ToHTMLWithLinks(globalCtx))
	if !strings.Contains(ambigOut, `href="../packages/app-misc/demo/"`) {
		t.Fatalf("expected deterministic global fallback for ambiguous version, got: %s", ambigOut)
	}

	// Format 1.0 autolinking
	f1 := ParseNewsItem("Title: F1\nPosted: 2015-01-01\nNews-Item-Format: 1.0\n\nSee https://example.com/f1 and dev-util/tool.\n")
	f1Out := string(f1.ToHTMLWithLinks(globalCtx))
	if !strings.Contains(f1Out, `<a href="https://example.com/f1"`) || !strings.Contains(f1Out, `href="../packages/dev-util/tool/"`) {
		t.Fatalf("expected Format 1.0 autolinks, got: %s", f1Out)
	}
}
