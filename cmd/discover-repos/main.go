package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/konflux-ci/coverage-dashboard/internal/discover"
)

func main() {
	var (
		apply          = flag.Bool("apply", false, "Create configuration files, update CODEOWNERS, and create PRs")
		org            = flag.String("org", "konflux-ci", "GitHub organization to scan")
		reposDir       = flag.String("repos-dir", "repos", "Directory containing repository configurations")
		codeownersFile = flag.String("codeowners", "CODEOWNERS", "Path to CODEOWNERS file")
	)

	flag.Parse()

	config := discover.Config{
		Organization:   *org,
		ReposDir:       *reposDir,
		CodeownersFile: *codeownersFile,
		DryRun:         !*apply,
	}

	ctx := context.Background()
	runner, err := discover.NewRunner(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing: %v\n", err)
		os.Exit(1)
	}

	if err := runner.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
