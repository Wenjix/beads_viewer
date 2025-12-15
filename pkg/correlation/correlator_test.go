package correlation

import (
	"testing"
	"time"
)

func TestBuildHistories_Empty(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	histories := c.buildHistories(nil, nil, nil)

	if len(histories) != 0 {
		t.Errorf("expected empty histories, got %d", len(histories))
	}
}

func TestBuildHistories_Basic(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	beads := []BeadInfo{
		{ID: "bv-1", Title: "Task 1", Status: "open"},
		{ID: "bv-2", Title: "Task 2", Status: "closed"},
	}

	now := time.Now()
	events := []BeadEvent{
		{BeadID: "bv-1", EventType: EventCreated, Timestamp: now.Add(-24 * time.Hour), Author: "Alice"},
		{BeadID: "bv-1", EventType: EventClaimed, Timestamp: now.Add(-12 * time.Hour), Author: "Alice"},
		{BeadID: "bv-2", EventType: EventCreated, Timestamp: now.Add(-48 * time.Hour), Author: "Bob"},
		{BeadID: "bv-2", EventType: EventClosed, Timestamp: now.Add(-1 * time.Hour), Author: "Bob"},
	}

	histories := c.buildHistories(beads, events, nil)

	if len(histories) != 2 {
		t.Errorf("expected 2 histories, got %d", len(histories))
	}

	h1 := histories["bv-1"]
	if len(h1.Events) != 2 {
		t.Errorf("expected 2 events for bv-1, got %d", len(h1.Events))
	}
	if h1.Milestones.Created == nil {
		t.Error("expected bv-1 to have created milestone")
	}
	if h1.Milestones.Claimed == nil {
		t.Error("expected bv-1 to have claimed milestone")
	}

	h2 := histories["bv-2"]
	if len(h2.Events) != 2 {
		t.Errorf("expected 2 events for bv-2, got %d", len(h2.Events))
	}
	if h2.CycleTime == nil {
		t.Error("expected bv-2 to have cycle time (closed bead)")
	}
}

func TestBuildCommitIndex(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	histories := map[string]BeadHistory{
		"bv-1": {
			BeadID: "bv-1",
			Commits: []CorrelatedCommit{
				{SHA: "abc123", Method: MethodCoCommitted},
				{SHA: "def456", Method: MethodCoCommitted},
			},
		},
		"bv-2": {
			BeadID: "bv-2",
			Commits: []CorrelatedCommit{
				{SHA: "abc123", Method: MethodCoCommitted}, // Same commit, different bead
				{SHA: "ghi789", Method: MethodCoCommitted},
			},
		},
	}

	index := c.buildCommitIndex(histories)

	if len(index) != 3 {
		t.Errorf("expected 3 unique commits in index, got %d", len(index))
	}

	// abc123 should reference both beads
	if len(index["abc123"]) != 2 {
		t.Errorf("expected abc123 to reference 2 beads, got %d", len(index["abc123"]))
	}
}

func TestCalculateStats_Empty(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	stats := c.calculateStats(make(map[string]BeadHistory), nil)

	if stats.TotalBeads != 0 {
		t.Errorf("expected 0 total beads, got %d", stats.TotalBeads)
	}
	if stats.BeadsWithCommits != 0 {
		t.Errorf("expected 0 beads with commits, got %d", stats.BeadsWithCommits)
	}
}

func TestCalculateStats_WithData(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	claimToClose := 24 * time.Hour
	histories := map[string]BeadHistory{
		"bv-1": {
			BeadID: "bv-1",
			Events: []BeadEvent{
				{Author: "Alice"},
			},
			Commits: []CorrelatedCommit{
				{SHA: "abc123", Author: "Alice", Method: MethodCoCommitted},
			},
			CycleTime: &CycleTime{ClaimToClose: &claimToClose},
		},
		"bv-2": {
			BeadID: "bv-2",
			Events: []BeadEvent{
				{Author: "Bob"},
			},
			Commits: []CorrelatedCommit{
				{SHA: "def456", Author: "Bob", Method: MethodExplicitID},
			},
		},
	}

	stats := c.calculateStats(histories, nil)

	if stats.TotalBeads != 2 {
		t.Errorf("expected 2 total beads, got %d", stats.TotalBeads)
	}
	if stats.BeadsWithCommits != 2 {
		t.Errorf("expected 2 beads with commits, got %d", stats.BeadsWithCommits)
	}
	if stats.TotalCommits != 2 {
		t.Errorf("expected 2 total commits, got %d", stats.TotalCommits)
	}
	if stats.UniqueAuthors != 2 {
		t.Errorf("expected 2 unique authors, got %d", stats.UniqueAuthors)
	}
	if stats.MethodDistribution["co_committed"] != 1 {
		t.Errorf("expected 1 co_committed, got %d", stats.MethodDistribution["co_committed"])
	}
	if stats.MethodDistribution["explicit_id"] != 1 {
		t.Errorf("expected 1 explicit_id, got %d", stats.MethodDistribution["explicit_id"])
	}
	if stats.AvgCycleTimeDays == nil {
		t.Error("expected avg cycle time to be set")
	}
}

func TestDescribeGitRange(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	tests := []struct {
		name     string
		opts     CorrelatorOptions
		expected string
	}{
		{
			name:     "no filters",
			opts:     CorrelatorOptions{},
			expected: "all history",
		},
		{
			name: "with limit",
			opts: CorrelatorOptions{Limit: 100},
			expected: "limit 100 commits",
		},
		{
			name: "with since",
			opts: func() CorrelatorOptions {
				since := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
				return CorrelatorOptions{Since: &since}
			}(),
			expected: "since 2024-01-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.describeGitRange(tt.opts)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCalculateDataHash(t *testing.T) {
	c := NewCorrelator("/tmp/test")

	beads1 := []BeadInfo{
		{ID: "bv-1", Status: "open"},
		{ID: "bv-2", Status: "closed"},
	}

	beads2 := []BeadInfo{
		{ID: "bv-1", Status: "open"},
		{ID: "bv-2", Status: "open"}, // Different status
	}

	hash1 := c.calculateDataHash(beads1)
	hash2 := c.calculateDataHash(beads2)

	if hash1 == hash2 {
		t.Error("different bead data should produce different hashes")
	}

	// Same data should produce same hash
	hash1Again := c.calculateDataHash(beads1)
	if hash1 != hash1Again {
		t.Error("same bead data should produce same hash")
	}
}

func TestDedupCommits(t *testing.T) {
	commits := []CorrelatedCommit{
		{SHA: "abc123", Message: "First"},
		{SHA: "def456", Message: "Second"},
		{SHA: "abc123", Message: "First duplicate"}, // Duplicate SHA
		{SHA: "ghi789", Message: "Third"},
	}

	result := dedupCommits(commits)

	if len(result) != 3 {
		t.Errorf("expected 3 unique commits, got %d", len(result))
	}

	// First occurrence should be kept
	if result[0].Message != "First" {
		t.Errorf("expected first commit message to be 'First', got %s", result[0].Message)
	}
}
