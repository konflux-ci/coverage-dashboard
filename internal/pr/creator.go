package pr

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v66/github"
	"github.com/konflux-ci/coverage-dashboard/internal/config"
)

const prBodyTemplate = `## Add Coverage Dashboard Tracking

This PR adds your repository to the **Konflux Coverage Dashboard** at:
https://konflux-ci.dev/coverage-dashboard/

### What is the Coverage Dashboard?

The coverage dashboard automatically collects and displays test coverage metrics for all Konflux Go repositories in one centralized location. It provides:
- Unified view of coverage across all repositories and teams
- Package-level coverage breakdown for each repository
- Detailed HTML coverage reports for deep-dive analysis

### What This PR Does

- ✅ Adds configuration for %s to the coverage dashboard
- ✅ Sets up ownership mapping so your team can manage future changes
- ✅ Enables automatic coverage report generation from your test suite

### After Merge

Your repository will automatically:
1. Appear on the dashboard within 24 hours (next scheduled run)
2. Have coverage metrics updated with each dashboard run
3. Generate detailed HTML coverage reports accessible from the dashboard

### Review Checklist

- [ ] Verify exclude patterns are appropriate for your repository structure
- [ ] Confirm ownership assignment includes the right team members
- [ ] Repository has Go tests that will generate coverage data`

const commitMsgTemplate = `chore: add coverage tracking for %s

Add configuration for %s to the Konflux coverage dashboard.
This enables automatic test coverage tracking and reporting for the repository.`

// Creator creates pull requests for repository configurations
type Creator struct {
	client      *github.Client
	workDir     string
	org         string
	baseBranch  string
	currentRepo string
}

// NewCreator creates a new PR creator
func NewCreator(client *github.Client, workDir, org, repo, baseBranch string) *Creator {
	return &Creator{
		client:      client,
		workDir:     workDir,
		org:         org,
		baseBranch:  baseBranch,
		currentRepo: repo,
	}
}

// CreatePullRequest creates a pull request for a repository configuration
func (c *Creator) CreatePullRequest(ctx context.Context, cfg config.RepositoryConfig) error {
	repoName := extractRepoName(cfg.Name)
	branchName := fmt.Sprintf("add-repo/%s", repoName)

	// 1. Create branch
	if err := c.createBranch(ctx, branchName); err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	// 2. Commit changes (files were already written by config.Writer)
	configFile := filepath.Join("repos", repoName+".yaml")
	if err := c.commitChanges(ctx, configFile, cfg.Name); err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// 3. Push branch
	if _, err := RunGitCommand(ctx, c.workDir, "push", "-u", "origin", branchName, "--force"); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	// 4. Create pull request
	_, err := c.createGitHubPR(ctx, branchName, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("PR already exists")
		}
		return fmt.Errorf("GitHub API error: %w", err)
	}

	// 5. Return to base branch for next iteration
	if _, err := RunGitCommand(ctx, c.workDir, "checkout", c.baseBranch); err != nil {
		fmt.Printf("    ⚠️  Warning: failed to checkout %s: %v\n", c.baseBranch, err)
	}

	return nil
}

func (c *Creator) createBranch(ctx context.Context, branchName string) error {
	// Delete branch if it exists
	if c.branchExists(ctx, branchName) {
		if _, err := RunGitCommand(ctx, c.workDir, "branch", "-D", branchName); err != nil {
			return err
		}
	}

	// Try to fetch the base branch from origin
	// If this fails, we'll try to use the local branch if it exists
	fetchSucceeded := false
	if _, err := RunGitCommand(ctx, c.workDir, "fetch", "origin", c.baseBranch); err == nil {
		fetchSucceeded = true
	}

	// Check if base branch exists locally
	baseExistsLocally := c.branchExists(ctx, c.baseBranch)

	if !baseExistsLocally && !fetchSucceeded {
		// Can't proceed without either local branch or successful fetch
		return fmt.Errorf("base branch %s does not exist locally and fetch from origin failed", c.baseBranch)
	}

	if fetchSucceeded {
		// Create/reset local branch to match FETCH_HEAD (latest from remote)
		// -B creates the branch if it doesn't exist, or resets it if it does
		if _, err := RunGitCommand(ctx, c.workDir, "checkout", "-B", c.baseBranch, "FETCH_HEAD"); err != nil {
			return fmt.Errorf("failed to setup base branch %s from remote: %w", c.baseBranch, err)
		}
	} else {
		// Fetch failed but local branch exists - use local copy
		if _, err := RunGitCommand(ctx, c.workDir, "checkout", c.baseBranch); err != nil {
			return fmt.Errorf("failed to checkout base branch %s: %w", c.baseBranch, err)
		}
		fmt.Printf("    ⚠️  Warning: using local %s branch (fetch failed)\n", c.baseBranch)
	}

	// Create and checkout new branch
	_, err := RunGitCommand(ctx, c.workDir, "checkout", "-b", branchName)
	return err
}

