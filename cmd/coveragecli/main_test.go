package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseVitestSummary_GroupByDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	summaryPath := filepath.Join(tmp, "coverage-summary.json")

	data := map[string]any{
		"total": map[string]any{
			"lines": map[string]any{"total": 30, "covered": 18, "skipped": 0, "pct": 60.0},
		},
		"/repo/src/a.ts": map[string]any{
			"lines": map[string]any{"total": 10, "covered": 8, "skipped": 0, "pct": 80.0},
		},
		"/repo/src/nested/b.ts": map[string]any{
			"lines": map[string]any{"total": 20, "covered": 10, "skipped": 0, "pct": 50.0},
		},
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal test data: %v", err)
	}
	if err := os.WriteFile(summaryPath, raw, 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	total, pkgs, considered, err := parseVitestSummary(summaryPath, "lines", "dir", "/repo", nil, nil)
	if err != nil {
		t.Fatalf("parseVitestSummary() error = %v", err)
	}

	if total != 60 {
		t.Fatalf("unexpected total: got %.2f want 60.00", total)
	}
	if considered != 2 {
		t.Fatalf("unexpected considered files: got %d want 2", considered)
	}
	if len(pkgs) != 2 {
		t.Fatalf("unexpected package count: got %d want 2", len(pkgs))
	}

	if pkgs[0].ImportPath != "src" || pkgs[0].CoveragePercent != 80 {
		t.Fatalf("unexpected pkg[0]: %+v", pkgs[0])
	}
	if pkgs[1].ImportPath != "src/nested" || pkgs[1].CoveragePercent != 50 {
		t.Fatalf("unexpected pkg[1]: %+v", pkgs[1])
	}
}

func TestParseVitestSummary_GroupByFileWithFilters(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	summaryPath := filepath.Join(tmp, "coverage-summary.json")

	data := map[string]any{
		"total": map[string]any{
			"lines": map[string]any{"total": 40, "covered": 20, "skipped": 0, "pct": 50.0},
		},
		`C:\repo\src\main.ts`: map[string]any{
			"lines": map[string]any{"total": 10, "covered": 5, "skipped": 0, "pct": 50.0},
		},
		`C:\repo\src\feature\view.ts`: map[string]any{
			"lines": map[string]any{"total": 20, "covered": 15, "skipped": 0, "pct": 75.0},
		},
		`C:\repo\e2e\spec.ts`: map[string]any{
			"lines": map[string]any{"total": 10, "covered": 0, "skipped": 0, "pct": 0.0},
		},
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal test data: %v", err)
	}
	if err := os.WriteFile(summaryPath, raw, 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	total, pkgs, considered, err := parseVitestSummary(
		summaryPath,
		"lines",
		"file",
		`C:\repo`,
		[]string{"src/**"},
		[]string{"**/view.ts"},
	)
	if err != nil {
		t.Fatalf("parseVitestSummary() error = %v", err)
	}

	if total != 50 {
		t.Fatalf("unexpected total: got %.2f want 50.00", total)
	}
	if considered != 1 {
		t.Fatalf("unexpected considered files: got %d want 1", considered)
	}
	if len(pkgs) != 1 {
		t.Fatalf("unexpected package count: got %d want 1", len(pkgs))
	}
	if pkgs[0].ImportPath != "src/main.ts" || pkgs[0].CoveragePercent != 50 {
		t.Fatalf("unexpected package: %+v", pkgs[0])
	}
}

func TestParseVitestSummary_EmptyDatasetAfterFiltering(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	summaryPath := filepath.Join(tmp, "coverage-summary.json")

	data := map[string]any{
		"total": map[string]any{
			"lines": map[string]any{"total": 10, "covered": 5, "skipped": 0, "pct": 50.0},
		},
		"/repo/src/a.ts": map[string]any{
			"lines": map[string]any{"total": 10, "covered": 5, "skipped": 0, "pct": 50.0},
		},
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal test data: %v", err)
	}
	if err := os.WriteFile(summaryPath, raw, 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	_, _, _, err = parseVitestSummary(summaryPath, "lines", "dir", "/repo", []string{"tests/**"}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestParseVitestSummary_FromDummyFixture_GroupByDir(t *testing.T) {
	t.Parallel()

	summaryPath := filepath.Join("testdata", "vitest-coverage-summary-dummy.json")

	total, pkgs, considered, err := parseVitestSummary(summaryPath, "lines", "dir", "", nil, nil)
	if err != nil {
		t.Fatalf("parseVitestSummary() error = %v", err)
	}

	if total != 70 {
		t.Fatalf("unexpected total: got %.2f want 70.00", total)
	}
	if considered != 4 {
		t.Fatalf("unexpected considered files: got %d want 4", considered)
	}
	if len(pkgs) != 3 {
		t.Fatalf("unexpected package count: got %d want 3", len(pkgs))
	}

	if pkgs[0].ImportPath != "e2e" || pkgs[0].CoveragePercent != 50 {
		t.Fatalf("unexpected pkg[0]: %+v", pkgs[0])
	}
	if pkgs[1].ImportPath != "src/features/reviewPlan/components" || pkgs[1].CoveragePercent != 60 {
		t.Fatalf("unexpected pkg[1]: %+v", pkgs[1])
	}
	if pkgs[2].ImportPath != "src/features/selectPlan/components" || pkgs[2].CoveragePercent != 83.08 {
		t.Fatalf("unexpected pkg[2]: %+v", pkgs[2])
	}
}

func TestParseVitestSummary_FromDummyFixture_WithFilters(t *testing.T) {
	t.Parallel()

	summaryPath := filepath.Join("testdata", "vitest-coverage-summary-dummy.json")

	total, pkgs, considered, err := parseVitestSummary(
		summaryPath,
		"lines",
		"file",
		"",
		[]string{"src/**"},
		[]string{"**/PlanCardHeader.tsx"},
	)
	if err != nil {
		t.Fatalf("parseVitestSummary() error = %v", err)
	}

	if total != 70 {
		t.Fatalf("unexpected total: got %.2f want 70.00", total)
	}
	if considered != 2 {
		t.Fatalf("unexpected considered files: got %d want 2", considered)
	}
	if len(pkgs) != 2 {
		t.Fatalf("unexpected package count: got %d want 2", len(pkgs))
	}

	if pkgs[0].ImportPath != "src/features/reviewPlan/components/ReviewPlan.tsx" || pkgs[0].CoveragePercent != 60 {
		t.Fatalf("unexpected pkg[0]: %+v", pkgs[0])
	}
	if pkgs[1].ImportPath != "src/features/selectPlan/components/PlanCard.tsx" || pkgs[1].CoveragePercent != 78 {
		t.Fatalf("unexpected pkg[1]: %+v", pkgs[1])
	}
}
