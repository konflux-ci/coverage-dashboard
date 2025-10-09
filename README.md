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

Edit `repos.yaml` to add or modify repositories:

```yaml
repositories:
  - name: konflux-ci/your-repo
    exclude_dirs:
      - vendor/
      - hack/
      - test/
    exclude_files:
      - zz_generated.deepcopy.go
```

## Workflow Triggers

The coverage workflow runs:
- **On push to main**: Automatically after merging PRs
- **Daily at 3:00 AM UTC**: Via cron schedule
- **Manual trigger**: Via GitHub Actions "Run workflow" button
- **On pull requests**: Runs tests but doesn't publish to gh-pages

## Contributing

1. Make changes to `repos.yaml` to add/modify repositories
2. Update `index.html` to modify the dashboard UI
3. Modify `.github/workflows/coverage.yml` to change workflow behavior
4. Test locally before submitting a PR
5. Include sample `coverage.json` files with pull requests which alter its schema
