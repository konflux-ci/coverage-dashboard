# coverage-dashboard

Interactive dashboard for tracking Go test coverage across Konflux repositories.

## Overview

This repository contains a GitHub Actions workflow that:
- Periodically collects test coverage from multiple Konflux repositories
- Generates package-level coverage breakdowns
- Publishes an interactive dashboard at https://konflux-ci.dev/coverage-dashboard/

## Running Locally

```bash
# Download the latest coverage data
curl -o coverage.json https://konflux-ci.dev/coverage-dashboard/coverage.json

# Start local web server
python3 -m http.server 8000

# Open http://localhost:8000/index.html in your browser
```

## Repository Configuration

Each repository has its own configuration file in the `repos/` directory:

```yaml
# repos/your-repo.yaml
name: konflux-ci/your-repo
exclude_dirs:
  - vendor/
  - hack/
  - test/
exclude_files:
  - zz_generated.deepcopy.go
```

To manually add a repository:
1. Create a new file `repos/your-repo.yaml`
2. Add ownership to `CODEOWNERS`: `/repos/your-repo.yaml @konflux-ci/your-team`
3. Submit a PR with both files

Or use the automated discovery tool (see below).

## Workflow Triggers

The coverage workflow runs:
- **On push to main**: Automatically after merging PRs
- **Daily at 3:00 AM UTC**: Via cron schedule
- **Manual trigger**: Via GitHub Actions "Run workflow" button
- **On pull requests**: Runs tests but doesn't publish to gh-pages

## Testing Repository Discovery

The `discover-repos` tool can be tested locally before making changes:

### Prerequisites

- Go 1.23 or later
- `GITHUB_READ_TOKEN` (optional): GitHub token for ownership detection
- `GITHUB_WRITE_TOKEN` (required for `--apply`): GitHub token for creating PRs

### Testing in Dry-Run Mode

```bash
# Build the tool
go build -o bin/discover-repos cmd/discover-repos/main.go

# Optional: Set token for better ownership detection
export GITHUB_READ_TOKEN="your-github-app-token"

# Run without creating files or PRs (preview only)
./bin/discover-repos
```

### Testing the Full Flow (Creates PRs)

```bash
# Set both tokens
export GITHUB_READ_TOKEN="your-github-app-token"
export GITHUB_WRITE_TOKEN="your-github-token"

# Run with --apply to create files and PRs
./bin/discover-repos --apply
```

## Contributing

1. Make changes to repository configs in `repos/` directory
2. Update `index.html` to modify the dashboard UI
3. Modify `.github/workflows/coverage.yml` to change workflow behavior
4. Test locally before submitting a PR
5. Include sample `coverage.json` files with pull requests which alter its schema
