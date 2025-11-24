package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	// Generate filename from repository name
	filename := w.getFilename(cfg.Name)

	var targetPath string
	if dryRun {
		targetPath = filepath.Join("discovered-repos", filename)
		// Ensure discovered-repos directory exists
		if err := os.MkdirAll("discovered-repos", 0755); err != nil {
			return fmt.Errorf("failed to create discovered-repos directory: %w", err)
		}
	} else {
		targetPath = filepath.Join(w.reposDir, filename)
		// Ensure repos directory exists
		if err := os.MkdirAll(w.reposDir, 0755); err != nil {
			return fmt.Errorf("failed to create repos directory: %w", err)
		}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("  ✅ Created: %s\n", filename)

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
	if len(parts) == 2 {
		return parts[1] + ".yaml"
	}
	return repoName + ".yaml"
}

// updateCodeowners appends an entry to the CODEOWNERS file
func (w *Writer) updateCodeowners(filename string, owners []string) error {
	if len(owners) == 0 {
		return nil
	}

	// Open CODEOWNERS file in append mode
	f, err := os.OpenFile(w.codeownersFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write entry
	entry := fmt.Sprintf("/repos/%s %s\n", filename, strings.Join(owners, " "))
	if _, err := f.WriteString(entry); err != nil {
		return err
	}

	return nil
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
