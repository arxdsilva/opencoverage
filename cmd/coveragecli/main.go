package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	Packages             []packageCoverage `json:"packages"`
}

func main() {
	coverprofile := flag.String("coverprofile", "coverage.out", "Path to go coverage profile")
	out := flag.String("out", "coverage-upload.json", "Path to output JSON payload file")
	projectKey := flag.String("project-key", "github.com/arxdsilva/coverage-api", "Project key")
	projectName := flag.String("project-name", "coverage-api", "Project display name")
	projectGroup := flag.String("project-group", "", "Project group (optional)")
	defaultBranch := flag.String("default-branch", "main", "Default branch")
	branch := flag.String("branch", envOrDefault("COVERAGE_BRANCH", "main"), "Current branch")
	commitSHA := flag.String("commit-sha", envOrDefault("COVERAGE_COMMIT_SHA", "local"), "Commit SHA")
	author := flag.String("author", envOrDefault("COVERAGE_AUTHOR", "local"), "Author")
	triggerType := flag.String("trigger-type", "manual", "Trigger type: push|pr|manual")
	upload := flag.Bool("upload", false, "Upload payload to API")
	apiURL := flag.String("api-url", envOrDefault("API_URL", "http://localhost:8080/v1/coverage-runs"), "Coverage API URL")
	apiKey := flag.String("api-key", os.Getenv("API_KEY"), "API key value")
	apiKeyHeader := flag.String("api-key-header", "X-API-Key", "API key header name")
	flag.Parse()

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
		Packages:             packages,
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		exitErr("marshal payload", err)
	}

	if err := os.WriteFile(*out, body, 0o644); err != nil {
		exitErr("write payload file", err)
	}
	fmt.Printf("payload written: %s\n", *out)

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
	fmt.Printf("upload status: %d\n", status)
	fmt.Printf("upload response: %s\n", strings.TrimSpace(string(respBody)))
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
