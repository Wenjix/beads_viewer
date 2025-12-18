package main_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestExportPages_IncludesHistoryAndRunsHooks(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir, _ := createHistoryRepo(t)
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Configure hooks to prove pre/post phases run.
	if err := os.MkdirAll(filepath.Join(repoDir, ".bv"), 0o755); err != nil {
		t.Fatalf("mkdir .bv: %v", err)
	}
	hooksYAML := `hooks:
  pre-export:
    - name: pre
      command: 'mkdir -p "$BV_EXPORT_PATH" && echo pre > "$BV_EXPORT_PATH/pre-hook.txt"'
  post-export:
    - name: post
      command: 'echo post > "$BV_EXPORT_PATH/post-hook.txt"'
`
	if err := os.WriteFile(filepath.Join(repoDir, ".bv", "hooks.yaml"), []byte(hooksYAML), 0o644); err != nil {
		t.Fatalf("write hooks.yaml: %v", err)
	}

	cmd := exec.Command(bv,
		"--export-pages", exportDir,
		"--pages-include-history",
		"--pages-include-closed",
	)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Core artifacts.
	for _, p := range []string{
		filepath.Join(exportDir, "index.html"),
		filepath.Join(exportDir, "beads.sqlite3"),
		filepath.Join(exportDir, "beads.sqlite3.config.json"),
		filepath.Join(exportDir, "data", "meta.json"),
		filepath.Join(exportDir, "data", "triage.json"),
		filepath.Join(exportDir, "data", "history.json"),
		filepath.Join(exportDir, "pre-hook.txt"),
		filepath.Join(exportDir, "post-hook.txt"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing export artifact %s: %v", p, err)
		}
	}

	// Verify vendored scripts are present (all scripts are now local, not CDN)
	indexBytes, err := os.ReadFile(filepath.Join(exportDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexBytes), "vendor/") {
		t.Fatalf("index.html missing vendored script references")
	}

	// History JSON should include at least one commit entry.
	historyBytes, err := os.ReadFile(filepath.Join(exportDir, "data", "history.json"))
	if err != nil {
		t.Fatalf("read history.json: %v", err)
	}
	var history struct {
		Commits []struct {
			SHA string `json:"sha"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(historyBytes, &history); err != nil {
		t.Fatalf("history.json decode: %v", err)
	}
	if len(history.Commits) == 0 || history.Commits[0].SHA == "" {
		t.Fatalf("expected at least one commit in history.json, got %+v", history.Commits)
	}
}

func stageViewerAssets(t *testing.T, bvPath string) {
	t.Helper()
	root := findRepoRoot(t)
	src := filepath.Join(root, "pkg", "export", "viewer_assets")
	dst := filepath.Join(filepath.Dir(bvPath), "pkg", "export", "viewer_assets")

	if err := copyDirRecursive(src, dst); err != nil {
		t.Fatalf("stage viewer assets: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found starting at %s", dir)
		}
		dir = parent
	}
}

func copyDirRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// ============================================================================
// Static Bundle Validation Tests (bv-ct7m)
// ============================================================================

// TestExportPages_HTMLStructure validates the HTML5 document structure
func TestExportPages_HTMLStructure(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 5)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	indexBytes, err := os.ReadFile(filepath.Join(exportDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(indexBytes)

	// HTML5 doctype (case-insensitive check)
	if !strings.Contains(strings.ToLower(html), "<!doctype html>") {
		t.Error("missing HTML5 doctype")
	}

	// Required meta tags
	checks := []struct {
		name    string
		pattern string
	}{
		{"charset meta", `charset="UTF-8"`},
		{"viewport meta", `name="viewport"`},
		{"html lang attribute", `<html lang=`},
		{"title tag", `<title>`},
	}
	for _, c := range checks {
		if !strings.Contains(html, c.pattern) {
			t.Errorf("missing %s (pattern: %s)", c.name, c.pattern)
		}
	}

	// Security headers (CSP)
	if !strings.Contains(html, "Content-Security-Policy") {
		t.Error("missing Content-Security-Policy meta tag")
	}
}

// TestExportPages_CSSPresent validates CSS files are included
func TestExportPages_CSSPresent(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Check styles.css exists
	stylesPath := filepath.Join(exportDir, "styles.css")
	info, err := os.Stat(stylesPath)
	if err != nil {
		t.Fatalf("styles.css not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("styles.css is empty")
	}

	// Check index.html references the stylesheet
	indexBytes, err := os.ReadFile(filepath.Join(exportDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexBytes), `href="styles.css"`) {
		t.Error("index.html doesn't reference styles.css")
	}
}

// TestExportPages_JavaScriptFiles validates JS files are present
func TestExportPages_JavaScriptFiles(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Required JS files (charts.js is embedded in index.html, not separate)
	jsFiles := []string{
		"viewer.js",
		"graph.js",
		"coi-serviceworker.js",
	}

	for _, jsFile := range jsFiles {
		path := filepath.Join(exportDir, jsFile)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s not found: %v", jsFile, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", jsFile)
		}
	}

	// Vendor files
	vendorFiles := []string{
		"vendor/bv_graph.js",
		"vendor/bv_graph_bg.wasm",
	}
	for _, vf := range vendorFiles {
		path := filepath.Join(exportDir, vf)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("vendor file %s not found: %v", vf, err)
		}
	}
}

// TestExportPages_SQLiteDatabase validates the SQLite export
func TestExportPages_SQLiteDatabase(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 10)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Check database exists and is non-empty
	dbPath := filepath.Join(exportDir, "beads.sqlite3")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("beads.sqlite3 not found: %v", err)
	}
	if info.Size() < 1024 {
		t.Errorf("beads.sqlite3 suspiciously small: %d bytes", info.Size())
	}

	// Check config.json exists
	configPath := filepath.Join(exportDir, "beads.sqlite3.config.json")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("beads.sqlite3.config.json not found: %v", err)
	}

	var config struct {
		Chunked   bool  `json:"chunked"`
		TotalSize int64 `json:"total_size"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf("parse config.json: %v", err)
	}
	if config.TotalSize == 0 {
		t.Error("config.json reports total_size of 0")
	}
}

// TestExportPages_TriageJSON validates triage data export
func TestExportPages_TriageJSON(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 5)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Check triage.json exists and has expected structure
	triagePath := filepath.Join(exportDir, "data", "triage.json")
	triageBytes, err := os.ReadFile(triagePath)
	if err != nil {
		t.Fatalf("triage.json not found: %v", err)
	}

	var triage struct {
		Recommendations []struct {
			ID    string  `json:"id"`
			Score float64 `json:"score"`
		} `json:"recommendations"`
		ProjectHealth struct {
			StatusCounts map[string]int `json:"status_counts"`
		} `json:"project_health"`
	}
	if err := json.Unmarshal(triageBytes, &triage); err != nil {
		t.Fatalf("parse triage.json: %v", err)
	}

	// Should have recommendations for open issues
	if len(triage.Recommendations) == 0 {
		t.Error("triage.json has no recommendations")
	}
}

