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

// Common test data
const (
	adminMaintainCollaboratorsJSON = `[
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
	
	noPermCollaboratorsJSON = `[
		{
			"login": "read-user",
			"permissions": {
				"admin": false,
				"maintain": false
			}
		}
	]`
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
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("NewDetector", func() {
		It("should create a new detector with provided client", func() {
			d := ownership.NewDetector(github.NewClient(nil), "")
			Expect(d).NotTo(BeNil())
		})

		It("should create a detector with nil client", func() {
			d := ownership.NewDetector(nil, "")
			Expect(d).NotTo(BeNil())
		})

		It("should use custom default owner when provided", func() {
			d := ownership.NewDetector(nil, "@custom-org/custom-team")
			Expect(d).NotTo(BeNil())
			owners, err := d.DetectOwners(ctx, "test-org", "test-repo")
			Expect(err).NotTo(HaveOccurred())
			Expect(owners).To(Equal([]string{"@custom-org/custom-team"}))
		})
	})

	Describe("DetectOwners", func() {
		Context("when no GitHub client is configured", func() {
			BeforeEach(func() {
				detector = ownership.NewDetector(nil, "")
			})

			It("should return exactly only the Vanguard owner as fallback", func() {
				owners, err := detector.DetectOwners(ctx, "org", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).To(HaveLen(1))
				Expect(owners).To(Equal([]string{"@konflux-ci/Vanguard"}))
			})
		})

		Context("CODEOWNERS file parsing", func() {			
			It("should check multiple CODEOWNERS paths in order", func() {
				// Verify that the paths list exists and contains expected paths
				Expect(ownership.GetCodeownersPaths()).To(ContainElement(".github/CODEOWNERS"))																																										
				Expect(ownership.GetCodeownersPaths()).To(ContainElement("CODEOWNERS"))
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
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, adminMaintainCollaboratorsJSON)
					case "/repos/no-teams/repo/teams":
						// Mock empty teams response
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, "[]")
					case "/repos/no-teams/repo/collaborators":
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, adminMaintainCollaboratorsJSON)
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
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, noPermCollaboratorsJSON)
					default:
						http.NotFound(w, r)
					}
				}))

				// Configure client to use test server
				baseURL, _ := url.Parse(server.URL + "/")
				client = github.NewClient(nil)
				client.BaseURL = baseURL
				detector = ownership.NewDetector(client, "")
			})

			It("should detect owners from teams when available", func() {
				owners, err := detector.DetectOwners(ctx, "org", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).NotTo(BeEmpty())
				
				// Should contain admin and maintain teams
				Expect(owners).To(ContainElement("@org/admin-team"))
				Expect(owners).To(ContainElement("@org/maintain-team"))
			})

			It("should fallback to collaborators when no teams available", func() {
				owners, err := detector.DetectOwners(ctx, "no-teams", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).NotTo(BeEmpty())
				
				// Should contain admin and maintain users
				Expect(owners).To(ContainElement("@admin-user"))
				Expect(owners).To(ContainElement("@maintain-user"))
			})

			It("should fallback to default when no teams or collaborators have permissions", func() {
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
				detector = ownership.NewDetector(client, "")
			})

			It("should fallback to Vanguard when API calls fail", func() {
				owners, err := detector.DetectOwners(ctx, "org", "repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(owners).To(Equal([]string{"@konflux-ci/Vanguard"}))
			})
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
				detector = ownership.NewDetector(client, "")
			})

			It("should only include teams with admin/maintain permissions", func() {
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
