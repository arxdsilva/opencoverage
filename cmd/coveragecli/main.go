package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type globList []string

func (g *globList) String() string {
	if len(*g) == 0 {
		return ""
	}
	return strings.Join(*g, ",")
}

func (g *globList) Set(value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return fmt.Errorf("glob cannot be empty")
	}
	*g = append(*g, v)
	return nil
}

type packageCoverage struct {
	ImportPath      string  `json:"importPath"`
	CoveragePercent float64 `json:"coveragePercent"`
}

type ingestPayload struct {
	ProjectKey           string            `json:"projectKey"`
	ProjectName          string            `json:"projectName,omitempty"`
	ProjectGroup         *string           `json:"projectGroup,omitempty"`
	DefaultBranch        string            `json:"defaultBranch,omitempty"`
	Branch               string            `json:"branch"`
	CommitSHA            string            `json:"commitSha"`
	Author               string            `json:"author,omitempty"`
	TriggerType          string            `json:"triggerType"`
	RunTimestamp         string            `json:"runTimestamp"`
	TotalCoveragePercent float64           `json:"totalCoveragePercent"`
	ThresholdPercent     *float64          `json:"thresholdPercent,omitempty"`
	Packages             []packageCoverage `json:"packages"`
}

type integrationPayload struct {
	ProjectKey    string         `json:"projectKey"`
	ProjectName   string         `json:"projectName,omitempty"`
	ProjectGroup  *string        `json:"projectGroup,omitempty"`
	DefaultBranch string         `json:"defaultBranch,omitempty"`
	Branch        string         `json:"branch"`
	CommitSHA     string         `json:"commitSha"`
	Author        string         `json:"author,omitempty"`
	TriggerType   string         `json:"triggerType"`
	RunTimestamp  string         `json:"runTimestamp"`
	Environment   *string        `json:"environment,omitempty"`
	GinkgoReport  map[string]any `json:"ginkgoReport"`
}

type e2ePayload struct {
	ProjectKey    string         `json:"projectKey"`
	ProjectName   string         `json:"projectName,omitempty"`
	ProjectGroup  *string        `json:"projectGroup,omitempty"`
	DefaultBranch string         `json:"defaultBranch,omitempty"`
	Branch        string         `json:"branch"`
	CommitSHA     string         `json:"commitSha"`
	Author        string         `json:"author,omitempty"`
	TriggerType   string         `json:"triggerType"`
	RunTimestamp  string         `json:"runTimestamp"`
	Environment   *string        `json:"environment,omitempty"`
	TestReport    map[string]any `json:"testReport"`
}

type uploadResponse struct {
	Run struct {
		Status          string  `json:"status"`
		PassRatePercent float64 `json:"passRatePercent"`
	} `json:"run"`
	Comparison struct {
		DeltaPercent *float64 `json:"deltaPercent"`
	} `json:"comparison"`
}

type vitestMetric struct {
	Total   float64 `json:"total"`
	Covered float64 `json:"covered"`
	Skipped float64 `json:"skipped"`
	Pct     float64 `json:"pct"`
}

type vitestSummaryEntry struct {
	Lines      vitestMetric `json:"lines"`
	Statements vitestMetric `json:"statements"`
	Functions  vitestMetric `json:"functions"`
	Branches   vitestMetric `json:"branches"`
}

type metricAgg struct {
	Covered float64
	Total   float64
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "integration-upload":
			runIntegrationUpload(os.Args[2:])
			return
		case "e2e-upload":
			runE2EUpload(os.Args[2:])
			return
		case "npm-upload":
			runNPMUpload(os.Args[2:])
			return
		}
	}

	runCoverageUpload(os.Args[1:])
}

