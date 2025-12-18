package discover_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konflux-ci/coverage-dashboard/internal/discover"
)

var _ = Describe("Runner", func() {
	var (
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "discover-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("NewRunner", func() {
		It("should create a new runner with valid config", func() {
			cfg := discover.Config{
				Organization:   "test-org",
				ReposDir:       filepath.Join(tempDir, "repos"),
				CodeownersFile: filepath.Join(tempDir, "CODEOWNERS"),
				DryRun:         true,
			}

			runner, err := discover.NewRunner(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner).NotTo(BeNil())
		})

		It("should work without GITHUB_READ_TOKEN in dry-run mode", func() {
			os.Unsetenv("GITHUB_READ_TOKEN")
			os.Unsetenv("GITHUB_WRITE_TOKEN")

			cfg := discover.Config{
				Organization:   "test-org",
				ReposDir:       filepath.Join(tempDir, "repos"),
				CodeownersFile: filepath.Join(tempDir, "CODEOWNERS"),
				DryRun:         true,
			}

			runner, err := discover.NewRunner(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner).NotTo(BeNil())
		})

		It("should require GITHUB_WRITE_TOKEN for apply mode", func() {
			os.Unsetenv("GITHUB_WRITE_TOKEN")

			cfg := discover.Config{
				Organization:   "test-org",
				ReposDir:       filepath.Join(tempDir, "repos"),
				CodeownersFile: filepath.Join(tempDir, "CODEOWNERS"),
				DryRun:         false,
			}

			runner, err := discover.NewRunner(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GITHUB_WRITE_TOKEN is required"))
			Expect(runner).To(BeNil())
		})

		Context("with environment variables", func() {
			BeforeEach(func() {
				os.Setenv("GITHUB_READ_TOKEN", "test-read-token")
				os.Setenv("GITHUB_WRITE_TOKEN", "test-write-token")
			})

			AfterEach(func() {
				os.Unsetenv("GITHUB_READ_TOKEN")
				os.Unsetenv("GITHUB_WRITE_TOKEN")
			})

			It("should create runner with tokens for apply mode", func() {
				cfg := discover.Config{
					Organization:   "test-org",
					ReposDir:       filepath.Join(tempDir, "repos"),
					CodeownersFile: filepath.Join(tempDir, "CODEOWNERS"),
					DryRun:         false,
				}

				runner, err := discover.NewRunner(cfg)
				Expect(err).NotTo(HaveOccurred())
				Expect(runner).NotTo(BeNil())
			})
		})
	})

	Describe("Config validation", func() {
		It("should accept custom organization", func() {
			cfg := discover.Config{
				Organization:   "custom-org",
				ReposDir:       filepath.Join(tempDir, "repos"),
				CodeownersFile: filepath.Join(tempDir, "CODEOWNERS"),
				DryRun:         true,
			}

			runner, err := discover.NewRunner(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner).NotTo(BeNil())
		})

		It("should accept custom repos directory", func() {
			cfg := discover.Config{
				Organization:   "test-org",
				ReposDir:       filepath.Join(tempDir, "custom-repos"),
				CodeownersFile: filepath.Join(tempDir, "CODEOWNERS"),
				DryRun:         true,
			}

			runner, err := discover.NewRunner(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner).NotTo(BeNil())
		})

		It("should accept custom CODEOWNERS path", func() {
			cfg := discover.Config{
				Organization:   "test-org",
				ReposDir:       filepath.Join(tempDir, "repos"),
				CodeownersFile: filepath.Join(tempDir, ".github", "CODEOWNERS"),
				DryRun:         true,
			}

			runner, err := discover.NewRunner(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(runner).NotTo(BeNil())
		})
	})
})