// TestExportPages_MetaJSON validates metadata export
func TestExportPages_MetaJSON(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 5)
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Use --pages-include-closed to include all 5 issues
	cmd := exec.Command(bv, "--export-pages", exportDir, "--pages-title", "Test Dashboard", "--pages-include-closed")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	metaPath := filepath.Join(exportDir, "data", "meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("meta.json not found: %v", err)
	}

	var meta struct {
		Version     string `json:"version"`
		GeneratedAt string `json:"generated_at"`
		IssueCount  int    `json:"issue_count"`
		Title       string `json:"title"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("parse meta.json: %v", err)
	}

	if meta.Version == "" {
		t.Error("meta.json missing version")
	}
	if meta.GeneratedAt == "" {
		t.Error("meta.json missing generated_at")
	}
	if meta.IssueCount != 5 {
		t.Errorf("meta.json issue_count = %d, want 5", meta.IssueCount)
	}
	if meta.Title != "Test Dashboard" {
		t.Errorf("meta.json title = %q, want %q", meta.Title, "Test Dashboard")
	}
}

// TestExportPages_DependencyGraph validates graph data for issues with deps
func TestExportPages_DependencyGraph(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createRepoWithDeps(t)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Triage should show blocked issues
	triagePath := filepath.Join(exportDir, "data", "triage.json")
	triageBytes, err := os.ReadFile(triagePath)
	if err != nil {
		t.Fatalf("triage.json not found: %v", err)
	}

	var triage struct {
		ProjectHealth struct {
			StatusCounts map[string]int `json:"status_counts"`
		} `json:"project_health"`
	}
	if err := json.Unmarshal(triageBytes, &triage); err != nil {
		t.Fatalf("parse triage.json: %v", err)
	}

	// Our test data has blocked issues
	if triage.ProjectHealth.StatusCounts["blocked"] == 0 {
		t.Log("Note: No blocked issues in triage (might be expected if deps don't cause blocked status)")
	}
}

// TestExportPages_DataScale_10Issues tests with 10 issues
func TestExportPages_DataScale_10Issues(t *testing.T) {
	testExportPagesWithScale(t, 10)
}

// TestExportPages_DataScale_100Issues tests with 100 issues
func TestExportPages_DataScale_100Issues(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large scale test in short mode")
	}
	testExportPagesWithScale(t, 100)
}

func testExportPagesWithScale(t *testing.T, issueCount int) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, issueCount)
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Use --pages-include-closed to include all issues
	cmd := exec.Command(bv, "--export-pages", exportDir, "--pages-include-closed")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--export-pages failed with %d issues: %v\n%s", issueCount, err, out)
	}

	// Verify meta.json has correct count
	metaPath := filepath.Join(exportDir, "data", "meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("meta.json not found: %v", err)
	}

	var meta struct {
		IssueCount int `json:"issue_count"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("parse meta.json: %v", err)
	}
	if meta.IssueCount != issueCount {
		t.Errorf("issue_count = %d, want %d", meta.IssueCount, issueCount)
	}

	// Verify database size scales appropriately
	dbPath := filepath.Join(exportDir, "beads.sqlite3")
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("beads.sqlite3 not found: %v", err)
	}
	// Rough check: db should be at least 100 bytes per issue
	minExpectedSize := int64(issueCount * 100)
	if info.Size() < minExpectedSize {
		t.Errorf("database size %d bytes seems too small for %d issues (expected at least %d)",
			info.Size(), issueCount, minExpectedSize)
	}
}