func runNPMUpload(args []string) {
	fs := flag.NewFlagSet("npm-upload", flag.ExitOnError)
	summaryPath := fs.String("vitest-summary", "", "Path to Vitest coverage summary JSON")
	apiURL := fs.String("api-url", envOrDefault("API_URL", "http://localhost:8080/v1/coverage-runs"), "Coverage API URL")
	apiKey := fs.String("api-key", os.Getenv("API_KEY"), "API key value")
	apiKeyHeader := fs.String("api-key-header", "X-API-Key", "API key header name")
	projectKey := fs.String("project-key", envOrDefault("COVERAGE_PROJECT_KEY", "github.com/arxdsilva/opencoverage"), "Project key")
	projectName := fs.String("project-name", envOrDefault("COVERAGE_PROJECT_NAME", "coverage-api"), "Project display name")
	projectGroup := fs.String("project-group", "", "Project group (optional)")
	defaultBranch := fs.String("default-branch", envOrDefault("COVERAGE_DEFAULT_BRANCH", "main"), "Default branch")
	branch := fs.String("branch", envOrDefault("COVERAGE_BRANCH", "main"), "Current branch")
	commitSHA := fs.String("commit-sha", envOrDefault("COVERAGE_COMMIT_SHA", "local"), "Commit SHA")
	author := fs.String("author", envOrDefault("COVERAGE_AUTHOR", "local"), "Author")
	triggerType := fs.String("trigger-type", "manual", "Trigger type: push|pr|manual")
	runTimestamp := fs.String("run-timestamp", time.Now().UTC().Format(time.RFC3339), "Run timestamp (RFC3339)")
	threshold := fs.Float64("threshold", 0, "Custom threshold percentage (0 to disable custom threshold)")
	metric := fs.String("metric", "lines", "Metric used for totals: lines|statements|functions|branches")
	groupBy := fs.String("group-by", "dir", "Grouping strategy: dir|file")
	pathStripPrefix := fs.String("path-strip-prefix", "", "Path prefix to remove from file keys")
	out := fs.String("out", "", "Optional path to write generated payload")
	dryRun := fs.Bool("dry-run", false, "Generate payload without upload")
	var includeGlobs globList
	var excludeGlobs globList
	fs.Var(&includeGlobs, "include-glob", "Include files matching this glob (repeatable)")
	fs.Var(&excludeGlobs, "exclude-glob", "Exclude files matching this glob (repeatable)")

	if err := fs.Parse(args); err != nil {
		exitErr("parse flags", err)
	}

	if strings.TrimSpace(*summaryPath) == "" {
		exitErr("validate input", fmt.Errorf("ERR_INPUT_SCHEMA: -vitest-summary is required"))
	}
	if _, err := time.Parse(time.RFC3339, *runTimestamp); err != nil {
		exitErr("validate input", fmt.Errorf("ERR_INPUT_SCHEMA: run timestamp must be RFC3339: %w", err))
	}

	total, packages, consideredFiles, err := parseVitestSummary(
		*summaryPath,
		*metric,
		*groupBy,
		*pathStripPrefix,
		includeGlobs,
		excludeGlobs,
	)
	if err != nil {
		exitErr("parse vitest summary", err)
	}

	var group *string
	if *projectGroup != "" {
		group = projectGroup
	}

	var thresh *float64
	if *threshold > 0 {
		thresh = threshold
	}

	slog.Info("summary", "metric", *metric, "totalCoveragePercent", total, "consideredFiles", consideredFiles, "generatedPackages", len(packages))

	payload := ingestPayload{
		ProjectKey:           *projectKey,
		ProjectName:          *projectName,
		ProjectGroup:         group,
		DefaultBranch:        *defaultBranch,
		Branch:               *branch,
		CommitSHA:            *commitSHA,
		Author:               *author,
		TriggerType:          *triggerType,
		RunTimestamp:         *runTimestamp,
		TotalCoveragePercent: total,
		ThresholdPercent:     thresh,
		Packages:             packages,
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		exitErr("marshal payload", err)
	}

	payloadOut := strings.TrimSpace(*out)
	if *dryRun && payloadOut == "" {
		payloadOut = "npm-coverage-upload.json"
	}
	if payloadOut != "" {
		if err := os.WriteFile(payloadOut, body, 0o644); err != nil {
			exitErr("write payload", err)
		}
		slog.Info("payload written", "path", payloadOut)
	}

	if *dryRun {
		fmt.Println("dry-run enabled: skipping upload")
		return
	}

	if strings.TrimSpace(*apiKey) == "" {
		exitErr("validate input", fmt.Errorf("ERR_INPUT_SCHEMA: -api-key is required (or API_KEY env var)"))
	}

	status, respBody, err := uploadPayload(*apiURL, *apiKeyHeader, *apiKey, body)
	if err != nil {
		exitErr("upload", fmt.Errorf("ERR_UPLOAD_FAILED: %w", err))
	}

	slog.Info("upload status", "status", status)
	slog.Info("upload response", "response", strings.TrimSpace(string(respBody)))

	if status >= http.StatusBadRequest {
		exitErr("upload", fmt.Errorf("ERR_UPLOAD_FAILED: server returned status %d", status))
	}
}

