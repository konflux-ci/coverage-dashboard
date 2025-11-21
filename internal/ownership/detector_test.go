package ownership

import (
	"reflect"
	"testing"
)

func TestExtractOwnersFromCodeowners(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "multiple teams",
			content: `# CODEOWNERS
/repos/caching.yaml @konflux-ci/Vanguard
/repos/build-service.yaml @konflux-ci/build-maintainers
`,
			expected: []string{"@konflux-ci/Vanguard", "@konflux-ci/build-maintainers"},
		},
		{
			name: "individual users",
			content: `# CODEOWNERS
/repos/cli.yaml @sbose78 @ralphbean
`,
			expected: []string{"@sbose78", "@ralphbean"},
		},
		{
			name: "mixed teams and users",
			content: `# CODEOWNERS
* @konflux-ci/Vanguard
/repos/integration-service.yaml @dirgim @hongweiliu17 @konflux-ci/integration
`,
			expected: []string{"@konflux-ci/Vanguard", "@dirgim", "@hongweiliu17", "@konflux-ci/integration"},
		},
		{
			name: "duplicate owners",
			content: `# CODEOWNERS
/foo @user1 @user2
/bar @user1 @user3
`,
			expected: []string{"@user1", "@user2", "@user3"},
		},
		{
			name:     "no owners",
			content:  "# Just a comment",
			expected: nil,
		},
		{
			name: "limit to 5 owners",
			content: `# CODEOWNERS
* @owner1 @owner2 @owner3 @owner4 @owner5 @owner6 @owner7
`,
			expected: []string{"@owner1", "@owner2", "@owner3", "@owner4", "@owner5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractOwnersFromCodeowners(tt.content)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("extractOwnersFromCodeowners() = %v, want %v", result, tt.expected)
			}
		})
	}
}