func (c *Creator) commitChanges(ctx context.Context, configFile, repoFullName string) error {
	// Configure git user identity
	if _, err := RunGitCommand(ctx, c.workDir, "config", "user.name", "github-actions[bot]"); err != nil {
		return fmt.Errorf("failed to set git user.name: %w", err)
	}
	if _, err := RunGitCommand(ctx, c.workDir, "config", "user.email", "github-actions[bot]@users.noreply.github.com"); err != nil {
		return fmt.Errorf("failed to set git user.email: %w", err)
	}

	// Stage files
	if _, err := RunGitCommand(ctx, c.workDir, "add", configFile, "CODEOWNERS"); err != nil {
		return err
	}

	// Create commit message
	commitMsg := fmt.Sprintf(commitMsgTemplate, repoFullName, repoFullName)

	_, err := RunGitCommand(ctx, c.workDir, "commit", "-m", commitMsg)
	return err
}

func (c *Creator) createGitHubPR(ctx context.Context, branchName string, cfg config.RepositoryConfig) (string, error) {
	repoName := extractRepoName(cfg.Name)

	title := fmt.Sprintf("chore: add coverage tracking for %s", repoName)
	body := c.generatePRBody(cfg)

	newPR := &github.NewPullRequest{
		Title:               github.String(title),
		Head:                github.String(branchName),
		Base:                github.String(c.baseBranch),
		Body:                github.String(body),
		MaintainerCanModify: github.Bool(true),
	}

	pr, _, err := c.client.PullRequests.Create(ctx, c.org, c.currentRepo, newPR)
	if err != nil {
		return "", err
	}

	// Add reviewers
	if err := c.addReviewers(ctx, pr.GetNumber(), cfg.Owners); err != nil {
		fmt.Printf("    ⚠️  Warning: failed to add reviewers: %v\n", err)
	}

	return pr.GetHTMLURL(), nil
}

func (c *Creator) addReviewers(ctx context.Context, prNumber int, owners []string) error {
	reviewers := extractReviewers(owners)
	if len(reviewers) == 0 {
		return nil
	}

	// Separate individual reviewers from teams
	var users []string
	var teams []string

	for _, reviewer := range reviewers {
		if strings.Contains(reviewer, "/") {
			parts := strings.Split(reviewer, "/")
			if len(parts) == 2 {
				teams = append(teams, parts[1])
			}
		} else {
			users = append(users, reviewer)
		}
	}

	reviewersRequest := github.ReviewersRequest{
		Reviewers:     users,
		TeamReviewers: teams,
	}

	_, _, err := c.client.PullRequests.RequestReviewers(ctx, c.org, c.currentRepo, prNumber, reviewersRequest)
	return err
}

func (c *Creator) generatePRBody(cfg config.RepositoryConfig) string {
	return fmt.Sprintf(prBodyTemplate, "`"+cfg.Name+"`")
}

// Helper functions

func extractRepoName(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) == 2 {
		return filepath.Base(parts[1])
	}
	return filepath.Base(fullName)
}

func extractReviewers(owners []string) []string {
	var reviewers []string
	for _, owner := range owners {
		reviewer := strings.TrimPrefix(owner, "@")
		if reviewer != "" {
			reviewers = append(reviewers, reviewer)
		}
	}
	return reviewers
}

func formatList(items []string, emptyText string) string {
	if len(items) == 0 {
		return emptyText
	}
	var formatted []string
	for _, item := range items {
		formatted = append(formatted, "- `"+item+"`")
	}
	return strings.Join(formatted, "\n")
}

// Git helper functions

// RunGitCommand executes a git command and returns the output and error
func RunGitCommand(ctx context.Context, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output), nil
}

func (c *Creator) branchExists(ctx context.Context, branchName string) bool {
	_, err := RunGitCommand(ctx, c.workDir, "rev-parse", "--verify", branchName)
	return err == nil
}