func runCoverageUpload(args []string) {
	fs := flag.NewFlagSet("coveragecli", flag.ExitOnError)
	coverprofile := fs.String("coverprofile", "coverage.out", "Path to go coverage profile")
	out := fs.String("out", "coverage-upload.json", "Path to output JSON payload file")
	projectKey := fs.String("project-key", "github.com/arxdsilva/opencoverage", "Project key")
	projectName := fs.String("project-name", "coverage-api", "Project display name")
	projectGroup := fs.String("project-group", "", "Project group (optional)")
	defaultBranch := fs.String("default-branch", "main", "Default branch")
	branch := fs.String("branch", envOrDefault("COVERAGE_BRANCH", "main"), "Current branch")
	commitSHA := fs.String("commit-sha", envOrDefault("COVERAGE_COMMIT_SHA", "local"), "Commit SHA")
	author := fs.String("author", envOrDefault("COVERAGE_AUTHOR", "local"), "Author")
	triggerType := fs.String("trigger-type", "manual", "Trigger type: push|pr|manual")
	threshold := fs.Float64("threshold", 0, "Custom threshold percentage (0 to disable custom threshold)")
	upload := fs.Bool("upload", false, "Upload payload to API")
	apiURL := fs.String("api-url", envOrDefault("API_URL", "http://localhost:8080/v1/coverage-runs"), "Coverage API URL")
	apiKey := fs.String("api-key", os.Getenv("API_KEY"), "API key value")
	apiKeyHeader := fs.String("api-key-header", "X-API-Key", "API key header name")
	if err := fs.Parse(args); err != nil {
		exitErr("parse flags", err)
	}

	total, packages, err := parseCoverage(*coverprofile)
	if err != nil {
		exitErr("parse coverage", err)
	}
	if len(packages) == 0 {
		exitErr("parse coverage", fmt.Errorf("no package coverage entries found"))
	}

	var group *string
	if *projectGroup != "" {
		group = projectGroup
	}

	var thresh *float64
	if *threshold > 0 {
		thresh = threshold
	}

	payload := ingestPayload{
		ProjectKey:           *projectKey,
		ProjectName:          *projectName,
		ProjectGroup:         group,
		DefaultBranch:        *defaultBranch,
		Branch:               *branch,
		CommitSHA:            *commitSHA,
		Author:               *author,
		TriggerType:          *triggerType,
		RunTimestamp:         time.Now().UTC().Format(time.RFC3339),
		TotalCoveragePercent: total,
		ThresholdPercent:     thresh,
		Packages:             packages,
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		exitErr("marshal payload", err)
	}

	if err := os.WriteFile(*out, body, 0o644); err != nil {
		exitErr("write payload file", err)
	}
	slog.Info("payload written", "path", *out)

	if !*upload {
		return
	}
	if strings.TrimSpace(*apiKey) == "" {
		exitErr("upload", fmt.Errorf("api key is required when -upload is set (use -api-key or API_KEY env var)"))
	}

	status, respBody, err := uploadPayload(*apiURL, *apiKeyHeader, *apiKey, body)
	if err != nil {
		exitErr("upload", err)
	}

	slog.Info("upload status", "status", status)
	slog.Info("upload response", "response", strings.TrimSpace(string(respBody)))
}

