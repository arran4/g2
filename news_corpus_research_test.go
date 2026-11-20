package g2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type corpusGoldenCase struct {
	File                 string   `json:"file"`
	Format               string   `json:"format"`
	ExpectedDialect      string   `json:"expected_dialect"`
	ExpectedHeadings     []string `json:"expected_headings"`
	ExpectedFootnoteDefs []string `json:"expected_footnote_defs"`
	ExpectedFootnoteRefs []string `json:"expected_footnote_refs"`
	ExpectedURLs         []string `json:"expected_urls"`
	ForbiddenHeadings    []string `json:"forbidden_headings"`
	Notes                string   `json:"notes"`
}

func TestNewsCorpusGoldenFixturesAnonymizedAndSafe(t *testing.T) {
	base := filepath.Join("testdata", "news_corpus")
	raw, err := os.ReadFile(filepath.Join(base, "cases.json"))
	if err != nil {
		t.Fatalf("read cases.json: %v", err)
	}
	var cases []corpusGoldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("parse cases.json: %v", err)
	}
	if len(cases) < 10 {
		t.Fatalf("expected >= 10 golden corpus cases, got %d", len(cases))
	}

	emailRe := regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	for _, tc := range cases {
		t.Run(filepath.Base(tc.File), func(t *testing.T) {
			contentBytes, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(tc.File)))
			if err != nil {
				t.Fatalf("read fixture %s: %v", tc.File, err)
			}
			content := string(contentBytes)

			for _, email := range emailRe.FindAllString(content, -1) {
				if !strings.HasSuffix(strings.ToLower(email), "@example.com") {
					t.Fatalf("non-anonymized email %q found in fixture %s", email, tc.File)
				}
			}

			item := ParseNewsItem(content)
			if item.Title == "" {
				t.Fatalf("missing Title in %s", tc.File)
			}
			if item.NewsItemFormat != tc.Format {
				t.Fatalf("format mismatch in %s: got %q, want %q", tc.File, item.NewsItemFormat, tc.Format)
			}

			rendered := string(item.ToHTMLTemplate())
			if strings.Contains(rendered, "<script>") || strings.Contains(rendered, "<img ") {
				t.Fatalf("raw HTML leaked unescaped into ToHTMLTemplate output for %s: %s", tc.File, rendered)
			}
			if strings.Contains(rendered, `href="javascript:`) || strings.Contains(rendered, `href="data:`) {
				t.Fatalf("unsafe URL scheme linked in %s: %s", tc.File, rendered)
			}
		})
	}
}

func TestNewsCorpusLocalDistributionReport(t *testing.T) {
	corpusDir := os.Getenv("G2_NEWS_CORPUS_DIR")
	if corpusDir == "" {
		t.Skip("set G2_NEWS_CORPUS_DIR to run local multi-overlay corpus distribution analysis")
	}

	var total, fmt1, fmt2 int
	err := filepath.WalkDir(corpusDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".txt") {
			return err
		}
		if !strings.Contains(filepath.ToSlash(path), "/metadata/news/") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		total++
		item := ParseNewsItem(string(b))
		switch item.NewsItemFormat {
		case "1.0":
			fmt1++
		case "2.0":
			fmt2++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking corpus dir: %v", err)
	}
	t.Logf("Corpus summary: total=%d format_1_0=%d format_2_0=%d", total, fmt1, fmt2)
}
