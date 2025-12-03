package config_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/konflux-ci/coverage-dashboard/internal/config"
)

var _ = Describe("Config", func() {
	Describe("RepositoryConfig marshaling", func() {
		It("should marshal to YAML correctly", func() {
			cfg := config.RepositoryConfig{
				Name: "konflux-ci/caching",
				ExcludeDirs: []string{
					"vendor/",
					"hack/",
					"/fake(/|$)",
				},
				ExcludeFiles: []string{
					"zz_generated.deepcopy.go",
					"openapi_generated.go",
				},
			}

			data, err := yaml.Marshal(cfg)
			Expect(err).NotTo(HaveOccurred())

			result := string(data)
			Expect(result).To(ContainSubstring("name: konflux-ci/caching"))
			Expect(result).To(ContainSubstring("exclude_dirs:"))
			Expect(result).To(ContainSubstring("- vendor/"))
			Expect(result).To(ContainSubstring("- hack/"))
			Expect(result).To(ContainSubstring("- /fake(/|$)"))
			Expect(result).To(ContainSubstring("exclude_files:"))
			Expect(result).To(ContainSubstring("- zz_generated.deepcopy.go"))
			Expect(result).To(ContainSubstring("- openapi_generated.go"))
		})

		It("should unmarshal from YAML correctly", func() {
			yamlContent := `name: konflux-ci/caching
exclude_dirs:
  - vendor/
  - hack/
  - /fake(/|$)
exclude_files:
  - zz_generated.deepcopy.go
  - openapi_generated.go
`
			var cfg config.RepositoryConfig
			err := yaml.Unmarshal([]byte(yamlContent), &cfg)
			Expect(err).NotTo(HaveOccurred())

			Expect(cfg.Name).To(Equal("konflux-ci/caching"))
			Expect(cfg.ExcludeDirs).To(HaveLen(3))
			Expect(cfg.ExcludeFiles).To(HaveLen(2))
		})
	})

	Describe("Writer", func() {
		var (
			tempDir        string
			reposDir       string
			codeownersFile string
			writer         *config.Writer
		)

		BeforeEach(func() {
			tempDir = GinkgoT().TempDir()
			reposDir = filepath.Join(tempDir, "repos")
			codeownersFile = filepath.Join(tempDir, "CODEOWNERS")
			writer = config.NewWriter(reposDir, codeownersFile)
		})

		Describe("Write", func() {
			It("should write config file and update CODEOWNERS", func() {
				cfg := config.RepositoryConfig{
					Name: "konflux-ci/test-repo",
					ExcludeDirs: []string{
						"vendor/",
						"hack/",
					},
					ExcludeFiles: []string{
						"zz_generated.deepcopy.go",
					},
					Owners: []string{"@konflux-ci/test-team"},
				}

				err := writer.Write(cfg, false)
				Expect(err).NotTo(HaveOccurred())

				// Verify config file was created
				configPath := filepath.Join(reposDir, "test-repo.yaml")
				Expect(configPath).To(BeAnExistingFile())

				// Load and verify config content
				loadedCfg, err := config.LoadRepositoryConfig(reposDir, "test-repo.yaml")
				Expect(err).NotTo(HaveOccurred())
				Expect(loadedCfg.Name).To(Equal(cfg.Name))

				// Verify CODEOWNERS was updated
				codeownersContent, err := os.ReadFile(codeownersFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(codeownersContent)).To(ContainSubstring("/repos/test-repo.yaml @konflux-ci/test-team"))
			})

			It("should write to discovered-repos in dry-run mode", func() {
				cfg := config.RepositoryConfig{
					Name: "konflux-ci/test-repo",
					ExcludeDirs: []string{
						"vendor/",
					},
					Owners: []string{"@konflux-ci/test-team"},
				}

				err := writer.Write(cfg, true)
				Expect(err).NotTo(HaveOccurred())

				// Verify config file was created in discovered-repos
				dryRunPath := filepath.Join(tempDir, "discovered-repos", "test-repo.yaml")
				Expect(dryRunPath).To(BeAnExistingFile())

				// Verify CODEOWNERS was NOT updated in dry-run mode
				Expect(codeownersFile).NotTo(BeAnExistingFile())

				// Verify config was NOT created in repos directory
				configPath := filepath.Join(reposDir, "test-repo.yaml")
				Expect(configPath).NotTo(BeAnExistingFile())
			})

			It("should fail when no owners are specified", func() {
				cfg := config.RepositoryConfig{
					Name: "org/repo",
				}

				err := writer.Write(cfg, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no owners specified"))
			})
		})

		Describe("CODEOWNERS updates", func() {
			It("should be idempotent when writing same config multiple times", func() {
				cfg := config.RepositoryConfig{
					Name:   "konflux-ci/test-repo",
					Owners: []string{"@konflux-ci/test-team"},
				}

				// Write first time
				err := writer.Write(cfg, false)
				Expect(err).NotTo(HaveOccurred())

				content1, err := os.ReadFile(codeownersFile)
				Expect(err).NotTo(HaveOccurred())

				// Write second time
				err = writer.Write(cfg, false)
				Expect(err).NotTo(HaveOccurred())

				content2, err := os.ReadFile(codeownersFile)
				Expect(err).NotTo(HaveOccurred())

				// Content should be identical (no duplicates)
				Expect(string(content1)).To(Equal(string(content2)))

				// Verify no duplicate entries
				lines := strings.Split(string(content2), "\n")
				entryCount := 0
				for _, line := range lines {
					if strings.HasPrefix(line, "/repos/test-repo.yaml") {
						entryCount++
					}
				}
				Expect(entryCount).To(Equal(1))
			})
		})

		Describe("Input validation", func() {
			It("should accept valid repository names in org/repo format", func() {
				validNames := []string{"konflux-ci/caching", "my-org/my-repo", "my_org/my_repo"}
				for _, name := range validNames {
					cfg := config.RepositoryConfig{Name: name, Owners: []string{"@test-team"}}
					err := writer.Write(cfg, false)
					Expect(err).NotTo(HaveOccurred(), "should accept valid name %q", name)
				}
			})

			It("should reject invalid repository names", func() {
				invalidNames := []string{"../etc/passwd", "org\\repo", "org/repo/extra", "org.name/repo", "", "no-slash"}
				for _, name := range invalidNames {
					cfg := config.RepositoryConfig{Name: name, Owners: []string{"@test-team"}}
					err := writer.Write(cfg, false)
					Expect(err).To(HaveOccurred(), "should reject invalid name %q", name)
				}
			})
		})

		Describe("Owner normalization", func() {
			It("should normalize and deduplicate owners", func() {
				cfg := config.RepositoryConfig{
					Name:   "konflux-ci/test-repo",
					Owners: []string{"team1", "@team2", "  team1  "}, // Missing @, has @, duplicate with whitespace
				}

				err := writer.Write(cfg, false)
				Expect(err).NotTo(HaveOccurred())

				content, err := os.ReadFile(codeownersFile)
				Expect(err).NotTo(HaveOccurred())

				// Should normalize to: @team1 @team2 (deduplicated, @ prefix added)
				Expect(string(content)).To(ContainSubstring("@team1 @team2"))
			})
		})
	})
})
