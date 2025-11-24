package discover

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v66/github"
	"github.com/konflux-ci/coverage-dashboard/internal/config"
	"github.com/konflux-ci/coverage-dashboard/internal/ownership"
	"github.com/konflux-ci/coverage-dashboard/internal/pr"
	"golang.org/x/oauth2"
)

// Config holds the configuration for the discovery process
type Config struct {
	Organization   string
	ReposDir       string
	CodeownersFile string
	DryRun         bool
}

// Runner orchestrates the repository discovery process
type Runner struct {
	config         Config
	githubClient   *github.Client // For general API calls and ownership detection
	writeClient    *github.Client // For PR creation
	ownerDetector  *ownership.Detector
	configWriter   *config.Writer
	existingRepos  map[string]bool
}

// NewRunner creates a new Runner instance
func NewRunner(cfg Config) (*Runner, error) {
	// Create read client for ownership detection (teams/collaborators)
	readToken := os.Getenv("GITHUB_READ_TOKEN")

	var readClient *github.Client
	if readToken != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: readToken})
		tc := oauth2.NewClient(context.Background(), ts)
		readClient = github.NewClient(tc)
	} else {
		readClient = github.NewClient(nil)
		fmt.Println("⚠️  Warning: GITHUB_READ_TOKEN not set, using unauthenticated API calls")
		fmt.Println("   Ownership detection will be limited to CODEOWNERS files only")
		fmt.Println()
	}

	// Create write client for PR creation
	writeToken := os.Getenv("GITHUB_WRITE_TOKEN")

	var writeClient *github.Client
	if writeToken != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: writeToken})
		tc := oauth2.NewClient(context.Background(), ts)
		writeClient = github.NewClient(tc)
	} else {
		if !cfg.DryRun {
			return nil, fmt.Errorf("GITHUB_WRITE_TOKEN is required for --apply (needed for creating PRs)")
		}
		// For dry-run, write client is not needed
		writeClient = github.NewClient(nil)
	}

	return &Runner{
		config:        cfg,
		githubClient:  readClient,
		writeClient:   writeClient,
		ownerDetector: ownership.NewDetector(readClient),
		configWriter:  config.NewWriter(cfg.ReposDir, cfg.CodeownersFile),
	}, nil
}

// Run executes the discovery process
func (r *Runner) Run(ctx context.Context) error {
	fmt.Println("🔍 Konflux-CI Repository Auto-Discovery")
	fmt.Println("========================================")

	if r.config.DryRun {
		fmt.Println("📋 Mode: DRY RUN (preview only)")
		fmt.Println("   Use --apply to create files and PRs")
	} else {
		fmt.Println("🚀 Mode: APPLY (will create files and PRs)")
	}
	fmt.Println()

	// Step 1: Fetch all Go repositories
	fmt.Println("→ Fetching Go repositories from", r.config.Organization, "organization...")
	repos, err := r.fetchGoRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch repositories: %w", err)
	}
	fmt.Printf("  ✅ Found %d Go repositories\n", len(repos))
	fmt.Println()

	// Step 2: Load currently tracked repositories
	fmt.Println("→ Checking currently tracked repositories...")
	if err := r.loadExistingRepos(); err != nil {
		return fmt.Errorf("failed to load existing repos: %w", err)
	}
	fmt.Printf("  ✅ Currently tracking %d repositories\n", len(r.existingRepos))
	fmt.Println()

	// Step 3: Find new repositories
	fmt.Println("→ Identifying new repositories to add...")
	newRepos := r.filterNewRepositories(repos)
	if len(newRepos) == 0 {
		fmt.Println("  ✅ No new repositories found. All Go repos are already tracked!")
		fmt.Println()
		fmt.Println("=========================================")
		fmt.Println("Summary: Up to date!")
		fmt.Println("=========================================")
		return nil
	}
	fmt.Printf("  ✅ Found %d new repositories to add\n", len(newRepos))
	fmt.Println()

	// Step 4: Analyze each repository
	fmt.Printf("Analyzing %d new repositories...\n", len(newRepos))
	fmt.Println()

	var repoConfigs []config.RepositoryConfig
	for i, repo := range newRepos {
		fmt.Printf("📦 [%d/%d] %s\n", i+1, len(newRepos), repo.GetName())

		// Skip if PR already exists (in --apply mode)
		if !r.config.DryRun {
			if r.prAlreadyExists(ctx, repo.GetName()) {
				fmt.Printf("  ⏭️  Skipped: PR already exists\n")
				continue
			}
		}

		cfg, err := r.analyzeRepository(ctx, repo)
		if err != nil {
			fmt.Printf("  ⚠️  Skipped: %v\n", err)
			continue
		}

		repoConfigs = append(repoConfigs, cfg)
	}
	fmt.Println()

	// Step 5: Write configuration files
	if err := r.writeConfigurations(ctx, repoConfigs); err != nil {
		return fmt.Errorf("failed to write configurations: %w", err)
	}

	// Step 6: Create PRs if applying changes
	if !r.config.DryRun {
		if err := r.createPullRequests(ctx, repoConfigs); err != nil {
			return fmt.Errorf("failed to create pull requests: %w", err)
		}
	}

	// Print summary
	r.printSummary(len(repos), len(newRepos), len(repoConfigs))

	return nil
}