func runIntegrationUpload(args []string) {
	fs := flag.NewFlagSet("integration-upload", flag.ExitOnError)
	reportPath := fs.String("ginkgo-report", "", "Path to Ginkgo JSON report")
	apiURL := fs.String("api-url", envOrDefault("API_URL", "http://localhost:8080/v1/integration-test-runs"), "Integration test API URL")
	apiKey := fs.String("api-key", os.Getenv("API_KEY"), "API key value")
	apiKeyHeader := fs.String("api-key-header", "X-API-Key", "API key header name")
	projectKey := fs.String("project-key", envOrDefault("COVERAGE_PROJECT_KEY", "github.com/arxdsilva/opencoverage"), "Project key")
	projectName := fs.String("project-name", envOrDefault("COVERAGE_PROJECT_NAME", "coverage-api"), "Project display name")
	projectGroup := fs.String("project-group", "", "Project group (optional)")
	defaultBranch := fs.String("default-branch", envOrDefault("COVERAGE_DEFAULT_BRANCH", "main"), "Default branch")
	branch := fs.String("branch", envOrDefault("COVERAGE_BRANCH", "main"), "Current branch")
	commitSHA := fs.String("commit-sha", envOrDefault("COVERAGE_COMMIT_SHA", "local"), "Commit SHA")
	author := fs.String("author", envOrDefault("COVERAGE_AUTHOR", "local"), "Author")
	triggerType := fs.String("trigger-type", "manual", "Trigger type: push|pr|manual")
	environment := fs.String("environment", "", "Environment: test|stage|prod (optional)")
	runTimestamp := fs.String("run-timestamp", time.Now().UTC().Format(time.RFC3339), "Run timestamp (RFC3339)")
	if err := fs.Parse(args); err != nil {
		exitErr("parse flags", err)
	}

	if strings.TrimSpace(*reportPath) == "" {
		exitErr("validate input", fmt.Errorf("-ginkgo-report is required"))
	}
	if strings.TrimSpace(*apiKey) == "" {
		exitErr("validate input", fmt.Errorf("-api-key is required (or API_KEY env var)"))
	}
	if _, err := time.Parse(time.RFC3339, *runTimestamp); err != nil {
		exitErr("validate input", fmt.Errorf("run timestamp must be RFC3339: %w", err))
	}

	rawReport, err := os.ReadFile(*reportPath)
	if err != nil {
		exitErr("read ginkgo report", err)
	}

	var report map[string]any
	if err := json.Unmarshal(rawReport, &report); err != nil {
		exitErr("parse ginkgo report json", err)
	}

	var group *string
	if *projectGroup != "" {
		group = projectGroup
	}

	var env *string
	if *environment != "" {
		if *environment != "test" && *environment != "stage" && *environment != "prod" {
			exitErr("validate input", fmt.Errorf("-environment must be one of: test, stage, prod"))
		}
		env = environment
	}

	payload := integrationPayload{
		ProjectKey:    *projectKey,
		ProjectName:   *projectName,
		ProjectGroup:  group,
		DefaultBranch: *defaultBranch,
		Branch:        *branch,
		CommitSHA:     *commitSHA,
		Author:        *author,
		TriggerType:   *triggerType,
		RunTimestamp:  *runTimestamp,
		Environment:   env,
		GinkgoReport:  normalizeReport(report),
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		exitErr("marshal payload", err)
	}

	status, respBody, err := uploadPayload(*apiURL, *apiKeyHeader, *apiKey, body)
	if err != nil {
		exitErr("upload integration report", err)
	}

	slog.Info("upload status", "status", status)
	slog.Info("upload response", "response", strings.TrimSpace(string(respBody)))

	var parsed uploadResponse
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		delta := "-"
		if parsed.Comparison.DeltaPercent != nil {
			delta = fmt.Sprintf("%.2f", *parsed.Comparison.DeltaPercent)
		}
		slog.Info("summary", "status", parsed.Run.Status, "passRatePercent", parsed.Run.PassRatePercent, "deltaPercent", delta)
	}

	if status >= http.StatusBadRequest {
		exitErr("upload integration report", fmt.Errorf("server returned status %d", status))
	}
}

