# coverage-dashboard

Interactive dashboard for tracking Go test coverage across Konflux repositories.

## Overview

This repository contains a GitHub Actions workflow that:
- Automatically discovers and tracks Go repositories in the Konflux organization
- Periodically collects test coverage from multiple Konflux repositories
- Generates package-level coverage breakdowns
- Publishes an interactive dashboard at https://konflux-ci.dev/coverage-dashboard/

## Automated Repository Discovery

Every Monday (02:00 UTC), an automated workflow discovers new Go repositories in the Konflux organization and creates pull requests to add them to the coverage dashboard.

### For Repository Owners: What to Expect

When your repository is discovered, you'll receive a pull request notification asking to add your repository to the coverage dashboard. Here's what happens:

1. **PR Notification**: You'll be automatically added as a reviewer on a PR titled "Add {your-repo} to coverage dashboard"

2. **What's in the PR**:
   - A new configuration file: `repos/{your-repo}.yaml` with common exclude patterns pre-configured
   - An update to `CODEOWNERS` file listing your team/user as the owner of this config

3. **What You Need to Do**:
   - **Review the exclude patterns**: The PR includes default exclusions (vendor/, hack/, test directories, generated files, etc.)
   - **Adjust if needed**: Add or remove patterns based on your repository's structure
   - **Approve and merge**: Once satisfied, approve the PR to start tracking coverage for your repository

4. **After Merging**:
   - Your repository will be included in the next coverage run (daily at 3:00 AM UTC or on-demand)
   - Coverage data will appear on the dashboard at https://konflux-ci.dev/coverage-dashboard/
   - Package-level coverage breakdowns will be available for your repository

## Running Locally

```bash
# Download the latest coverage data
curl -o coverage.json https://konflux-ci.dev/coverage-dashboard/coverage.json

# Start local web server
python3 -m http.server 8000

# Open http://localhost:8000/index.html in your browser
```

## Repository Configuration

Repositories are configured using individual YAML files in the `repos/` directory:

```yaml
# repos/your-repo.yaml
name: konflux-ci/your-repo
exclude_dirs:
  - vendor/
  - hack/
  - test/
  - /fake(/|$)  # Regex patterns are supported
exclude_files:
  - zz_generated.deepcopy.go
  - "*.pb.go"
```

Each repository configuration is owned by the repository's team or maintainers, as defined in the `CODEOWNERS` file.

### Adding Repositories Manually

While the automated discovery process handles new repositories weekly, you can manually add repositories:

1. Create a new YAML file in `repos/` directory named `{repo-name}.yaml`
2. Add the configuration following the format above
3. Update `CODEOWNERS` with the repository owner
4. Submit a pull request

The next coverage workflow run will automatically pick up the new repository.

## Workflow Triggers

The coverage workflow runs:
- **On push to main**: Automatically after merging PRs
- **Daily at 3:00 AM UTC**: Via cron schedule
- **Manual trigger**: Via GitHub Actions "Run workflow" button
- **On pull requests**: Runs tests but doesn't publish to gh-pages

## Contributing

1. **Repository configurations**: Add/modify YAML files in `repos/` directory
2. **Dashboard UI**: Update `index.html` to modify the dashboard appearance
3. **Workflow behavior**: Modify `.github/workflows/coverage.yml` or `.github/workflows/discover-repos.yml`
4. **Testing**: Run tests locally with `go test ./...` before submitting a PR
5. **Schema changes**: Include sample `coverage.json` files with pull requests which alter its schema