func (r *Runner) fetchGoRepositories(ctx context.Context) ([]*github.Repository, error) {
	opts := &github.RepositoryListByOrgOptions{
		Type: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allRepos []*github.Repository
	for {
		repos, resp, err := r.githubClient.Repositories.ListByOrg(ctx, r.config.Organization, opts)
		if err != nil {
			return nil, err
		}

		// Filter for Go repositories that are not archived
		for _, repo := range repos {
			if repo.GetLanguage() == "Go" && !repo.GetArchived() {
				allRepos = append(allRepos, repo)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (r *Runner) loadExistingRepos() error {
	r.existingRepos = make(map[string]bool)

	entries, err := os.ReadDir(r.config.ReposDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		cfg, err := config.LoadRepositoryConfig(r.config.ReposDir, entry.Name())
		if err != nil {
			fmt.Printf("  ⚠️  Warning: failed to parse %s: %v\n", entry.Name(), err)
			continue
		}

		r.existingRepos[cfg.Name] = true
	}

	return nil
}

func (r *Runner) filterNewRepositories(repos []*github.Repository) []*github.Repository {
	var newRepos []*github.Repository
	for _, repo := range repos {
		fullName := fmt.Sprintf("%s/%s", r.config.Organization, repo.GetName())
		if !r.existingRepos[fullName] {
			newRepos = append(newRepos, repo)
		}
	}
	return newRepos
}

func (r *Runner) analyzeRepository(ctx context.Context, repo *github.Repository) (config.RepositoryConfig, error) {
	fullName := fmt.Sprintf("%s/%s", r.config.Organization, repo.GetName())

	// Detect ownership
	owners, err := r.ownerDetector.DetectOwners(ctx, r.config.Organization, repo.GetName())
	if err != nil {
		owners = []string{"@konflux-ci/Vanguard"}
		fmt.Printf("  👥 Owners: %v (default - %s)\n", owners, err.Error())
	} else {
		fmt.Printf("  👥 Owners: %v\n", owners)
	}

	// Apply common exclude patterns - repository owners can adjust in PR
	excludeDirs := []string{
		"vendor/",
		".github/",
		".tekton/",
		"hack/",
		"proto/",
		"test/",
		"tests/",
		"integration-tests/",
		"/fake(/|$)",
		"/mock(s)?(/|$)",
		"/e2e(-tests)?(/|$)",
		"docs/",
	}

	excludeFiles := []string{
		"zz_generated.deepcopy.go",
		"openapi_generated.go",
		"*.pb.go",
		"mock_*.go",
		"*_mock.go",
	}

	return config.RepositoryConfig{
		Name:         fullName,
		ExcludeDirs:  excludeDirs,
		ExcludeFiles: excludeFiles,
		Owners:       owners,
	}, nil
}

func (r *Runner) writeConfigurations(ctx context.Context, configs []config.RepositoryConfig) error {
	if len(configs) == 0 {
		fmt.Println("📝 No configurations to generate")
		return nil
	}

	fmt.Printf("📝 Generating %d configuration files...\n", len(configs))

	for _, cfg := range configs {
		if err := r.configWriter.Write(cfg, r.config.DryRun); err != nil {
			return err
		}
	}

	if r.config.DryRun {
		fmt.Printf("  ✅ Created %d files in discovered-repos/ directory\n", len(configs))
	} else {
		fmt.Printf("  ✅ Created %d files and updated CODEOWNERS\n", len(configs))
	}
	fmt.Println()

	return nil
}

func (r *Runner) createPullRequests(ctx context.Context, configs []config.RepositoryConfig) error {
	if len(configs) == 0 {
		return nil
	}

	fmt.Printf("🔀 Creating %d pull requests...\n", len(configs))

	// Get current working directory
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Extract repository name from git remote
	currentRepo, err := r.getCurrentRepoName(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current repository name: %w", err)
	}

	// Determine base branch (main or master)
	baseBranch := "main"

	// Use writeClient for PR creation (may have different permissions than readClient)
	prCreator := pr.NewCreator(r.writeClient, workDir, r.config.Organization, currentRepo, baseBranch)

	successCount := 0
	for i, cfg := range configs {
		fmt.Printf("  [%d/%d] %s... ", i+1, len(configs), extractRepoNameFromConfig(cfg.Name))
		if err := prCreator.CreatePullRequest(ctx, cfg); err != nil {
			fmt.Printf("failed (%v)\n", err)
			continue
		}
		successCount++
	}

	if successCount < len(configs) {
		fmt.Printf("  ⚠️  Created %d/%d pull requests\n", successCount, len(configs))
	} else {
		fmt.Printf("  🎉 All %d pull requests created successfully!\n", successCount)
	}
	fmt.Println()

	return nil
}

func (r *Runner) getCurrentRepoName(ctx context.Context) (string, error) {
	workDir, _ := os.Getwd()

	remoteURL, err := getGitRemoteURL(ctx, workDir)
	if err != nil {
		// Default to "coverage-dashboard" if we can't determine
		return "coverage-dashboard", nil
	}

	// Parse repository name from URL
	// Examples:
	//   https://github.com/konflux-ci/coverage-dashboard.git -> coverage-dashboard
	//   git@github.com:konflux-ci/coverage-dashboard.git -> coverage-dashboard
	parts := strings.Split(remoteURL, "/")
	if len(parts) > 0 {
		repoName := parts[len(parts)-1]
		repoName = strings.TrimSuffix(repoName, ".git")
		return repoName, nil
	}

	return "coverage-dashboard", nil
}

func (r *Runner) printSummary(totalRepos, newRepos, created int) {
	fmt.Println("=========================================")
	fmt.Println("Discovery Summary")
	fmt.Println("=========================================")
	fmt.Println()
	fmt.Println("📊 Statistics:")
	fmt.Printf("  • Total Go repositories: %d\n", totalRepos)
	fmt.Printf("  • Currently tracked: %d\n", len(r.existingRepos))
	fmt.Printf("  • New repositories: %d\n", newRepos)
	fmt.Printf("  • Configurations created: %d\n", created)
	fmt.Println()

	if r.config.DryRun {
		fmt.Println("💡 Next Steps:")
		fmt.Println("  • Review files in discovered-repos/")
		fmt.Printf("  • Run: go run cmd/discover-repos/main.go --apply\n")
		fmt.Println("  • Set GITHUB_TOKEN if not already set")
	} else {
		fmt.Println("🎉 Success! Repository configurations created.")
		fmt.Println("💡 Next Steps:")
		fmt.Println("  • Repository owners will receive PR notifications")
		fmt.Println("  • PRs can be reviewed and approved by teams")
		fmt.Println("  • Dashboard will update within 24 hours")
	}

	fmt.Println("\n✨ Discovery Complete!")
}

// Helper functions

func extractRepoNameFromConfig(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return fullName
}

// prAlreadyExists checks if a PR already exists for the given repository
func (r *Runner) prAlreadyExists(ctx context.Context, repoName string) bool {
	// Get the current repository name
	currentRepo, err := r.getCurrentRepoName(ctx)
	if err != nil {
		return false
	}

	// Branch name format matches pr/creator.go
	branchName := fmt.Sprintf("add-repo/%s", repoName)

	// Check if PR exists with this branch as head
	opts := &github.PullRequestListOptions{
		State: "open",
		Head:  fmt.Sprintf("%s:%s", r.config.Organization, branchName),
		Base:  "main",
	}

	prs, _, err := r.writeClient.PullRequests.List(ctx, r.config.Organization, currentRepo, opts)
	if err != nil {
		return false
	}

	return len(prs) > 0
}

func getGitRemoteURL(ctx context.Context, workDir string) (string, error) {
	output, err := pr.RunGitCommand(ctx, workDir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}
	return strings.TrimSpace(output), nil
}