func runE2EUpload(args []string) {
	fs := flag.NewFlagSet("e2e-upload", flag.ExitOnError)
	reportPath := fs.String("e2e-report", "", "Path to e2e JSON report")
	reportType := fs.String("report-type", "playwright", "E2E report type")
	apiURL := fs.String("api-url", envOrDefault("API_URL", "http://localhost:8080/v1/e2e-test-runs"), "E2E test API URL")
	apiKey := fs.String("api-key", os.Getenv("API_KEY"), "API key value")
	apiKeyHeader := fs.String("api-key-header", "X-API-Key", "API key header name")
	projectKey := fs.String("project-key", envOrDefault("COVERAGE_PROJECT_KEY", "github.com/arxdsilva/opencoverage"), "Project key")
	projectName := fs.String("project-name", envOrDefault("COVERAGE_PROJECT_NAME", "coverage-api"), "Project display name")
	projectGroup := fs.String("project-group", "", "Project group (optional)")
	defaultBranch := fs.String("default-branch", envOrDefault("COVERAGE_DEFAULT_BRANCH", "main"), "Default branch")
	branch := fs.String("branch", envOrDefault("COVERAGE_BRANCH", "main"), "Current branch")
	commitSHA := fs.String("commit-sha", envOrDefault("COVERAGE_COMMIT_SHA", "local"), "Commit SHA")
	author := fs.String("author", envOrDefault("COVERAGE_AUTHOR", "local"), "Author")
	triggerType := fs.String("trigger-type", "manual", "Trigger type: push|pr|manual")
	environment := fs.String("environment", "", "Environment: test|stage|prod (optional)")
	runTimestamp := fs.String("run-timestamp", time.Now().UTC().Format(time.RFC3339), "Run timestamp (RFC3339)")
	platformType := fs.String("platform-type", "web", "Platform type: web|android|ios")
	if err := fs.Parse(args); err != nil {
		exitErr("parse flags", err)
	}

	if strings.TrimSpace(*reportPath) == "" {
		exitErr("validate input", fmt.Errorf("-e2e-report is required"))
	}
	if strings.TrimSpace(*apiKey) == "" {
		exitErr("validate input", fmt.Errorf("-api-key is required (or API_KEY env var)"))
	}
	if _, err := time.Parse(time.RFC3339, *runTimestamp); err != nil {
		exitErr("validate input", fmt.Errorf("run timestamp must be RFC3339: %w", err))
	}

	rawReport, err := os.ReadFile(*reportPath)
	if err != nil {
		exitErr("read e2e report", err)
	}

	var report map[string]any
	if err := json.Unmarshal(rawReport, &report); err != nil {
		exitErr("parse e2e report json", err)
	}

	var group *string
	if *projectGroup != "" {
		group = projectGroup
	}

	var env *string
	if *environment != "" {
		if *environment != "test" && *environment != "stage" && *environment != "prod" {
			exitErr("validate input", fmt.Errorf("-environment must be one of: test, stage, prod"))
		}
		env = environment
	}

	// Normalize report structure based on report type
	var normalizeReport map[string]any
	switch *reportType {
	case "playwright":
		normalizeReport = normalizePlaywrightReport(report)
	case "appium":
		normalizeReport = normalizeAppiumReport(report)
	default:
		exitErr("validate input", fmt.Errorf("unsupported report type: %s", *reportType))
	}
	normalizeReport["platformType"] = *platformType

	payload := e2ePayload{
		ProjectKey:    *projectKey,
		ProjectName:   *projectName,
		ProjectGroup:  group,
		DefaultBranch: *defaultBranch,
		Branch:        *branch,
		CommitSHA:     *commitSHA,
		Author:        *author,
		TriggerType:   *triggerType,
		RunTimestamp:  *runTimestamp,
		Environment:   env,
		TestReport:    normalizeReport,
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		exitErr("marshal payload", err)
	}

	status, respBody, err := uploadPayload(*apiURL, *apiKeyHeader, *apiKey, body)
	if err != nil {
		exitErr("upload report", err)
	}

	var parsed uploadResponse
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		delta := "-"
		if parsed.Comparison.DeltaPercent != nil {
			delta = fmt.Sprintf("%.2f", *parsed.Comparison.DeltaPercent)
		}
		slog.Info("summary", "status", parsed.Run.Status, "passRatePercent", parsed.Run.PassRatePercent, "deltaPercent", delta)
	}

	if status >= http.StatusBadRequest {
		exitErr("upload report", fmt.Errorf("server returned status %d", status))
	}
}

func normalizeReport(raw map[string]any) map[string]any {
	result := make(map[string]any)
	result["suiteDescription"] = firstString(raw, "suiteDescription", "SuiteDescription")
	result["suitePath"] = firstString(raw, "suitePath", "SuitePath")
	result["ginkgoVersion"] = firstString(raw, "ginkgoVersion", "GinkgoVersion")

	specReports := firstSlice(raw, "specReports", "SpecReports")
	normalizedSpecs := make([]map[string]any, 0, len(specReports))
	for _, item := range specReports {
		specMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		normalized := map[string]any{
			"leafNodeText":            firstString(specMap, "leafNodeText", "LeafNodeText"),
			"containerHierarchyTexts": firstSlice(specMap, "containerHierarchyTexts", "ContainerHierarchyTexts"),
			"state":                   firstString(specMap, "state", "State"),
			"runTime":                 firstFloat(specMap, "runTime", "RunTime"),
		}

		failureVal := firstMap(specMap, "failure", "Failure")
		if len(failureVal) > 0 {
			failure := map[string]any{
				"message": firstString(failureVal, "message", "Message"),
			}
			locationVal := firstMap(failureVal, "location", "Location")
			if len(locationVal) > 0 {
				failure["location"] = map[string]any{
					"fileName":   firstString(locationVal, "fileName", "FileName"),
					"lineNumber": int(firstFloat(locationVal, "lineNumber", "LineNumber")),
				}
			}
			normalized["failure"] = failure
		}

		normalizedSpecs = append(normalizedSpecs, normalized)
	}

	result["specReports"] = normalizedSpecs
	return result
}

