package main

import (
	"os"
	"path/filepath"
	"testing"
	"strings"
)

func TestSkillInstallLocal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-skill-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	sourceDir := filepath.Join(tempDir, "source", "my-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# My Skill\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	c := &MainArgConfig{Args: []string{"g2"}}
	err = c.cmdSkillInstall([]string{"--scope", "project", sourceDir})
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	destDir := filepath.Join(projectDir, ".agents", "skills", "my-skill")
	if _, err := os.Stat(filepath.Join(destDir, "SKILL.md")); err != nil {
		t.Fatalf("SKILL.md not found in dest: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "skill-metadata.json")); err != nil {
		t.Fatalf("skill-metadata.json not found in dest: %v", err)
	}
}

func TestSkillUpdateLocal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-skill-test-update-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	sourceDir := filepath.Join(tempDir, "source", "my-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# My Skill\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil { t.Fatal(err) }
	defer func() { _ = os.Chdir(cwd) }()

	c := &MainArgConfig{Args: []string{"g2"}}

	// Install
	if err := c.cmdSkillInstall([]string{"--scope", "project", sourceDir}); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Update without changes
	if err := c.cmdSkillUpdate([]string{"--scope", "project", "my-skill"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Modify Source
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# My Skill\nUpdated!\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Update with changes
	if err := c.cmdSkillUpdate([]string{"--scope", "project", "my-skill"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Check if updated
	destDir := filepath.Join(projectDir, ".agents", "skills", "my-skill")
	content, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if string(content) != "# My Skill\nUpdated!\n" {
		t.Fatalf("Skill not updated! Content: %s", string(content))
	}

	// Modify dest (local modifications)
	if err := os.WriteFile(filepath.Join(destDir, "SKILL.md"), []byte("Local changes"), 0644); err != nil {
		t.Fatal(err)
	}

	// Update should skip
	if err := c.cmdSkillUpdate([]string{"--scope", "project", "my-skill"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	content2, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if string(content2) != "Local changes" {
		t.Fatalf("Skill overwritten despite local changes!")
	}

	// Update with force
	if err := c.cmdSkillUpdate([]string{"--scope", "project", "--force", "my-skill"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	content3, _ := os.ReadFile(filepath.Join(destDir, "SKILL.md"))
	if string(content3) != "# My Skill\nUpdated!\n" {
		t.Fatalf("Skill not forced updated! Content: %s", string(content3))
	}
}

func TestSkillRemove(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-skill-test-remove-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	sourceDir := filepath.Join(tempDir, "source", "my-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# My Skill\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil { t.Fatal(err) }
	defer func() { _ = os.Chdir(cwd) }()

	c := &MainArgConfig{Args: []string{"g2"}}

	// Install
	if err := c.cmdSkillInstall([]string{"--scope", "project", sourceDir}); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Remove
	if err := c.cmdSkillRemove([]string{"--scope", "project", "my-skill"}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	destDir := filepath.Join(projectDir, ".agents", "skills", "my-skill")
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		t.Fatalf("Skill directory still exists after remove!")
	}
}

func TestSkillList(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-skill-test-list-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	sourceDir := filepath.Join(tempDir, "source", "my-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# My Skill\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil { t.Fatal(err) }
	defer func() { _ = os.Chdir(cwd) }()

	c := &MainArgConfig{Args: []string{"g2"}}

	// Install
	if err := c.cmdSkillInstall([]string{"--scope", "project", sourceDir}); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// List
	if err := c.cmdSkillList([]string{"--scope", "project"}); err != nil {
		t.Fatalf("List failed: %v", err)
	}
}

func TestSkillInspect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-skill-test-inspect-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	sourceDir := filepath.Join(tempDir, "source", "my-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# My Skill\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil { t.Fatal(err) }
	defer func() { _ = os.Chdir(cwd) }()

	c := &MainArgConfig{Args: []string{"g2"}}

	// Install
	if err := c.cmdSkillInstall([]string{"--scope", "project", sourceDir}); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Inspect
	if err := c.cmdSkillInspect([]string{"--scope", "project", "my-skill"}); err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
}

func TestSkillInstallInvalid(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "g2-skill-test-invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	sourceDir := filepath.Join(tempDir, "source", "invalid-skill")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	// No SKILL.md written

	projectDir := filepath.Join(tempDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil { t.Fatal(err) }
	defer func() { _ = os.Chdir(cwd) }()

	c := &MainArgConfig{Args: []string{"g2"}}

	// Install should fail
	err = c.cmdSkillInstall([]string{"--scope", "project", sourceDir})
	if err == nil {
		t.Fatalf("Install should have failed because SKILL.md is missing")
	}

	if !strings.Contains(err.Error(), "SKILL.md not found") {
		t.Fatalf("Unexpected error message: %v", err)
	}
}
