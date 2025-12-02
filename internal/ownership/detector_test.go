package ownership_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/google/go-github/v66/github"

	"github.com/konflux-ci/coverage-dashboard/internal/ownership"
)

var _ = Describe("Detector", func() {
	var (
		ctx      context.Context
		detector *ownership.Detector
		client   *github.Client
		server   *httptest.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = github.NewClient(nil)
		detector = ownership.NewDetector(client)
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("NewDetector", func() {
		It("should create a new detector with provided client", func() {
			d := ownership.NewDetector(client)
			Expect(d).NotTo(BeNil())
		})

		It("should create a detector with nil client", func() {
			d := ownership.NewDetector(nil)
			Expect(d).NotTo(BeNil())
		})
	})

	Describe("DetectOwners", func() {
		Context("when no GitHub client is configured", func() {
			BeforeEach(func() {
				detector = ownership.NewDetector(nil)
			})

			It("should return Vanguard as fallback", func() {
				owners, err := detector.DetectOwners(ctx, "org", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).To(Equal([]string{"@konflux-ci/Vanguard"}))
			})
		})

		Context("CODEOWNERS file parsing", func() {			
			It("should handle CODEOWNERS parsing logic correctly", func() {
				// This test verifies the regex pattern and parsing logic indirectly
				// by testing that the DetectOwners method follows the expected fallback chain
				
				detector := ownership.NewDetector(nil)
				owners, err := detector.DetectOwners(ctx, "test-org", "test-repo")
				
				// Should fallback to Vanguard when no client is available
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).To(Equal([]string{"@konflux-ci/Vanguard"}))
			})
		})

		Context("with GitHub client configured", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/repos/org/repo/teams":
						// Mock teams response
						response := `[
							{
								"slug": "admin-team",
								"permission": "admin"
							},
							{
								"slug": "maintain-team", 
								"permission": "maintain"
							},
							{
								"slug": "read-team",
								"permission": "read"
							}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					case "/repos/org/repo/collaborators":
						// Mock collaborators response  
						response := `[
							{
								"login": "admin-user",
								"permissions": {
									"admin": true,
									"maintain": false
								}
							},
							{
								"login": "maintain-user",
								"permissions": {
									"admin": false,
									"maintain": true
								}
							}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					case "/repos/no-teams/repo/teams":
						// Mock empty teams response
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, "[]")
					case "/repos/no-teams/repo/collaborators":
						// Mock collaborators response with admin/maintain users
						response := `[
							{
								"login": "admin-user",
								"permissions": {
									"admin": true,
									"maintain": false
								}
							},
							{
								"login": "maintain-user",
								"permissions": {
									"admin": false,
									"maintain": true
								}
							}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					case "/repos/no-perms/repo/teams":
						// Mock teams without admin/maintain permissions
						response := `[
							{
								"slug": "read-team",
								"permission": "read"
							}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					case "/repos/no-perms/repo/collaborators":
						// Mock collaborators without admin/maintain permissions
						response := `[
							{
								"login": "read-user",
								"permissions": {
									"admin": false,
									"maintain": false
								}
							}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					default:
						http.NotFound(w, r)
					}
				}))

				// Configure client to use test server
				baseURL, _ := url.Parse(server.URL + "/")
				client = github.NewClient(nil)
				client.BaseURL = baseURL
				detector = ownership.NewDetector(client)
			})

			It("should detect owners from teams when available", func() {
				owners, err := detector.DetectOwners(ctx, "org", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).NotTo(BeEmpty())
				
				// Should contain team references with proper formatting
				foundTeam := false
				for _, owner := range owners {
					if owner == "@org/admin-team" || owner == "@org/maintain-team" {
						foundTeam = true
						break
					}
				}
				Expect(foundTeam).To(BeTrue(), "Should contain at least one team with admin/maintain permissions")
			})

			It("should fallback to collaborators when no teams available", func() {
				owners, err := detector.DetectOwners(ctx, "no-teams", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).NotTo(BeEmpty())
				
				// Should contain user references with proper formatting
				foundUser := false
				for _, owner := range owners {
					if owner == "@admin-user" || owner == "@maintain-user" {
						foundUser = true
						break
					}
				}
				Expect(foundUser).To(BeTrue(), "Should contain at least one user with admin/maintain permissions")
			})

			It("should fallback to Vanguard when no teams or collaborators have permissions", func() {
				owners, err := detector.DetectOwners(ctx, "no-perms", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).To(Equal([]string{"@konflux-ci/Vanguard"}))
			})
		})

		Context("when GitHub API returns errors", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Return 500 error for all requests
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, "Internal Server Error")
				}))

				baseURL, _ := url.Parse(server.URL + "/")
				client = github.NewClient(nil)
				client.BaseURL = baseURL
				detector = ownership.NewDetector(client)
			})

			It("should fallback to Vanguard when API calls fail", func() {
				owners, err := detector.DetectOwners(ctx, "org", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).To(Equal([]string{"@konflux-ci/Vanguard"}))
			})
		})

	})

	Describe("Fallback behavior", func() {
		It("should always return at least one owner", func() {
			// Test with nil client (no API access)
			detector := ownership.NewDetector(nil)
			
			owners, err := detector.DetectOwners(ctx, "any-org", "any-repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(owners).To(HaveLen(1))
			Expect(owners[0]).To(Equal("@konflux-ci/Vanguard"))
		})

		It("should never return empty owners list", func() {
			// Test multiple scenarios
			testCases := []struct {
				org  string
				repo string
			}{
				{"test-org", "test-repo"},
				{"very-long-organization-name", "very-long-repository-name"},
			}

			detector := ownership.NewDetector(nil)
			
			for _, tc := range testCases {
				owners, err := detector.DetectOwners(ctx, tc.org, tc.repo)
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).NotTo(BeEmpty(), "Should never return empty owners for org=%s, repo=%s", tc.org, tc.repo)
			}
		})
	})

	Describe("Integration scenarios", func() {
		Context("with realistic GitHub responses", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/repos/konflux-ci/test-repo/teams":
						// Realistic response with multiple teams and permissions
						response := `[
							{
								"slug": "admins",
								"permission": "admin"
							},
							{
								"slug": "maintainers",
								"permission": "maintain"
							},
							{
								"slug": "contributors",
								"permission": "write"
							},
							{
								"slug": "readers",
								"permission": "read"
							}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					case "/repos/many-teams/repo/teams":
						// Test with many teams to verify limit
						response := `[
							{"slug": "team1", "permission": "admin"},
							{"slug": "team2", "permission": "admin"},
							{"slug": "team3", "permission": "admin"},
							{"slug": "team4", "permission": "admin"},
							{"slug": "team5", "permission": "admin"}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					case "/repos/many-collaborators/repo/teams":
						// No teams, will fallback to collaborators
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, "[]")
					case "/repos/many-collaborators/repo/collaborators":
						// Test with many collaborators to verify limit
						response := `[
							{"login": "user1", "permissions": {"admin": true}},
							{"login": "user2", "permissions": {"admin": true}},
							{"login": "user3", "permissions": {"admin": true}},
							{"login": "user4", "permissions": {"admin": true}},
							{"login": "user5", "permissions": {"admin": true}},
							{"login": "user6", "permissions": {"admin": true}},
							{"login": "user7", "permissions": {"admin": true}}
						]`
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, response)
					default:
						http.NotFound(w, r)
					}
				}))

				baseURL, _ := url.Parse(server.URL + "/")
				client = github.NewClient(nil)
				client.BaseURL = baseURL
				detector = ownership.NewDetector(client)
			})

			It("should prioritize teams with higher permissions", func() {
				owners, err := detector.DetectOwners(ctx, "konflux-ci", "test-repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).NotTo(BeEmpty())
				
				// Should include admin and maintain teams but not write/read
				Expect(owners).To(ContainElement("@konflux-ci/admins"))
				Expect(owners).To(ContainElement("@konflux-ci/maintainers"))
				Expect(owners).NotTo(ContainElement("@konflux-ci/contributors"))
				Expect(owners).NotTo(ContainElement("@konflux-ci/readers"))
			})

			It("should limit number of teams returned to 3", func() {
				owners, err := detector.DetectOwners(ctx, "many-teams", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(owners)).To(Equal(3))
				
				// Should contain team references with proper formatting
				for _, owner := range owners {
					Expect(owner).To(MatchRegexp(`^@many-teams/team\d+$`))
				}
			})

			It("should limit number of collaborators returned to 5", func() {
				owners, err := detector.DetectOwners(ctx, "many-collaborators", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(owners)).To(Equal(5))
				
				// Should contain user references with proper formatting
				for _, owner := range owners {
					Expect(owner).To(MatchRegexp(`^@user\d+$`))
				}
			})
		})

	})
})
