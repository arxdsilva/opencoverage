package mcpadapter

import (
	"fmt"
	"strings"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"
)

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
			if spec.Failure.Location == nil {
				return application.NewInvalidArgument("failure.location is required when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.location", i)})
			}
			if strings.TrimSpace(spec.Failure.Location.FileName) == "" {
				return application.NewInvalidArgument("failure.location.fileName is required when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.location.fileName", i)})
			}
			if spec.Failure.Location.LineNumber < 0 {
				return application.NewInvalidArgument("failure.location.lineNumber must be >= 0 when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.location.lineNumber", i)})
			}
		}
	}
	return nil
}
