// Package skills loads custom skill definitions from markdown files
// in ~/.claude/skills and project-level .claude/skills directories.
package skills

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/btwiuse/claude-code/config"
)

// Skill represents a loaded skill definition parsed from a markdown file.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	WhenToUse   string `json:"whenToUse,omitempty"`
	Effort      string `json:"effort,omitempty"`
	Source      string `json:"source"` // "user", "project", "bundled"
	FilePath    string `json:"filePath"`
	Content     string `json:"content"` // Full markdown body (after frontmatter)
}

// LoadAll discovers and loads skills from all sources.
// Priority (highest to lowest): project, user.
func LoadAll() ([]Skill, error) {
	var all []Skill

	// Load user skills (~/.claude/skills/)
	userSkills, err := loadFromUserDir()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	all = append(all, userSkills...)

	// Load project skills (./.claude/skills/)
	projectSkills, err := loadFromProjectDir()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	all = append(all, projectSkills...)

	return all, nil
}

// loadFromUserDir loads skills from ~/.claude/skills/.
func loadFromUserDir() ([]Skill, error) {
	claudeHome, err := config.ClaudeHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(claudeHome, "skills")
	return loadDir(dir, "user")
}

// loadFromProjectDir loads skills from ./.claude/skills/ in the current directory.
func loadFromProjectDir() ([]Skill, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cwd, ".claude", "skills")
	return loadDir(dir, "project")
}

// loadDir reads all .md files from a directory and parses them as skills.
func loadDir(dir string, source string) ([]Skill, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	var skills []Skill

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		skill, err := parseSkillFile(path, source)
		if err != nil {
			return nil // Skip unparseable files
		}
		skills = append(skills, *skill)
		return nil
	})

	return skills, err
}

// parseSkillFile reads a markdown file with optional YAML frontmatter.
//
// Format:
//
//	---
//	name: MySkill
//	description: Does something useful
//	whenToUse: When the user asks for...
//	effort: medium
//	---
//
//	# Skill body
//	Instructions and content here...
func parseSkillFile(path string, source string) (*Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	skill := &Skill{
		Source:   source,
		FilePath: path,
	}

	// Default name from filename
	base := filepath.Base(path)
	skill.Name = strings.TrimSuffix(base, ".md")

	scanner := bufio.NewScanner(f)
	var (
		inFrontmatter bool
		body          strings.Builder
		frontmatter   []string
	)

	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 && strings.TrimSpace(line) == "---" {
			inFrontmatter = true
			continue
		}

		if inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = false
				continue
			}
			frontmatter = append(frontmatter, line)
			continue
		}

		body.WriteString(line)
		body.WriteString("\n")
	}

	// Parse frontmatter (simple YAML key: value)
	for _, line := range frontmatter {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch strings.ToLower(key) {
		case "name":
			skill.Name = value
		case "description":
			skill.Description = value
		case "whentouse":
			skill.WhenToUse = value
		case "effort":
			skill.Effort = value
		}
	}

	skill.Content = strings.TrimSpace(body.String())

	return skill, nil
}

// SkillsPrompt builds a system prompt section from loaded skills.
func SkillsPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## Custom Skills\n\n")
	sb.WriteString("The following custom skills are available:\n\n")

	for _, s := range skills {
		sb.WriteString("### ")
		sb.WriteString(s.Name)
		sb.WriteString("\n")
		if s.Description != "" {
			sb.WriteString(s.Description)
			sb.WriteString("\n")
		}
		if s.WhenToUse != "" {
			sb.WriteString("Use when: ")
			sb.WriteString(s.WhenToUse)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		if s.Content != "" {
			sb.WriteString(s.Content)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}
