package analysis

import (
	"testing"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// Cover getter and configured analysis pathways that were previously untested.
func TestAnalyzerProfileAndGetters(t *testing.T) {
	issues := []model.Issue{
		{ID: "A", Title: "Alpha", Status: model.StatusOpen, Dependencies: []*model.Dependency{{DependsOnID: "B", Type: model.DepBlocks}}},
		{ID: "B", Title: "Beta", Status: model.StatusOpen},
	}

	custom := ConfigForSize(len(issues), 1)
	a := NewAnalyzer(issues)
	a.SetConfig(&custom)

	stats, profile := a.AnalyzeWithProfile(custom)
	if profile == nil || stats == nil {
		t.Fatalf("expected stats and profile")
	}
	if !stats.IsPhase2Ready() {
		t.Fatalf("phase2 should be ready after AnalyzeWithProfile")
	}

	_ = a.GetIssue("A")
	_ = stats.GetPageRankScore("A")
	_ = stats.GetBetweennessScore("A")
	_ = stats.GetEigenvectorScore("A")
	_ = stats.GetHubScore("A")
	_ = stats.GetAuthorityScore("A")
	_ = stats.GetCriticalPathScore("A")
}

func TestAnalyzerAnalyzeWithConfigCachesPhase2(t *testing.T) {
	issues := []model.Issue{{ID: "X", Status: model.StatusOpen}}
	a := NewAnalyzer(issues)
	cfg := FullAnalysisConfig()
	stats := a.AnalyzeWithConfig(cfg)
	stats.WaitForPhase2()
	if stats.NodeCount != 1 || stats.EdgeCount != 0 {
		t.Fatalf("unexpected counts: nodes=%d edges=%d", stats.NodeCount, stats.EdgeCount)
	}
	if stats.IsPhase2Ready() == false {
		t.Fatalf("expected phase2 ready")
	}
	// Ensure empty graph path still returns non-nil profile
	a2 := NewAnalyzer(nil)
	if _, profile := a2.AnalyzeWithProfile(cfg); profile == nil {
		t.Fatalf("expected non-nil profile for empty graph")
	}
	// Tiny sleep to avoid zero durations in formatDuration paths
	time.Sleep(1 * time.Millisecond)
}
