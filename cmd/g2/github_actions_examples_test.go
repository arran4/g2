package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arran4/g2"
	"gopkg.in/yaml.v3"
)

type ghWorkflow struct {
	Name        string            `yaml:"name"`
	On          any               `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Concurrency struct {
		Group            string `yaml:"group"`
		CancelInProgress bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]struct {
		Name           string            `yaml:"name"`
		RunsOn         string            `yaml:"runs-on"`
		Needs          any               `yaml:"needs"`
		TimeoutMinutes int               `yaml:"timeout-minutes"`
		Permissions    map[string]string `yaml:"permissions"`
		Steps          []struct {
			Name string         `yaml:"name"`
			ID   string         `yaml:"id"`
			Uses string         `yaml:"uses"`
			With map[string]any `yaml:"with"`
			Env  map[string]any `yaml:"env"`
			Run  string         `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func TestGitHubActionsPagesExamplesValidAndSanitized(t *testing.T) {
	files := []struct {
		rel          string
		expectSubcmd string
	}{
		{rel: "../../examples/github-actions/pages-single-overlay.yml", expectSubcmd: "g2 overlay site generate -out _site"},
		{rel: "../../examples/github-actions/pages-multi-overlay.yml", expectSubcmd: "g2 overlays site generate -out _site"},
	}

	forbiddenStrings := []string{
		"arrans_overlay",
		"gentoo-packages",
		"rm -rf /",
		"gh-pages",
		"example@example",
	}

	for _, tc := range files {
		t.Run(filepath.Base(tc.rel), func(t *testing.T) {
			raw, err := os.ReadFile(tc.rel)
			if err != nil {
				t.Fatalf("read %s: %v", tc.rel, err)
			}
			content := string(raw)
			for _, bad := range forbiddenStrings {
				if strings.Contains(content, bad) {
					t.Fatalf("workflow %s contains forbidden hardcoded/branch reference %q", tc.rel, bad)
				}
			}

			var wf ghWorkflow
			if err := yaml.Unmarshal(raw, &wf); err != nil {
				t.Fatalf("yaml.Unmarshal(%s): %v", tc.rel, err)
			}
			if wf.Name == "" || wf.On == nil {
				t.Fatalf("workflow %s missing name or on trigger", tc.rel)
			}
			if len(wf.Permissions) != 0 {
				t.Fatalf("workflow %s should have empty top-level permissions, got %v", tc.rel, wf.Permissions)
			}
			if wf.Concurrency.Group == "" || wf.Concurrency.CancelInProgress {
				t.Fatalf("workflow %s should set concurrency group with cancel-in-progress: false", tc.rel)
			}

			buildJob, ok := wf.Jobs["build"]
			if !ok {
				t.Fatalf("workflow %s missing build job", tc.rel)
			}
			if buildJob.Permissions["contents"] != "read" || buildJob.TimeoutMinutes <= 0 {
				t.Fatalf("build job permissions/timeout invalid: perms=%v timeout=%d", buildJob.Permissions, buildJob.TimeoutMinutes)
			}

			var hasCheckout, hasG2Action, hasCmd, hasValidate, hasUpload bool
			for _, st := range buildJob.Steps {
				if strings.HasPrefix(st.Uses, "actions/checkout@") {
					hasCheckout = true
				}
				if strings.HasPrefix(st.Uses, "arran4/g2-action@") {
					hasG2Action = true
					if st.With["mode"] != "install" {
						t.Fatalf("expected g2-action mode: install, got %v", st.With["mode"])
					}
				}
				if strings.Contains(st.Run, tc.expectSubcmd) {
					hasCmd = true
				}
				if strings.Contains(st.Run, "test -s _site/index.html") && strings.Contains(st.Run, "test -s _site/stats/index.html") {
					hasValidate = true
				}
				if strings.HasPrefix(st.Uses, "actions/upload-pages-artifact@") {
					hasUpload = true
					if st.With["path"] != "_site" {
						t.Fatalf("expected upload-pages-artifact path=_site, got %v", st.With["path"])
					}
				}
			}
			if !hasCheckout || !hasG2Action || !hasCmd || !hasValidate || !hasUpload {
				t.Fatalf("build job missing required steps: checkout=%v g2=%v cmd=%v validate=%v upload=%v", hasCheckout, hasG2Action, hasCmd, hasValidate, hasUpload)
			}

			deployJob, ok := wf.Jobs["deploy"]
			if !ok {
				t.Fatalf("workflow %s missing deploy job", tc.rel)
			}
			if deployJob.Permissions["pages"] != "write" || deployJob.Permissions["id-token"] != "write" || deployJob.TimeoutMinutes <= 0 {
				t.Fatalf("deploy job permissions/timeout invalid: perms=%v timeout=%d", deployJob.Permissions, deployJob.TimeoutMinutes)
			}
			hasDeploy := false
			for _, st := range deployJob.Steps {
				if strings.HasPrefix(st.Uses, "actions/deploy-pages@") {
					hasDeploy = true
				}
			}
			if !hasDeploy {
				t.Fatalf("deploy job missing actions/deploy-pages")
			}
		})
	}

	// Verify sample artifact path (_site/index.html and _site/stats/index.html) via site generation
	siteData, err := parseRepo(os.DirFS("../../testdata/test_overlay"), ".", "test-overlay", false, nil)
	if err != nil {
		t.Fatalf("parseRepo failed: %v", err)
	}
	outDir := filepath.Join(t.TempDir(), "_site")
	if err := generateSite(outDir, []*g2.SiteData{siteData}, 90*24*time.Hour, "3mo", GenerationInfo{RecentDuration: "3mo"}); err != nil {
		t.Fatalf("generateSite failed: %v", err)
	}
	for _, rel := range []string{"index.html", "stats/index.html"} {
		info, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(rel)))
		if err != nil || info.Size() == 0 {
			t.Fatalf("expected non-empty %s in _site, err=%v", rel, err)
		}
	}
}