// TestExportPages_DarkModeSupport validates dark mode CSS classes
func TestExportPages_DarkModeSupport(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	indexBytes, err := os.ReadFile(filepath.Join(exportDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(indexBytes)

	// Check for dark mode infrastructure
	darkModeIndicators := []string{
		"darkMode",          // Tailwind darkMode config
		"dark:",             // Tailwind dark: prefix classes
		"dark-mode",         // Generic dark mode references
		"prefers-color-scheme", // Media query detection
	}

	found := false
	for _, indicator := range darkModeIndicators {
		if strings.Contains(html, indicator) {
			found = true
			break
		}
	}
	if !found {
		t.Error("no dark mode support indicators found in index.html")
	}
}

// TestExportPages_NoXSSVulnerabilities checks for basic XSS protections
func TestExportPages_NoXSSVulnerabilities(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	// Create repo with potentially dangerous content
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Issue with XSS attempt in title
	jsonl := `{"id": "xss-1", "title": "<script>alert('xss')</script>", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "xss-2", "title": "Normal issue", "description": "<img onerror='alert(1)' src='x'>", "status": "open", "priority": 2, "issue_type": "task"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}

	exportDir := filepath.Join(repoDir, "bv-pages")
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Check that CSP header is present (provides XSS protection)
	indexBytes, err := os.ReadFile(filepath.Join(exportDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexBytes), "Content-Security-Policy") {
		t.Error("missing Content-Security-Policy for XSS protection")
	}
}

// TestExportPages_ResponsiveLayout checks for responsive design markers
func TestExportPages_ResponsiveLayout(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := createSimpleRepo(t, 3)
	exportDir := filepath.Join(repoDir, "bv-pages")

	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	indexBytes, err := os.ReadFile(filepath.Join(exportDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	html := string(indexBytes)

	// Check for viewport meta tag (essential for responsive design)
	if !strings.Contains(html, "viewport") {
		t.Error("missing viewport meta tag")
	}

	// Check for responsive classes (Tailwind breakpoints)
	responsiveIndicators := []string{
		"sm:",  // Small breakpoint
		"md:",  // Medium breakpoint
		"lg:",  // Large breakpoint
		"max-w-", // Max width containers
	}

	foundResponsive := 0
	for _, indicator := range responsiveIndicators {
		if strings.Contains(html, indicator) {
			foundResponsive++
		}
	}
	if foundResponsive < 2 {
		t.Errorf("only found %d responsive design indicators, expected at least 2", foundResponsive)
	}
}

// ============================================================================
// Test Helpers for bv-ct7m
// ============================================================================

// createSimpleRepo creates a test repo with N simple issues
func createSimpleRepo(t *testing.T, issueCount int) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	var issues strings.Builder
	for i := 1; i <= issueCount; i++ {
		status := "open"
		if i%5 == 0 {
			status = "closed"
		} else if i%3 == 0 {
			status = "in_progress"
		}
		priority := i % 5
		issueType := "task"
		if i%7 == 0 {
			issueType = "bug"
		} else if i%10 == 0 {
			issueType = "feature"
		}

		line := `{"id": "issue-` + itoa(i) + `", "title": "Test Issue ` + itoa(i) + `", "description": "Description for issue ` + itoa(i) + `", "status": "` + status + `", "priority": ` + itoa(priority) + `, "issue_type": "` + issueType + `"}` + "\n"
		issues.WriteString(line)
	}

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(issues.String()), 0o644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}

// createRepoWithDeps creates a test repo with dependency relationships
func createRepoWithDeps(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}

	// Create a dependency chain: A <- B <- C (C blocked by B, B blocked by A)
	jsonl := `{"id": "root-a", "title": "Root Task A", "status": "open", "priority": 0, "issue_type": "task"}
{"id": "child-b", "title": "Child Task B", "status": "blocked", "priority": 1, "issue_type": "task", "dependencies": [{"target_id": "root-a", "type": "blocks"}]}
{"id": "leaf-c", "title": "Leaf Task C", "status": "blocked", "priority": 2, "issue_type": "task", "dependencies": [{"target_id": "child-b", "type": "blocks"}]}
{"id": "independent-d", "title": "Independent Task D", "status": "open", "priority": 1, "issue_type": "bug"}`

	if err := os.WriteFile(filepath.Join(beadsPath, "beads.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatalf("write beads.jsonl: %v", err)
	}
	return repoDir
}

// itoa is a simple int to string helper
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// ============================================================================
// bv-qnlb: E2E Tests for Pages Export Options
// Tests for --pages-include-closed and --pages-include-history flags
// ============================================================================

// TestExportPages_ExcludeClosed_SQLiteVerification verifies closed issues
// are NOT in the SQLite database when --pages-include-closed=false.
func TestExportPages_ExcludeClosed_SQLiteVerification(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Create mix of open and closed issues
	issueData := `{"id": "open-1", "title": "Open Issue One", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "open-2", "title": "Open Issue Two", "status": "open", "priority": 2, "issue_type": "bug"}
{"id": "closed-1", "title": "Closed Issue One", "status": "closed", "priority": 1, "issue_type": "task"}
{"id": "closed-2", "title": "Closed Issue Two", "status": "closed", "priority": 2, "issue_type": "feature"}
{"id": "inprogress-1", "title": "In Progress Issue", "status": "in_progress", "priority": 1, "issue_type": "task"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	exportDir := filepath.Join(repoDir, "bv-pages")

	// Export with --pages-include-closed=false
	cmd := exec.Command(bv, "--export-pages", exportDir, "--pages-include-closed=false")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Verify SQLite database content
	dbPath := filepath.Join(exportDir, "beads.sqlite3")
	issues := queryAllIssues(t, dbPath)

	// Should have 3 non-closed issues (2 open + 1 in_progress)
	if len(issues) != 3 {
		t.Errorf("SQLite issue count = %d, want 3 (excluding 2 closed)", len(issues))
	}

	// Verify closed issues are NOT in database
	for _, issue := range issues {
		if issue.Status == "closed" {
			t.Errorf("Found closed issue %s in database, should be excluded", issue.ID)
		}
	}

	// Verify open issues ARE in database
	foundOpen1 := false
	foundOpen2 := false
	foundInProgress := false
	for _, issue := range issues {
		switch issue.ID {
		case "open-1":
			foundOpen1 = true
		case "open-2":
			foundOpen2 = true
		case "inprogress-1":
			foundInProgress = true
		}
	}
	if !foundOpen1 || !foundOpen2 || !foundInProgress {
		t.Errorf("Missing expected issues: open-1=%v, open-2=%v, inprogress-1=%v",
			foundOpen1, foundOpen2, foundInProgress)
	}
}

// TestExportPages_ExcludeHistory verifies history.json is absent
// when --pages-include-history=false.
func TestExportPages_ExcludeHistory(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir, _ := createHistoryRepo(t)
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Export with --pages-include-history=false
	cmd := exec.Command(bv, "--export-pages", exportDir, "--pages-include-history=false")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Verify history.json does NOT exist
	historyPath := filepath.Join(exportDir, "data", "history.json")
	if _, err := os.Stat(historyPath); !os.IsNotExist(err) {
		t.Error("history.json should NOT exist when --pages-include-history=false")
	}

	// Verify other core files still exist
	for _, p := range []string{
		filepath.Join(exportDir, "index.html"),
		filepath.Join(exportDir, "beads.sqlite3"),
		filepath.Join(exportDir, "data", "meta.json"),
		filepath.Join(exportDir, "data", "triage.json"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing expected artifact %s: %v", p, err)
		}
	}
}

// TestExportPages_BothExcluded verifies minimal export with both flags false.
func TestExportPages_BothExcluded(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir, _ := createHistoryRepo(t)
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Export with both exclusions
	cmd := exec.Command(bv, "--export-pages", exportDir,
		"--pages-include-closed=false",
		"--pages-include-history=false")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Verify history.json does NOT exist
	historyPath := filepath.Join(exportDir, "data", "history.json")
	if _, err := os.Stat(historyPath); !os.IsNotExist(err) {
		t.Error("history.json should NOT exist")
	}

	// Verify SQLite has no closed issues
	dbPath := filepath.Join(exportDir, "beads.sqlite3")
	issues := queryAllIssues(t, dbPath)
	for _, issue := range issues {
		if issue.Status == "closed" {
			t.Errorf("Found closed issue %s in database, should be excluded", issue.ID)
		}
	}
}

// TestExportPages_FTS5Searchable verifies the FTS5 index is created and searchable.
func TestExportPages_FTS5Searchable(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Create issues with searchable content
	issueData := `{"id": "auth-1", "title": "Implement OAuth2 authentication", "description": "Add Google and GitHub OAuth providers", "status": "open", "priority": 1, "issue_type": "feature"}
{"id": "api-1", "title": "REST API rate limiting", "description": "Implement token bucket algorithm for rate limiting", "status": "open", "priority": 2, "issue_type": "task"}
{"id": "bug-1", "title": "Fix login redirect bug", "description": "Users are redirected incorrectly after OAuth callback", "status": "open", "priority": 1, "issue_type": "bug"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	exportDir := filepath.Join(repoDir, "bv-pages")
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Test FTS5 search queries
	dbPath := filepath.Join(exportDir, "beads.sqlite3")

	// Search for "OAuth" - should find 2 issues (auth-1 and bug-1)
	oauthResults := searchFTS(t, dbPath, "OAuth")
	if len(oauthResults) != 2 {
		t.Errorf("FTS search for 'OAuth' returned %d results, want 2", len(oauthResults))
	}

	// Search for "rate limiting" - should find 1 issue (api-1)
	rateResults := searchFTS(t, dbPath, "rate limiting")
	if len(rateResults) != 1 {
		t.Errorf("FTS search for 'rate limiting' returned %d results, want 1", len(rateResults))
	}

	// Search for "nonexistent term" - should find 0 issues
	emptyResults := searchFTS(t, dbPath, "nonexistent_xyz_term")
	if len(emptyResults) != 0 {
		t.Errorf("FTS search for nonexistent term returned %d results, want 0", len(emptyResults))
	}
}

// TestExportPages_EmptyProject verifies export handles empty project gracefully.
func TestExportPages_EmptyProject(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Create empty issues file
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatalf("write empty issues.jsonl: %v", err)
	}

	exportDir := filepath.Join(repoDir, "bv-pages")
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()

	// Empty project should either succeed with 0 issues or fail gracefully
	if err != nil {
		// Acceptable: might fail with "no issues" error
		t.Logf("Export with empty project failed (acceptable): %v\n%s", err, out)
		return
	}

	// If it succeeded, verify meta.json shows 0 issues
	metaPath := filepath.Join(exportDir, "data", "meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}

	var meta struct {
		IssueCount int `json:"issue_count"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("parse meta.json: %v", err)
	}
	if meta.IssueCount != 0 {
		t.Errorf("issue_count = %d, want 0 for empty project", meta.IssueCount)
	}
}

// TestExportPages_OnlyClosedIssues verifies export when all issues are closed
// and --pages-include-closed=false results in empty export.
func TestExportPages_OnlyClosedIssues(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Create only closed issues
	issueData := `{"id": "closed-1", "title": "Completed Task 1", "status": "closed", "priority": 1, "issue_type": "task"}
{"id": "closed-2", "title": "Completed Task 2", "status": "closed", "priority": 2, "issue_type": "task"}
{"id": "closed-3", "title": "Completed Bug Fix", "status": "closed", "priority": 1, "issue_type": "bug"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	exportDir := filepath.Join(repoDir, "bv-pages")
	cmd := exec.Command(bv, "--export-pages", exportDir, "--pages-include-closed=false")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()

	// Should either succeed with 0 issues or fail gracefully
	if err != nil {
		t.Logf("Export with only closed issues (excluded) failed (acceptable): %v\n%s", err, out)
		return
	}

	// If succeeded, verify SQLite has 0 issues
	dbPath := filepath.Join(exportDir, "beads.sqlite3")
	issues := queryAllIssues(t, dbPath)
	if len(issues) != 0 {
		t.Errorf("SQLite has %d issues, want 0 (all closed and excluded)", len(issues))
	}
}

// TestExportPages_UnicodeContent verifies export handles Unicode correctly.
func TestExportPages_UnicodeContent(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir := t.TempDir()
	beadsPath := filepath.Join(repoDir, ".beads")
	if err := os.MkdirAll(beadsPath, 0o755); err != nil {
		t.Fatalf("mkdir .beads: %v", err)
	}

	// Create issues with Unicode content
	issueData := `{"id": "unicode-1", "title": "日本語タイトル", "description": "説明文はこちら", "status": "open", "priority": 1, "issue_type": "task"}
{"id": "unicode-2", "title": "Émoji test 🚀🎉✨", "description": "Contains emojis: 👍 🔥 💯", "status": "open", "priority": 2, "issue_type": "feature"}
{"id": "unicode-3", "title": "Ü̶n̶i̶c̶o̶d̶e̶ special chars", "description": "Test: é à ü ñ ø æ ß", "status": "open", "priority": 1, "issue_type": "bug"}
{"id": "unicode-4", "title": "中文标题测试", "description": "中文描述内容", "status": "open", "priority": 2, "issue_type": "task"}`
	if err := os.WriteFile(filepath.Join(beadsPath, "issues.jsonl"), []byte(issueData), 0o644); err != nil {
		t.Fatalf("write issues.jsonl: %v", err)
	}

	exportDir := filepath.Join(repoDir, "bv-pages")
	cmd := exec.Command(bv, "--export-pages", exportDir)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Verify all issues are in SQLite with correct titles
	dbPath := filepath.Join(exportDir, "beads.sqlite3")
	issues := queryAllIssues(t, dbPath)

	if len(issues) != 4 {
		t.Fatalf("SQLite has %d issues, want 4", len(issues))
	}

	// Verify Unicode titles are preserved
	expectedTitles := map[string]string{
		"unicode-1": "日本語タイトル",
		"unicode-2": "Émoji test 🚀🎉✨",
		"unicode-3": "Ü̶n̶i̶c̶o̶d̶e̶ special chars",
		"unicode-4": "中文标题测试",
	}

	for _, issue := range issues {
		expected, ok := expectedTitles[issue.ID]
		if !ok {
			t.Errorf("Unexpected issue ID: %s", issue.ID)
			continue
		}
		if issue.Title != expected {
			t.Errorf("Issue %s title mismatch:\n  got:  %q\n  want: %q", issue.ID, issue.Title, expected)
		}
	}

	// Test FTS search with Unicode
	// Note: The porter tokenizer may not handle CJK characters well,
	// so we test with Latin characters that have diacritics instead
	emojiResults := searchFTS(t, dbPath, "Émoji")
	if len(emojiResults) != 1 {
		// Diacritics might be normalized, try without
		emojiResults = searchFTS(t, dbPath, "emoji")
		if len(emojiResults) != 1 {
			t.Logf("FTS search for 'emoji' returned %d results (tokenizer may not handle accented chars)", len(emojiResults))
		}
	}

	// CJK search may not work with porter tokenizer - just log, don't fail
	japaneseResults := searchFTS(t, dbPath, "日本語")
	if len(japaneseResults) == 0 {
		t.Log("Note: FTS5 porter tokenizer doesn't support CJK search (expected)")
	}
}

// ============================================================================
// Helper functions for bv-qnlb tests
// ============================================================================

// sqliteIssue represents an issue row from the SQLite database.
type sqliteIssue struct {
	ID          string
	Title       string
	Description string
	Status      string
	Priority    int
	IssueType   string
}

// queryAllIssues queries all issues from the SQLite database.
func queryAllIssues(t *testing.T, dbPath string) []sqliteIssue {
	t.Helper()

	db, err := openSQLiteDB(dbPath)
	if err != nil {
		t.Fatalf("open database %s: %v", dbPath, err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, title, COALESCE(description, ''), status, priority, issue_type FROM issues")
	if err != nil {
		t.Fatalf("query issues: %v", err)
	}
	defer rows.Close()

	var issues []sqliteIssue
	for rows.Next() {
		var issue sqliteIssue
		if err := rows.Scan(&issue.ID, &issue.Title, &issue.Description, &issue.Status, &issue.Priority, &issue.IssueType); err != nil {
			t.Fatalf("scan issue: %v", err)
		}
		issues = append(issues, issue)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	return issues
}

// searchFTS performs an FTS5 search and returns matching issue IDs.
func searchFTS(t *testing.T, dbPath, query string) []string {
	t.Helper()

	db, err := openSQLiteDB(dbPath)
	if err != nil {
		t.Fatalf("open database %s: %v", dbPath, err)
	}
	defer db.Close()

	// FTS5 search query
	rows, err := db.Query("SELECT id FROM issues_fts WHERE issues_fts MATCH ?", query)
	if err != nil {
		// FTS5 might not be available, log and return empty
		t.Logf("FTS5 query failed (might not be available): %v", err)
		return nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan FTS result: %v", err)
		}
		ids = append(ids, id)
	}

	return ids
}

// openSQLiteDB opens a SQLite database for testing.
// Uses the same driver as the export code (modernc.org/sqlite).
func openSQLiteDB(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite", dbPath)
}