func normalizePlaywrightReport(raw map[string]any) map[string]any {
	var suiteDescription string
	var suitePath string
	var framework_version string

	result := make(map[string]any)
	testFramework := "playwright"

	config := firstMap(raw, "config")
	suites := firstSlice(raw, "suites")
	if config != nil {
		suitePath = firstString(config, "rootDir")
		framework_version = firstString(config, "version")
	}
	if len(suites) > 0 {
		if first, ok := suites[0].(map[string]any); ok {
			suiteDescription = firstString(first, "title")
		}
	}
	result["suiteDescription"] = suiteDescription
	result["suitePath"] = suitePath
	result["reportType"] = &testFramework
	result["testFramework"] = &testFramework
	result["frameworkVersion"] = framework_version
	result["platformType"] = "web"

	// collectSpecs recursively walks Playwright's nested suite tree,
	// accumulating containerHierarchyTexts as it descends, and normalises each leaf spec
	// Suites can be nested N level deep and leaf specs can be at any level, so we need to recurse fully to find all specs and get their full hierarchy.
	var collectSpecs func(suites []any, hierarchy []string) []map[string]any
	collectSpecs = func(suites []any, hierarchy []string) []map[string]any {
		var out []map[string]any
		// iterates over all the suites branches at the current level
		for _, item := range suites {
			suiteMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			title := firstString(suiteMap, "title")
			currentHierarchy := hierarchy
			// appends hierarchy with current suite title if it exists
			// coppies all elements from hierarchy into new slice to avoid mutating the original slice in recursive calls
			if title != "" {
				currentHierarchy = append(append([]string{}, hierarchy...), title)
			}

			// Recurse into nested suites leaves first.
			// as the suites can be nested N level deep
			// uses recursive calls to collect all leaf specs
			if nested := firstSlice(suiteMap, "suites"); len(nested) > 0 {
				out = append(out, collectSpecs(nested, currentHierarchy)...)
			}

			// Normalise leaf specs within this suite.
			for _, specItem := range firstSlice(suiteMap, "specs") {
				specMap, ok := specItem.(map[string]any)
				if !ok {
					slog.Warn("skipping spec with unexpected structure", "specItem", specItem)
					continue
				}

				// Use the last test result (accounts for retries).
				tests := firstSlice(specMap, "tests")
				state := "skipped"
				runTime := 0.0
				var failureBlock map[string]any

				if len(tests) > 0 {
					if testMap, ok := tests[0].(map[string]any); ok {
						switch firstString(testMap, "status") {
						case "expected":
							state = "passed"
						case "unexpected":
							state = "failed"
						case "flaky":
							state = "flaky"
						default:
							state = "skipped"
						}

						results := firstSlice(testMap, "results")
						if len(results) > 0 {
							// Use last result (final retry).
							if lastResult, ok := results[len(results)-1].(map[string]any); ok {
								// Playwright reports duration in ms; convert to seconds.
								runTime = firstFloat(lastResult, "duration") / 1000.0

								if errVal := firstMap(lastResult, "error"); len(errVal) > 0 {
									failure := map[string]any{
										"message": stripANSI(firstString(errVal, "message")),
									}
									if locVal := firstMap(errVal, "location"); len(locVal) > 0 {
										failure["location"] = map[string]any{
											"fileName":   firstString(locVal, "file"),
											"lineNumber": int(firstFloat(locVal, "line")),
										}
									}
									failureBlock = failure
								}
							}
						}
					}
				}

				// copies currentHierarchy into new slice to avoid mutating the original slice in recursive calls
				hierarchyCopy := make([]any, len(currentHierarchy))
				for i, h := range currentHierarchy {
					hierarchyCopy[i] = h
				}

				normalized := map[string]any{
					"leafNodeText":            firstString(specMap, "title"),
					"containerHierarchyTexts": hierarchyCopy,
					"state":                   state,
					"runTime":                 runTime,
				}
				if failureBlock != nil {
					normalized["failure"] = failureBlock
				}
				out = append(out, normalized)
			}
		}
		return out
	}
	result["specReports"] = collectSpecs(suites, nil)
	return result
}

