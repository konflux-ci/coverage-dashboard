package ownership

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-github/v66/github"
)

// codeownersPaths defines the list of paths to check for CODEOWNERS files
// Ordered by priority - checked in sequence until one is found
var codeownersPaths = []string{
	".github/CODEOWNERS",
	"CODEOWNERS",
	"docs/CODEOWNERS",
}

// GetCodeownersPaths returns a copy of the CODEOWNERS file paths
// This allows inspection of the paths without allowing external modification
func GetCodeownersPaths() []string {
	paths := make([]string, len(codeownersPaths))
	copy(paths, codeownersPaths)
	return paths
}

// Detector detects repository ownership using multiple strategies
type Detector struct {
	client       *github.Client
	defaultOwner string
}

// NewDetector creates a new ownership detector
// defaultOwner specifies the fallback owner when no owners can be detected through other means
// If empty, defaults to "@konflux-ci/Vanguard"
func NewDetector(client *github.Client, defaultOwner string) *Detector {
	if defaultOwner == "" {
		defaultOwner = "@konflux-ci/Vanguard"
	}
	return &Detector{
		client:       client,
		defaultOwner: defaultOwner,
	}
}

// DetectOwners detects repository owners using a fallback chain:
// 1. CODEOWNERS file (most authoritative)
// 2. GitHub repository teams with admin/maintain permissions
// 3. Individual collaborators with admin/maintain permissions
// 4. Configured default owner (@konflux-ci/Vanguard if empty was provided to constructor)
func (d *Detector) DetectOwners(ctx context.Context, org, repo string) ([]string, error) {
	// Try CODEOWNERS file first
	owners, err := d.detectFromCodeowners(ctx, org, repo)
	if err == nil && len(owners) > 0 {
		return owners, nil
	}

	// Fallback to repository teams
	owners, err = d.detectFromTeams(ctx, org, repo)
	if err == nil && len(owners) > 0 {
		return owners, nil
	}

	// Fallback to individual collaborators
	owners, err = d.detectFromCollaborators(ctx, org, repo)
	if err == nil && len(owners) > 0 {
		return owners, nil
	}

	// Final fallback to configured default owner
	return []string{d.defaultOwner}, nil
}

// detectFromCodeowners attempts to find owners in CODEOWNERS file
// Checks multiple standard locations in priority order
func (d *Detector) detectFromCodeowners(ctx context.Context, org, repo string) ([]string, error) {
	var lastErr error

	// Try each CODEOWNERS path in order
	for _, path := range codeownersPaths {
		content, err := d.fetchFile(ctx, org, repo, path)
		if err != nil {
			lastErr = err
			continue
		}

		// Successfully fetched file, extract owners
		owners := extractOwnersFromCodeowners(content)
		if len(owners) > 0 {
			return owners, nil
		}

		// File exists but has no valid owners
		lastErr = fmt.Errorf("no valid owners found in %s", path)
	}

	// No CODEOWNERS file found or none had valid owners
	if lastErr != nil {
		return nil, fmt.Errorf("failed to detect owners from CODEOWNERS: %w", lastErr)
	}

	return nil, fmt.Errorf("no CODEOWNERS files found")
}

// detectFromTeams queries GitHub API for repository teams
func (d *Detector) detectFromTeams(ctx context.Context, org, repo string) ([]string, error) {
	if d.client == nil {
		return nil, fmt.Errorf("GitHub client not configured")
	}

	teams, _, err := d.client.Repositories.ListTeams(ctx, org, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams for %s/%s: %w", org, repo, err)
	}

	var owners []string
	for _, team := range teams {
		// Only include teams with admin or maintain permissions
		perm := team.GetPermission()
		if perm == "admin" || perm == "maintain" {
			owners = append(owners, fmt.Sprintf("@%s/%s", org, team.GetSlug()))
			if len(owners) >= 3 {
				break
			}
		}
	}

	if len(owners) == 0 {
		return nil, fmt.Errorf("no teams with admin/maintain permissions found")
	}

	return owners, nil
}

// detectFromCollaborators queries GitHub API for individual repository collaborators
func (d *Detector) detectFromCollaborators(ctx context.Context, org, repo string) ([]string, error) {
	if d.client == nil {
		return nil, fmt.Errorf("GitHub client not configured")
	}

	opts := &github.ListCollaboratorsOptions{
		Affiliation: "direct",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	collaborators, _, err := d.client.Repositories.ListCollaborators(ctx, org, repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list collaborators for %s/%s: %w", org, repo, err)
	}

	var owners []string
	for _, collab := range collaborators {
		// Only include collaborators with admin or maintain permissions
		perms := collab.GetPermissions()
		if perms["admin"] || perms["maintain"] {
			owners = append(owners, "@"+collab.GetLogin())
			if len(owners) >= 5 {
				break
			}
		}
	}

	if len(owners) == 0 {
		return nil, fmt.Errorf("no collaborators with admin/maintain permissions found")
	}

	return owners, nil
}

// fetchFile fetches a file from GitHub repository using the GitHub API
func (d *Detector) fetchFile(ctx context.Context, org, repo, path string) (string, error) {
	if d.client == nil {
		return "", fmt.Errorf("GitHub client not configured")
	}

	// GetContents automatically uses the default branch
	fileContent, _, _, err := d.client.Repositories.GetContents(ctx, org, repo, path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %w", path, err)
	}

	if fileContent == nil {
		return "", fmt.Errorf("file %s exists but content is nil", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode content from %s: %w", path, err)
	}

	return content, nil
}

// extractOwnersFromCodeowners parses CODEOWNERS content and extracts owner references
func extractOwnersFromCodeowners(content string) []string {
	ownerPattern := regexp.MustCompile(`@[a-zA-Z0-9_-]+(/[a-zA-Z0-9_-]+)?`)

	matches := ownerPattern.FindAllString(content, -1)

	// Deduplicate and limit to 5
	seen := make(map[string]bool)
	var owners []string
	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			owners = append(owners, match)
			if len(owners) >= 5 {
				break
			}
		}
	}

	return owners
}
