package search

import (
	"strings"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// IssueDocument returns the default text representation used for semantic indexing.
// For bv-9gf.3 we intentionally keep this minimal (title + description) for predictability.
func IssueDocument(issue model.Issue) string {
	title := strings.TrimSpace(issue.Title)
	desc := strings.TrimSpace(issue.Description)
	if title == "" {
		return desc
	}
	if desc == "" {
		return title
	}
	return title + "\n" + desc
}

// DocumentsFromIssues builds an ID->document map suitable for indexing.
func DocumentsFromIssues(issues []model.Issue) map[string]string {
	docs := make(map[string]string, len(issues))
	for _, issue := range issues {
		if issue.ID == "" {
			continue
		}
		docs[issue.ID] = IssueDocument(issue)
	}
	return docs
}
