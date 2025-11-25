package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// repoNamePattern validates repository names in org/repo format
	// Allows: alphanumerics, underscores, hyphens, and requires a single forward slash
	repoNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)
)

// RepositoryConfig represents a repository configuration
type RepositoryConfig struct {
	Name         string   `yaml:"name"`
	ExcludeDirs  []string `yaml:"exclude_dirs"`
	ExcludeFiles []string `yaml:"exclude_files"`
	Owners       []string `yaml:"-"` // Not serialized, used for CODEOWNERS
}

// Writer writes repository configurations to disk
type Writer struct {
	reposDir       string
	codeownersFile string
}

// NewWriter creates a new configuration writer
func NewWriter(reposDir, codeownersFile string) *Writer {
	return &Writer{
		reposDir:       reposDir,
		codeownersFile: codeownersFile,
	}
}

// Write writes a repository configuration to disk
func (w *Writer) Write(cfg RepositoryConfig, dryRun bool) error {
	// Validate repository name
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("repository name cannot be empty")
	}
	// Validate using allowlist: must be in org/repo format with allowed characters
	if !repoNamePattern.MatchString(cfg.Name) {
		return fmt.Errorf("invalid repository name: %q (must be in org/repo format with only alphanumerics, underscores, and hyphens)", cfg.Name)
	}

	// Generate filename from repository name
	filename := w.getFilename(cfg.Name)

	var targetPath string
	if dryRun {
		dryRunDir := filepath.Join(filepath.Dir(w.reposDir), "discovered-repos")
		targetPath = filepath.Join(dryRunDir, filename)
	} else {
		targetPath = filepath.Join(w.reposDir, filename)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write config file
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config to %s: %w", targetPath, err)
	}

	// Update CODEOWNERS if not dry run
	if !dryRun {
		if err := w.updateCodeowners(filename, cfg.Owners); err != nil {
			return fmt.Errorf("failed to update CODEOWNERS: %w", err)
		}
	}

	return nil
}

// getFilename generates a filename from repository name
func (w *Writer) getFilename(repoName string) string {
	// Extract repo name from "org/repo" format
	parts := strings.Split(repoName, "/")
	return parts[1] + ".yaml"
}

// updateCodeowners updates or adds an entry in the CODEOWNERS file
func (w *Writer) updateCodeowners(filename string, owners []string) error {
	if len(owners) == 0 {
		return fmt.Errorf("no owners specified for %s", filename)
	}

	// Normalize and deduplicate owners
	normalizedOwners := normalizeOwners(owners)
	if len(normalizedOwners) == 0 {
		return fmt.Errorf("all owners for %s were invalid after normalization", filename)
	}

	// Read existing CODEOWNERS file
	var lines []string
	data, err := os.ReadFile(w.codeownersFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		lines = strings.Split(string(data), "\n")
	}

	// Pattern for matching this repository's entry
	pattern := fmt.Sprintf("/repos/%s", filename)
	newEntry := fmt.Sprintf("/repos/%s %s", filename, strings.Join(normalizedOwners, " "))
	found := false

	// Look for existing entry and update it
	for i, line := range lines {
		if matchesPattern(line, pattern) {
			lines[i] = newEntry
			found = true
			break
		}
	}

	// If not found, append new entry
	if !found {
		// Ensure there's a blank line before adding if file exists and isn't empty
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, newEntry)
	}

	return w.writeCodeowners(lines)
}

// matchesPattern checks if a line matches the given CODEOWNERS pattern
func matchesPattern(line, pattern string) bool {
	// Strip inline comments and surrounding spaces
	trimmed := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
	// Match exact file path token (followed by space or end of line)
	return trimmed == pattern || strings.HasPrefix(trimmed, pattern+" ")
}

// normalizeOwners normalizes a list of owners: trim whitespace, ensure @ prefix, deduplicate
func normalizeOwners(owners []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, owner := range owners {
		owner = strings.TrimSpace(owner)
		if owner == "" {
			continue
		}
		// Ensure @ prefix for team/user handles
		if !strings.HasPrefix(owner, "@") {
			owner = "@" + owner
		}
		// Deduplicate
		if !seen[owner] {
			seen[owner] = true
			result = append(result, owner)
		}
	}
	return result
}

// writeCodeowners writes lines to CODEOWNERS file with proper formatting
func (w *Writer) writeCodeowners(lines []string) error {
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(w.codeownersFile, []byte(content), 0644)
}

// LoadRepositoryConfig loads a repository configuration from disk
func LoadRepositoryConfig(reposDir, filename string) (RepositoryConfig, error) {
	path := filepath.Join(reposDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return RepositoryConfig{}, err
	}

	var cfg RepositoryConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return RepositoryConfig{}, err
	}

	return cfg, nil
}
