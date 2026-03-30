package application

import "github.com/arxdsilva/coverage-api/internal/domain"

type ProjectResponse struct {
	ID                     string  `json:"id"`
	ProjectKey             string  `json:"projectKey"`
	Name                   string  `json:"name,omitempty"`
	Group                  *string `json:"group,omitempty"`
	DefaultBranch          string  `json:"defaultBranch"`
	GlobalThresholdPercent float64 `json:"globalThresholdPercent"`
	Created                bool    `json:"created"`
}

type RunResponse struct {
	ID                   string  `json:"id"`
	Branch               string  `json:"branch"`
	CommitSHA            string  `json:"commitSha"`
	Author               string  `json:"author,omitempty"`
	TriggerType          string  `json:"triggerType"`
	RunTimestamp         string  `json:"runTimestamp"`
	TotalCoveragePercent float64 `json:"totalCoveragePercent"`
}

type ComparisonResponse struct {
	BaselineSource               string   `json:"baselineSource"`
	PreviousTotalCoveragePercent *float64 `json:"previousTotalCoveragePercent"`
	CurrentTotalCoveragePercent  float64  `json:"currentTotalCoveragePercent"`
	DeltaPercent                 *float64 `json:"deltaPercent"`
	Direction                    string   `json:"direction"`
	ThresholdPercent             float64  `json:"thresholdPercent"`
	ThresholdStatus              string   `json:"thresholdStatus"`
}

type PackageComparisonResponse struct {
	ImportPath              string   `json:"importPath"`
	PreviousCoveragePercent *float64 `json:"previousCoveragePercent"`
	CurrentCoveragePercent  float64  `json:"currentCoveragePercent"`
	DeltaPercent            *float64 `json:"deltaPercent"`
	Direction               string   `json:"direction"`
}

func buildComparison(current float64, previous *float64, threshold float64) ComparisonResponse {
	delta, direction := domain.CompareCoverage(current, previous)
	status := domain.EvaluateThreshold(current, threshold)
	return ComparisonResponse{
		BaselineSource:               "latest_default_branch",
		PreviousTotalCoveragePercent: previous,
		CurrentTotalCoveragePercent:  current,
		DeltaPercent:                 delta,
		Direction:                    string(direction),
		ThresholdPercent:             threshold,
		ThresholdStatus:              string(status),
	}
}

func buildPackageComparisons(current []domain.PackageCoverage, previous []domain.PackageCoverage) []PackageComparisonResponse {
	prevByPath := make(map[string]float64, len(previous))
	for _, p := range previous {
		prevByPath[p.PackageImportPath] = p.CoveragePercent
	}

	out := make([]PackageComparisonResponse, 0, len(current))
	for _, c := range current {
		var prevPtr *float64
		if prev, ok := prevByPath[c.PackageImportPath]; ok {
			p := prev
			prevPtr = &p
		}
		delta, direction := domain.CompareCoverage(c.CoveragePercent, prevPtr)
		out = append(out, PackageComparisonResponse{
			ImportPath:              c.PackageImportPath,
			PreviousCoveragePercent: prevPtr,
			CurrentCoveragePercent:  c.CoveragePercent,
			DeltaPercent:            delta,
			Direction:               string(direction),
		})
	}

	return out
}