func normalizeAppiumReport(raw map[string]any) map[string]any {
	// not implemented yet
	exitErr("normalize report", fmt.Errorf("appium report normalization not implemented yet"))
	return nil
}

// stripANSI removes ANSI escape codes from a string.
// This is useful to clean up error messages from Playwright which may include ANSI codes for coloring.
func stripANSI(s string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(s, "")
}

func firstString(src map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := src[key]; ok {
			if value, ok := raw.(string); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func firstFloat(src map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if raw, ok := src[key]; ok {
			switch v := raw.(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case json.Number:
				f, err := v.Float64()
				if err == nil {
					return f
				}
			}
		}
	}
	return 0
}

func firstSlice(src map[string]any, keys ...string) []any {
	for _, key := range keys {
		if raw, ok := src[key]; ok {
			if value, ok := raw.([]any); ok {
				return value
			}
		}
	}
	return nil
}

func firstMap(src map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if raw, ok := src[key]; ok {
			if value, ok := raw.(map[string]any); ok {
				return value
			}
		}
	}
	return nil
}

func parseVitestSummary(summaryPath, metric, groupBy, pathStripPrefix string, includeGlobs, excludeGlobs []string) (float64, []packageCoverage, int, error) {
	if metric != "lines" && metric != "statements" && metric != "functions" && metric != "branches" {
		return 0, nil, 0, fmt.Errorf("ERR_INPUT_SCHEMA: unsupported metric %q", metric)
	}
	if groupBy != "dir" && groupBy != "file" {
		return 0, nil, 0, fmt.Errorf("ERR_INPUT_SCHEMA: unsupported group-by %q", groupBy)
	}

	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("ERR_INPUT_READ: %w", err)
	}

	entries := map[string]vitestSummaryEntry{}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return 0, nil, 0, fmt.Errorf("ERR_INPUT_PARSE: %w", err)
	}

	totalEntry, ok := entries["total"]
	if !ok {
		return 0, nil, 0, fmt.Errorf("ERR_INPUT_SCHEMA: total section is required")
	}

	totalMetric, ok := selectVitestMetric(totalEntry, metric)
	if !ok {
		return 0, nil, 0, fmt.Errorf("ERR_INPUT_SCHEMA: selected metric %q not found in total section", metric)
	}
	if totalMetric.Pct < 0 || totalMetric.Pct > 100 {
		return 0, nil, 0, fmt.Errorf("ERR_INPUT_SCHEMA: total %s.pct must be between 0 and 100", metric)
	}

	stripPrefix := strings.TrimSpace(pathStripPrefix)
	if stripPrefix == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			stripPrefix = cwd
		}
	}

	byGroup := make(map[string]metricAgg)
	consideredFiles := 0

	for filePath, entry := range entries {
		if filePath == "total" {
			continue
		}

		fileMetric, ok := selectVitestMetric(entry, metric)
		if !ok {
			continue
		}
		if fileMetric.Total <= 0 {
			continue
		}

		normalizedPath := normalizeCoveragePath(filePath, stripPrefix)
		if normalizedPath == "" {
			continue
		}

		if len(includeGlobs) > 0 && !matchesAnyGlob(normalizedPath, includeGlobs) {
			continue
		}
		if matchesAnyGlob(normalizedPath, excludeGlobs) {
			continue
		}

		groupKey := normalizedPath
		if groupBy == "dir" {
			groupKey = path.Dir(normalizedPath)
			if groupKey == "." || groupKey == "/" {
				groupKey = path.Base(normalizedPath)
			}
		}

		agg := byGroup[groupKey]
		agg.Covered += fileMetric.Covered
		agg.Total += fileMetric.Total
		byGroup[groupKey] = agg
		consideredFiles++
	}

	if consideredFiles == 0 || len(byGroup) == 0 {
		return 0, nil, 0, fmt.Errorf("ERR_EMPTY_DATASET: no coverage files remained after filtering")
	}

	pkgs := make([]packageCoverage, 0, len(byGroup))
	for groupKey, agg := range byGroup {
		if agg.Total <= 0 {
			continue
		}
		pct := round2((agg.Covered / agg.Total) * 100)
		if pct < 0 || pct > 100 {
			return 0, nil, 0, fmt.Errorf("ERR_INPUT_SCHEMA: computed package coverage out of range for %q", groupKey)
		}
		pkgs = append(pkgs, packageCoverage{
			ImportPath:      groupKey,
			CoveragePercent: pct,
		})
	}

	if len(pkgs) == 0 {
		return 0, nil, 0, fmt.Errorf("ERR_EMPTY_DATASET: generated packages list is empty")
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })
	return round2(totalMetric.Pct), pkgs, consideredFiles, nil
}

