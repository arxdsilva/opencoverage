package main

import (
	"encoding/json"
	"encoding/xml"
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

func loadTestData(t *testing.T, filename string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatalf("failed to read test data %s: %v", filename, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse test data %s: %v", filename, err)
	}
	return result
}

func assertHierarchy(t *testing.T, spec map[string]any, expected []string) {
	t.Helper()
	raw, ok := spec["containerHierarchyTexts"].([]any)
	if !ok {
		t.Fatalf("containerHierarchyTexts: expected []any, got %T", spec["containerHierarchyTexts"])
	}
	if len(raw) != len(expected) {
		t.Fatalf("hierarchy length: expected %d, got %d (%v)", len(expected), len(raw), raw)
	}
	for i, want := range expected {
		got, ok := raw[i].(string)
		if !ok {
			t.Fatalf("hierarchy[%d]: expected string, got %T", i, raw[i])
		}
		if got != want {
			t.Fatalf("hierarchy[%d]: expected %q, got %q", i, want, got)
		}
	}
}
func TestNormalizePlaywrightReport(t *testing.T) {
	t.Parallel()

	t.Run("normalizes report with all tests passed", func(t *testing.T) {
		t.Parallel()

		raw := loadTestData(t, "playwright-report-pass-dummy.json")
		result := normalizePlaywrightReport(raw)

		if result["suiteDescription"] != "first title" {
			t.Fatalf("field %q: expected %q, got %q", "suiteDescription", "first title", result["suiteDescription"])
		}
		if result["suitePath"] != "test/e2e" {
			t.Fatalf("field %q: expected %q, got %q", "suitePath", "test/e2e", result["suitePath"])
		}
		if result["suitePath"] != "test/e2e" {
			t.Fatalf("field %q: expected %q, got %q", "suitePath", "test/e2e", result["suitePath"])
		}
		if result["frameworkVersion"] != "1.58.2" {
			t.Fatalf("field %q: expected %q, got %q", "frameworkVersion", "1.58.2", result["frameworkVersion"])
		}
		if rt, ok := result["reportType"].(*string); !ok || *rt != "playwright" {
			t.Fatalf("field %q: expected %q, got %v", "reportType", "playwright", result["reportType"])
		}
		if tf, ok := result["testFramework"].(*string); !ok || *tf != "playwright" {
			t.Fatalf("field %q: expected %q, got %v", "testFramework", "playwright", result["testFramework"])
		}

		specs, ok := result["specReports"].([]map[string]any)
		if !ok {
			t.Fatalf("specReports: expected []map[string]any, got %T", result["specReports"])
		}
		if len(specs) != 3 {
			t.Fatalf("expected 3 specs, got %d", len(specs))
		}

		// Spec 0: top-level suite "first title" > spec "first title"
		if specs[0]["leafNodeText"] != "first title" {
			t.Fatalf("spec[0].leafNodeText: expected %q, got %q", "first title", specs[0]["leafNodeText"])
		}
		if specs[0]["state"] != "passed" {
			t.Fatalf("spec[0].state: expected %q, got %q", "passed", specs[0]["state"])
		}

		assertHierarchy(t, specs[0], []string{"first title"})

		// Spec 1: "title 2" > "Title 2" > spec "Title 3"
		if specs[1]["leafNodeText"] != "Title 3" {
			t.Fatalf("spec[1].leafNodeText: expected %q, got %q", "Title 3", specs[1]["leafNodeText"])
		}
		if specs[1]["state"] != "passed" {
			t.Fatalf("spec[1].state: expected %q, got %q", "passed", specs[1]["state"])
		}
		assertHierarchy(t, specs[1], []string{"title 2", "Title 2"})

		// Spec 2: "title 2" > "Title 2" > spec "Title 4"
		if specs[2]["leafNodeText"] != "Title 4" {
			t.Fatalf("spec[2].leafNodeText: expected %q, got %q", "Title 4", specs[2]["leafNodeText"])
		}
		if specs[2]["state"] != "passed" {
			t.Fatalf("spec[2].state: expected %q, got %q", "passed", specs[2]["state"])
		}
		assertHierarchy(t, specs[2], []string{"title 2", "Title 2"})

		// No failures on any spec
		for i, spec := range specs {
			if _, hasFailure := spec["failure"]; hasFailure {
				t.Fatalf("spec[%d] should not have a failure block", i)
			}
		}
	})

	t.Run("normalizes report with failed test and strips ANSI", func(t *testing.T) {
		t.Parallel()

		raw := loadTestData(t, "playwright-report-fail-dummy.json")
		result := normalizePlaywrightReport(raw)

		if result["suiteDescription"] != "first title" {
			t.Fatalf("field %q: expected %q, got %q", "suiteDescription", "first title", result["suiteDescription"])
		}
		if result["suitePath"] != "test/e2e" {
			t.Fatalf("field %q: expected %q, got %q", "suitePath", "test/e2e", result["suitePath"])
		}
		if result["frameworkVersion"] != "1.58.2" {
			t.Fatalf("field %q: expected %q, got %q", "frameworkVersion", "1.58.2", result["frameworkVersion"])
		}
		if rt, ok := result["reportType"].(*string); !ok || *rt != "playwright" {
			t.Fatalf("field %q: expected %q, got %v", "reportType", "playwright", result["reportType"])
		}
		if tf, ok := result["testFramework"].(*string); !ok || *tf != "playwright" {
			t.Fatalf("field %q: expected %q, got %v", "testFramework", "playwright", result["testFramework"])
		}

		specs, ok := result["specReports"].([]map[string]any)
		if !ok {
			t.Fatalf("specReports: expected []map[string]any, got %T", result["specReports"])
		}
		if len(specs) != 3 {
			t.Fatalf("expected 3 specs, got %d", len(specs))
		}

		// Spec 0: passed
		if specs[0]["leafNodeText"] != "first title" {
			t.Fatalf("spec[0].leafNodeText: expected %q, got %q", "first title", specs[0]["leafNodeText"])
		}
		if specs[0]["state"] != "passed" {
			t.Fatalf("spec[0].state: expected %q, got %q", "passed", specs[0]["state"])
		}
		if _, hasFailure := specs[0]["failure"]; hasFailure {
			t.Fatal("passed spec should not have failure")
		}

		// Spec 1: passed nested
		if specs[1]["leafNodeText"] != "Title 3" {
			t.Fatalf("spec[1].leafNodeText: expected %q, got %q", "Title 3", specs[1]["leafNodeText"])
		}
		if specs[1]["state"] != "passed" {
			t.Fatalf("spec[1].state: expected %q, got %q", "passed", specs[1]["state"])
		}
		assertHierarchy(t, specs[1], []string{"title 4", "Title 4"})

		// Spec 2: failed with ANSI-stripped error
		if specs[2]["leafNodeText"] != "title 5" {
			t.Fatalf("spec[2].leafNodeText: expected %q, got %q", "title 5", specs[2]["leafNodeText"])
		}
		if specs[2]["state"] != "failed" {
			t.Fatalf("spec[2].state: expected %q, got %q", "failed", specs[2]["state"])
		}
		assertHierarchy(t, specs[2], []string{"title 4", "Title 4"})

		failureRaw, hasFailure := specs[2]["failure"]
		if !hasFailure {
			t.Fatal("failed spec should have a failure block")
		}
		failure := failureRaw.(map[string]any)
		msg := failure["message"].(string)
		if msg != "Test timeout of 150000ms exceeded." {
			t.Fatalf("expected ANSI-stripped message, got %q", msg)
		}
	})
}

func TestStripANSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"strips color codes", "\x1b[31mError\x1b[39m", "Error"},
		{"strips multiple codes", "\x1b[31m\x1b[1mBold Red\x1b[22m\x1b[39m", "Bold Red"},
		{"no-op on clean string", "clean message", "clean message"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripANSI(tt.input)
			if got != tt.want {
				t.Fatalf("stripANSI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeAppiumJUnit(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("testdata", "appium-junit-report.xml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var data JUnitTestSuites
	if err := xml.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal xml: %v", err)
	}

	result, err := normalizeAppiumJUnit(data)
	if err != nil {
		t.Fatalf("normalizeAppiumJUnit: %v", err)
	}

	// Verify metadata
	if rt, ok := result["reportType"].(*string); !ok || *rt != "appium" {
		t.Fatalf("reportType = %v, want *appium", result["reportType"])
	}
	if result["suiteDescription"] != "Appium Android Test Suite" {
		t.Fatalf("suiteDescription = %v, want 'Appium Android Test Suite'", result["suiteDescription"])
	}
	if result["platformType"] != "android" {
		t.Fatalf("platformType = %v, want android", result["platformType"])
	}
	if result["frameworkVersion"] != "UiAutomator2" {
		t.Fatalf("frameworkVersion = %v, want UiAutomator2", result["frameworkVersion"])
	}

	specs, ok := result["specReports"].([]map[string]any)
	if !ok {
		t.Fatalf("specReports is not []map[string]any")
	}
	if len(specs) != 3 {
		t.Fatalf("specReports count = %d, want 3", len(specs))
	}

	tests := []struct {
		name     string
		state    string
		specType string
		time     float64
	}{
		{"Test1", "passed", "happyPath", 12.542},
		{"Test2", "failed", "happyPath", 9.118},
		{"Test3", "passed", "happyPath", 20.658},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := specs[i]
			if spec["leafNodeText"] != tt.name {
				t.Errorf("spec[%d] leafNodeText = %v, want %v", i, spec["leafNodeText"], tt.name)
			}
			if spec["state"] != tt.state {
				t.Errorf("spec[%d] state = %v, want %v", i, spec["state"], tt.state)
			}
			if spec["specType"] != tt.specType {
				t.Errorf("spec[%d] specType = %v, want %v", i, spec["specType"], tt.specType)
			}
			if spec["runTime"] != tt.time {
				t.Errorf("spec[%d] runTime = %v, want %v", i, spec["runTime"], tt.time)
			}
			// Verify hierarchy
			hier, ok := spec["containerHierarchyTexts"].([]any)
			if !ok || len(hier) == 0 {
				t.Errorf("spec[%d] containerHierarchyTexts empty", i)
			}
			if len(hier) > 0 && hier[0] != "Tests" {
				t.Errorf("spec[%d] hierarchy[0] = %v, want Tests", i, hier[0])
			}
		})
	}

	// Verify failure block on the failed spec
	failedSpec := specs[1]
	failure, ok := failedSpec["failure"].(map[string]any)
	if !ok {
		t.Fatalf("failed spec has no failure block")
	}
	if failure["message"] != "Assertion failed: Expected error message not displayed" {
		t.Errorf("failure message = %v", failure["message"])
	}
	if _, ok := failure["stackTrace"]; !ok {
		t.Errorf("failure missing stackTrace")
	}
}
