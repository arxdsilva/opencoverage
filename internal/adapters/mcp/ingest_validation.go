package mcpadapter

import (
	"fmt"
	"strings"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"
)

func validateCoverageIngestInput(in application.IngestCoverageRunInput) error {
	if strings.TrimSpace(in.ProjectKey) == "" {
		return application.NewInvalidArgument("projectKey is required", map[string]any{"field": "projectKey"})
	}
	if strings.TrimSpace(in.Branch) == "" {
		return application.NewInvalidArgument("branch is required", map[string]any{"field": "branch"})
	}
	if strings.TrimSpace(in.CommitSHA) == "" {
		return application.NewInvalidArgument("commitSha is required", map[string]any{"field": "commitSha"})
	}
	if strings.TrimSpace(in.TriggerType) == "" {
		return application.NewInvalidArgument("triggerType is required", map[string]any{"field": "triggerType"})
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(in.RunTimestamp)); err != nil {
		return application.NewInvalidArgument("runTimestamp must be RFC3339", map[string]any{"field": "runTimestamp"})
	}
	if in.TotalCoveragePercent < 0 || in.TotalCoveragePercent > 100 {
		return application.NewInvalidArgument("totalCoveragePercent must be between 0 and 100", map[string]any{"field": "totalCoveragePercent"})
	}
	if len(in.Packages) == 0 {
		return application.NewInvalidArgument("packages is required", map[string]any{"field": "packages"})
	}
	for i, p := range in.Packages {
		if strings.TrimSpace(p.ImportPath) == "" {
			return application.NewInvalidArgument("package importPath is required", map[string]any{"field": fmt.Sprintf("packages[%d].importPath", i)})
		}
		if p.CoveragePercent < 0 || p.CoveragePercent > 100 {
			return application.NewInvalidArgument("coveragePercent must be between 0 and 100", map[string]any{"field": fmt.Sprintf("packages[%d].coveragePercent", i)})
		}
	}
	return nil
}

func validateIntegrationIngestInput(in application.IngestIntegrationRunInput) error {
	if strings.TrimSpace(in.ProjectKey) == "" {
		return application.NewInvalidArgument("projectKey is required", map[string]any{"field": "projectKey"})
	}
	if strings.TrimSpace(in.Branch) == "" {
		return application.NewInvalidArgument("branch is required", map[string]any{"field": "branch"})
	}
	if strings.TrimSpace(in.CommitSHA) == "" {
		return application.NewInvalidArgument("commitSha is required", map[string]any{"field": "commitSha"})
	}
	if strings.TrimSpace(in.TriggerType) == "" {
		return application.NewInvalidArgument("triggerType is required", map[string]any{"field": "triggerType"})
	}
	if _, err := time.Parse(time.RFC3339, strings.TrimSpace(in.RunTimestamp)); err != nil {
		return application.NewInvalidArgument("runTimestamp must be RFC3339", map[string]any{"field": "runTimestamp"})
	}
	if strings.TrimSpace(in.GinkgoReport.SuiteDescription) == "" {
		return application.NewInvalidArgument("ginkgoReport.suiteDescription is required", map[string]any{"field": "ginkgoReport.suiteDescription"})
	}
	if strings.TrimSpace(in.GinkgoReport.SuitePath) == "" {
		return application.NewInvalidArgument("ginkgoReport.suitePath is required", map[string]any{"field": "ginkgoReport.suitePath"})
	}
	if len(in.GinkgoReport.SpecReports) == 0 {
		return application.NewInvalidArgument("ginkgoReport.specReports must not be empty", map[string]any{"field": "ginkgoReport.specReports"})
	}
	for i, spec := range in.GinkgoReport.SpecReports {
		if strings.TrimSpace(spec.LeafNodeText) == "" {
			return application.NewInvalidArgument("leafNodeText is required", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].leafNodeText", i)})
		}
		if strings.TrimSpace(spec.State) == "" {
			return application.NewInvalidArgument("state is required", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].state", i)})
		}
		if spec.RunTime < 0 {
			return application.NewInvalidArgument("runTime must be >= 0", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].runTime", i)})
		}
		if strings.EqualFold(strings.TrimSpace(spec.State), "failed") {
			if spec.Failure == nil || strings.TrimSpace(spec.Failure.Message) == "" {
				return application.NewInvalidArgument("failure.message is required when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.message", i)})
			}
		}
	}
	return nil
}
