package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test-skill.md")

	content := `---
name: TestSkill
description: A test skill
whenToUse: When testing
effort: low
---

# Test Skill

This is the skill body.
Run this command: echo hello
`
	os.WriteFile(path, []byte(content), 0o644)

	skill, err := parseSkillFile(path, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skill.Name != "TestSkill" {
		t.Errorf("expected name 'TestSkill', got %q", skill.Name)
	}
	if skill.Description != "A test skill" {
		t.Errorf("expected description 'A test skill', got %q", skill.Description)
	}
	if skill.WhenToUse != "When testing" {
		t.Errorf("expected whenToUse 'When testing', got %q", skill.WhenToUse)
	}
	if skill.Effort != "low" {
		t.Errorf("expected effort 'low', got %q", skill.Effort)
	}
	if skill.Source != "user" {
		t.Errorf("expected source 'user', got %q", skill.Source)
	}
	if !strings.Contains(skill.Content, "Test Skill") {
		t.Errorf("expected content to contain 'Test Skill', got %q", skill.Content)
	}
	if !strings.Contains(skill.Content, "echo hello") {
		t.Errorf("expected content to contain 'echo hello', got %q", skill.Content)
	}
}

func TestParseSkillFile_NoFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "simple.md")

	content := "# Just a simple skill\n\nDo the thing.\n"
	os.WriteFile(path, []byte(content), 0o644)

	skill, err := parseSkillFile(path, "project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skill.Name != "simple" {
		t.Errorf("expected name 'simple' (from filename), got %q", skill.Name)
	}
	if skill.Description != "" {
		t.Errorf("expected empty description, got %q", skill.Description)
	}
	if !strings.Contains(skill.Content, "Just a simple skill") {
		t.Errorf("expected content, got %q", skill.Content)
	}
}

func TestLoadDir(t *testing.T) {
	tmp := t.TempDir()

	// Create some skill files
	os.WriteFile(filepath.Join(tmp, "skill1.md"), []byte(`---
name: Skill1
description: First skill
---

Do first thing.
`), 0o644)

	os.WriteFile(filepath.Join(tmp, "skill2.md"), []byte(`---
name: Skill2
description: Second skill
---

Do second thing.
`), 0o644)

	// Non-markdown file should be ignored
	os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte("not a skill"), 0o644)

	skills, err := loadDir(tmp, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestLoadDir_Empty(t *testing.T) {
	tmp := t.TempDir()

	skills, err := loadDir(tmp, "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("expected 0 skills from empty dir, got %d", len(skills))
	}
}

func TestLoadDir_Nonexistent(t *testing.T) {
	skills, err := loadDir("/nonexistent/dir", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skills != nil {
		t.Errorf("expected nil skills for nonexistent dir, got %v", skills)
	}
}

func TestSkillsPrompt(t *testing.T) {
	skills := []Skill{
		{
			Name:        "TestSkill",
			Description: "A test skill",
			WhenToUse:   "When testing",
			Content:     "Run the tests",
		},
	}

	prompt := SkillsPrompt(skills)

	if !strings.Contains(prompt, "TestSkill") {
		t.Errorf("expected prompt to contain skill name, got %q", prompt)
	}
	if !strings.Contains(prompt, "A test skill") {
		t.Errorf("expected prompt to contain description, got %q", prompt)
	}
	if !strings.Contains(prompt, "When testing") {
		t.Errorf("expected prompt to contain whenToUse, got %q", prompt)
	}
	if !strings.Contains(prompt, "Run the tests") {
		t.Errorf("expected prompt to contain content, got %q", prompt)
	}
}

func TestSkillsPrompt_Empty(t *testing.T) {
	prompt := SkillsPrompt(nil)
	if prompt != "" {
		t.Errorf("expected empty prompt for no skills, got %q", prompt)
	}
}

func TestLoadAll(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Create user skills directory
	skillsDir := filepath.Join(tmp, ".claude", "skills")
	os.MkdirAll(skillsDir, 0o755)

	os.WriteFile(filepath.Join(skillsDir, "myskill.md"), []byte(`---
name: MySkill
description: My custom skill
---

Do the custom thing.
`), 0o644)

	skills, err := LoadAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) < 1 {
		t.Fatalf("expected at least 1 skill, got %d", len(skills))
	}

	found := false
	for _, s := range skills {
		if s.Name == "MySkill" {
			found = true
			if s.Source != "user" {
				t.Errorf("expected source 'user', got %q", s.Source)
			}
		}
	}
	if !found {
		t.Error("expected to find MySkill in loaded skills")
	}
}
