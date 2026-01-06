package pr

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helper Functions", func() {
	Describe("extractRepoName", func() {
		It("should extract repo name from org/repo format", func() {
			result := extractRepoName("konflux-ci/caching")
			Expect(result).To(Equal("caching"))
		})

		It("should return full name when no org prefix", func() {
			result := extractRepoName("standalone-repo")
			Expect(result).To(Equal("standalone-repo"))
		})
	})

	Describe("extractReviewers", func() {
		It("should extract reviewers from mixed teams and users with @ prefix", func() {
			owners := []string{"@konflux-ci/Vanguard", "@user1"}
			result := extractReviewers(owners)
			Expect(result).To(Equal([]string{"konflux-ci/Vanguard", "user1"}))
		})

		It("should return nil for empty list", func() {
			owners := []string{}
			result := extractReviewers(owners)
			Expect(result).To(BeNil())
		})
	})

	Describe("formatList", func() {
		It("should format multiple items", func() {
			items := []string{"vendor/", "hack/"}
			result := formatList(items, "None")
			Expect(result).To(Equal("- `vendor/`\n- `hack/`"))
		})

		It("should return emptyText for empty list", func() {
			items := []string{}
			result := formatList(items, "None")
			Expect(result).To(Equal("None"))
		})
	})
})
