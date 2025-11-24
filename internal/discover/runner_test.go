package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestNewRunner(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		setToken      bool
		expectError   bool
		errorContains string
	}{
		{
			name: "with tokens",
			config: Config{
				Organization:   "test-org",
				ReposDir:       "repos",
				CodeownersFile: "CODEOWNERS",
				DryRun:         true,
			},
			setToken:    true,
			expectError: false,
		},
		{
			name: "without tokens in dry run",
			config: Config{
				Organization:   "test-org",
				ReposDir:       "repos",
				CodeownersFile: "CODEOWNERS",
				DryRun:         true,
			},
			setToken:    false,
			expectError: false,
		},
		{
			name: "without write token in apply mode",
			config: Config{
				Organization:   "test-org",
				ReposDir:       "repos",
				CodeownersFile: "CODEOWNERS",
				DryRun:         false,
			},
			setToken:      false,
			expectError:   true,
			errorContains: "GITHUB_WRITE_TOKEN is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.setToken {
				os.Setenv("GITHUB_READ_TOKEN", "test-read-token")
				os.Setenv("GITHUB_WRITE_TOKEN", "test-write-token")
				defer os.Unsetenv("GITHUB_READ_TOKEN")
				defer os.Unsetenv("GITHUB_WRITE_TOKEN")
			} else {
				os.Unsetenv("GITHUB_READ_TOKEN")
				os.Unsetenv("GITHUB_WRITE_TOKEN")
			}

			runner, err := NewRunner(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error %q does not contain %q", err.Error(), tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if runner == nil {
				t.Fatal("Runner is nil")
			}

			if runner.config.Organization != tt.config.Organization {
				t.Errorf("Organization = %q, want %q", runner.config.Organization, tt.config.Organization)
			}

			if runner.githubClient == nil {
				t.Error("GitHub read client is nil")
			}

			if runner.writeClient == nil {
				t.Error("GitHub write client is nil")
			}
		})
	}
}

func TestFilterNewRepositories(t *testing.T) {
	runner := &Runner{
		existingRepos: map[string]bool{
			"test-org/repo1": true,
			"test-org/repo2": true,
		},
		config: Config{
			Organization: "test-org",
		},
	}

	repos := []*github.Repository{
		{Name: github.String("repo1")},
		{Name: github.String("repo2")},
		{Name: github.String("repo3")},
		{Name: github.String("repo4")},
	}

	newRepos := runner.filterNewRepositories(repos)

	if len(newRepos) != 2 {
		t.Errorf("Expected 2 new repos, got %d", len(newRepos))
	}

	// Verify the correct repos were filtered
	expectedNew := map[string]bool{
		"repo3": true,
		"repo4": true,
	}

	for _, repo := range newRepos {
		if !expectedNew[repo.GetName()] {
			t.Errorf("Unexpected repo in filtered list: %s", repo.GetName())
		}
	}
}

func TestLoadExistingRepos(t *testing.T) {
	// Create a temporary directory with test config files
	tempDir := t.TempDir()
	reposDir := filepath.Join(tempDir, "repos")
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test config files
	configs := []struct {
		filename string
		content  string
	}{
		{
			filename: "repo1.yaml",
			content:  "name: test-org/repo1\n",
		},
		{
			filename: "repo2.yaml",
			content:  "name: test-org/repo2\n",
		},
		{
			filename: "invalid.yaml",
			content:  "invalid: yaml: content:\n  - bad:\nstructure",
		},
	}

	for _, cfg := range configs {
		path := filepath.Join(reposDir, cfg.filename)
		if err := os.WriteFile(path, []byte(cfg.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	runner := &Runner{
		config: Config{
			ReposDir: reposDir,
		},
	}

	err := runner.loadExistingRepos()
	if err != nil {
		t.Errorf("loadExistingRepos failed: %v", err)
	}

	// Should have loaded 2 valid repos
	if len(runner.existingRepos) != 2 {
		t.Errorf("Expected 2 existing repos, got %d", len(runner.existingRepos))
	}

	// Verify the correct repos were loaded
	expectedRepos := []string{"test-org/repo1", "test-org/repo2"}
	for _, repoName := range expectedRepos {
		if !runner.existingRepos[repoName] {
			t.Errorf("Expected repo %q to be loaded", repoName)
		}
	}
}

func TestGetCurrentRepoName(t *testing.T) {
	tests := []struct {
		name        string
		remoteURL   string
		expected    string
		shouldError bool
	}{
		{
			name:      "HTTPS URL",
			remoteURL: "https://github.com/konflux-ci/coverage-dashboard.git",
			expected:  "coverage-dashboard",
		},
		{
			name:      "HTTPS URL without .git",
			remoteURL: "https://github.com/konflux-ci/coverage-dashboard",
			expected:  "coverage-dashboard",
		},
		{
			name:      "SSH URL",
			remoteURL: "git@github.com:konflux-ci/coverage-dashboard.git",
			expected:  "coverage-dashboard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.Split(tt.remoteURL, "/")
			if len(parts) > 0 {
				repoName := parts[len(parts)-1]
				repoName = strings.TrimSuffix(repoName, ".git")

				if repoName != tt.expected {
					t.Errorf("Parsed repo name = %q, want %q", repoName, tt.expected)
				}
			}
		})
	}
}
