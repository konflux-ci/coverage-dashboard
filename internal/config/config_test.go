package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRepositoryConfigMarshal(t *testing.T) {
	cfg := RepositoryConfig{
		Name: "konflux-ci/caching",
		ExcludeDirs: []string{
			"vendor/",
			"hack/",
			"/fake(/|$)",
		},
		ExcludeFiles: []string{
			"zz_generated.deepcopy.go",
			"openapi_generated.go",
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	result := string(data)

	// Check that all fields are present
	expectedFields := []string{
		"name: konflux-ci/caching",
		"exclude_dirs:",
		"- vendor/",
		"- hack/",
		"- /fake(/|$)",
		"exclude_files:",
		"- zz_generated.deepcopy.go",
		"- openapi_generated.go",
	}

	for _, field := range expectedFields {
		if !strings.Contains(result, field) {
			t.Errorf("marshaled YAML missing field: %q\nGot:\n%s", field, result)
		}
	}
}

func TestRepositoryConfigUnmarshal(t *testing.T) {
	yamlContent := `name: konflux-ci/caching
exclude_dirs:
  - vendor/
  - hack/
  - /fake(/|$)
exclude_files:
  - zz_generated.deepcopy.go
  - openapi_generated.go
`

	var cfg RepositoryConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if cfg.Name != "konflux-ci/caching" {
		t.Errorf("Name = %q, want %q", cfg.Name, "konflux-ci/caching")
	}

	expectedDirs := []string{"vendor/", "hack/", "/fake(/|$)"}
	if len(cfg.ExcludeDirs) != len(expectedDirs) {
		t.Errorf("ExcludeDirs length = %d, want %d", len(cfg.ExcludeDirs), len(expectedDirs))
	}

	expectedFiles := []string{"zz_generated.deepcopy.go", "openapi_generated.go"}
	if len(cfg.ExcludeFiles) != len(expectedFiles) {
		t.Errorf("ExcludeFiles length = %d, want %d", len(cfg.ExcludeFiles), len(expectedFiles))
	}
}

func TestWrite(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	reposDir := filepath.Join(tempDir, "repos")
	codeownersFile := filepath.Join(tempDir, "CODEOWNERS")

	w := NewWriter(reposDir, codeownersFile)

	cfg := RepositoryConfig{
		Name: "konflux-ci/test-repo",
		ExcludeDirs: []string{
			"vendor/",
			"hack/",
		},
		ExcludeFiles: []string{
			"zz_generated.deepcopy.go",
		},
		Owners: []string{"@konflux-ci/test-team"},
	}

	// Write config
	if err := w.Write(cfg, false); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify config file was created
	configPath := filepath.Join(reposDir, "test-repo.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config file was not created: %s", configPath)
	}

	// Load and verify config content
	loadedCfg, err := LoadRepositoryConfig(reposDir, "test-repo.yaml")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loadedCfg.Name != cfg.Name {
		t.Errorf("loaded Name = %q, want %q", loadedCfg.Name, cfg.Name)
	}

	// Verify CODEOWNERS was updated
	codeownersContent, err := os.ReadFile(codeownersFile)
	if err != nil {
		t.Fatalf("failed to read CODEOWNERS: %v", err)
	}

	expectedEntry := "/repos/test-repo.yaml @konflux-ci/test-team"
	if !strings.Contains(string(codeownersContent), expectedEntry) {
		t.Errorf("CODEOWNERS missing entry: %q\nGot:\n%s", expectedEntry, string(codeownersContent))
	}
}

func TestWriteDryRun(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()

	// Change to temp directory for dry run
	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	reposDir := filepath.Join(tempDir, "repos")
	codeownersFile := filepath.Join(tempDir, "CODEOWNERS")

	w := NewWriter(reposDir, codeownersFile)

	cfg := RepositoryConfig{
		Name: "konflux-ci/test-repo",
		ExcludeDirs: []string{
			"vendor/",
		},
		Owners: []string{"@konflux-ci/test-team"},
	}

	// Write config in dry-run mode
	if err := w.Write(cfg, true); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	// Verify config file was created in discovered-repos
	dryRunPath := filepath.Join(tempDir, "discovered-repos", "test-repo.yaml")
	if _, err := os.Stat(dryRunPath); os.IsNotExist(err) {
		t.Errorf("dry-run config file was not created: %s", dryRunPath)
	}

	// Verify CODEOWNERS was NOT updated in dry-run mode
	if _, err := os.Stat(codeownersFile); !os.IsNotExist(err) {
		t.Errorf("CODEOWNERS should not be created in dry-run mode")
	}

	// Verify config was NOT created in repos directory
	configPath := filepath.Join(reposDir, "test-repo.yaml")
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Errorf("config file should not be created in repos dir during dry-run")
	}
}
