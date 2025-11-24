package pr

import (
	"testing"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name     string
		fullName string
		expected string
	}{
		{
			name:     "org/repo format",
			fullName: "konflux-ci/caching",
			expected: "caching",
		},
		{
			name:     "no org prefix",
			fullName: "standalone-repo",
			expected: "standalone-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRepoName(tt.fullName)
			if result != tt.expected {
				t.Errorf("extractRepoName(%q) = %q, want %q", tt.fullName, result, tt.expected)
			}
		})
	}
}

func TestExtractReviewers(t *testing.T) {
	tests := []struct {
		name     string
		owners   []string
		expected []string
	}{
		{
			name:     "mixed teams and users with @ prefix",
			owners:   []string{"@konflux-ci/Vanguard", "@user1"},
			expected: []string{"konflux-ci/Vanguard", "user1"},
		},
		{
			name:     "empty list",
			owners:   []string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractReviewers(tt.owners)
			if len(result) != len(tt.expected) {
				t.Errorf("extractReviewers(%v) returned %d items, want %d", tt.owners, len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("extractReviewers(%v)[%d] = %q, want %q", tt.owners, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFormatList(t *testing.T) {
	tests := []struct {
		name      string
		items     []string
		emptyText string
		expected  string
	}{
		{
			name:      "multiple items",
			items:     []string{"vendor/", "hack/"},
			emptyText: "None",
			expected:  "- `vendor/`\n- `hack/`",
		},
		{
			name:      "empty list",
			items:     []string{},
			emptyText: "None",
			expected:  "None",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatList(tt.items, tt.emptyText)
			if result != tt.expected {
				t.Errorf("formatList(%v, %q) = %q, want %q", tt.items, tt.emptyText, result, tt.expected)
			}
		})
	}
}