func selectVitestMetric(entry vitestSummaryEntry, metric string) (vitestMetric, bool) {
	switch metric {
	case "lines":
		return entry.Lines, true
	case "statements":
		return entry.Statements, true
	case "functions":
		return entry.Functions, true
	case "branches":
		return entry.Branches, true
	default:
		return vitestMetric{}, false
	}
}

func normalizeCoveragePath(filePath, stripPrefix string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
	if normalized == "" {
		return ""
	}

	normalized = path.Clean(normalized)
	if normalized == "." {
		return ""
	}

	if stripPrefix != "" {
		prefix := path.Clean(strings.ReplaceAll(strings.TrimSpace(stripPrefix), "\\", "/"))
		if prefix != "." && prefix != "" {
			trimmed := strings.TrimPrefix(normalized, prefix)
			trimmed = strings.TrimPrefix(trimmed, "/")
			if trimmed != normalized {
				normalized = trimmed
			}
		}
	}

	if len(normalized) >= 2 && normalized[1] == ':' {
		normalized = strings.TrimPrefix(normalized[2:], "/")
	}
	normalized = strings.TrimPrefix(normalized, "/")
	normalized = strings.TrimPrefix(normalized, "./")

	if normalized == "" {
		return ""
	}

	return path.Clean(normalized)
}

func matchesAnyGlob(pathValue string, globs []string) bool {
	for _, glob := range globs {
		if matchGlob(pathValue, glob) {
			return true
		}
	}
	return false
}

func matchGlob(pathValue, glob string) bool {
	pattern := regexp.QuoteMeta(strings.TrimSpace(glob))
	if pattern == "" {
		return false
	}

	pattern = strings.ReplaceAll(pattern, `\*\*`, `.*`)
	pattern = strings.ReplaceAll(pattern, `\*`, `[^/]*`)
	pattern = strings.ReplaceAll(pattern, `\?`, `[^/]`)

	re, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return false
	}

	return re.MatchString(pathValue)
}

func parseCoverage(profilePath string) (float64, []packageCoverage, error) {
	cmd := exec.Command("go", "tool", "cover", "-func", profilePath)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return 0, nil, fmt.Errorf("go tool cover failed: %s", string(ee.Stderr))
		}
		return 0, nil, err
	}

	lineRe := regexp.MustCompile(`^(.+):[0-9]+:\s+\S+\s+([0-9]+(?:\.[0-9]+)?)%$`)
	totalRe := regexp.MustCompile(`^total:\s+\(statements\)\s+([0-9]+(?:\.[0-9]+)?)%$`)

	type agg struct {
		sum   float64
		count int
	}
	byPackage := map[string]*agg{}
	var total float64
	foundTotal := false

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := totalRe.FindStringSubmatch(line); len(m) == 2 {
			t, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				return 0, nil, fmt.Errorf("parse total coverage: %w", err)
			}
			total = t
			foundTotal = true
			continue
		}

		m := lineRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		filePath := m[1]
		percent, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return 0, nil, fmt.Errorf("parse package coverage: %w", err)
		}
		pkg := path.Dir(filePath)
		if byPackage[pkg] == nil {
			byPackage[pkg] = &agg{}
		}
		byPackage[pkg].sum += percent
		byPackage[pkg].count++
	}

	if !foundTotal {
		return 0, nil, fmt.Errorf("total coverage line not found in cover output")
	}

	pkgs := make([]packageCoverage, 0, len(byPackage))
	for pkg, a := range byPackage {
		if a.count == 0 {
			continue
		}
		pkgs = append(pkgs, packageCoverage{
			ImportPath:      pkg,
			CoveragePercent: round2(a.sum / float64(a.count)),
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })

	return round2(total), pkgs, nil
}

func uploadPayload(url, apiKeyHeader, apiKey string, body []byte) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(apiKeyHeader, apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func exitErr(stage string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", stage, err)
	os.Exit(1)
}
