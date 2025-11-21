package ownership

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/go-github/v66/github"
)

// Detector detects repository ownership using multiple strategies
type Detector struct {
	client *github.Client
}

// NewDetector creates a new ownership detector
func NewDetector(client *github.Client) *Detector {
	return &Detector{client: client}
}

// DetectOwners detects repository owners using a fallback chain:
// 1. CODEOWNERS file (most authoritative)
// 2. GitHub repository teams with admin/maintain permissions
// 3. Individual collaborators with admin/maintain permissions
// 4. Default fallback
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

	// Final fallback
	return []string{"@konflux-ci/Vanguard"}, nil
}

// detectFromCodeowners attempts to find owners in CODEOWNERS file
func (d *Detector) detectFromCodeowners(ctx context.Context, org, repo string) ([]string, error) {
	// Try .github/CODEOWNERS first
	content, err := d.fetchFile(ctx, org, repo, ".github/CODEOWNERS")
	if err != nil {
		// Try root CODEOWNERS
		content, err = d.fetchFile(ctx, org, repo, "CODEOWNERS")
		if err != nil {
			return nil, fmt.Errorf("CODEOWNERS not found")
		}
	}

	return extractOwnersFromCodeowners(content), nil
}

// detectFromTeams queries GitHub API for repository teams
func (d *Detector) detectFromTeams(ctx context.Context, org, repo string) ([]string, error) {
	teams, _, err := d.client.Repositories.ListTeams(ctx, org, repo, nil)
	if err != nil {
		return nil, err
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
	opts := &github.ListCollaboratorsOptions{
		Affiliation: "direct",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	collaborators, _, err := d.client.Repositories.ListCollaborators(ctx, org, repo, opts)
	if err != nil {
		return nil, err
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

// fetchFile fetches a file from GitHub repository
func (d *Detector) fetchFile(ctx context.Context, org, repo, path string) (string, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", org, repo, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("file not found: %s", path)
	}

	scanner := bufio.NewScanner(resp.Body)
	var content strings.Builder
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	return content.String(), scanner.Err()
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
