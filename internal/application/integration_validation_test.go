package application

import "testing"

func validIntegrationIngestInput() IngestIntegrationRunInput {
	return IngestIntegrationRunInput{
		ProjectKey:   "org/repo",
		Branch:       "main",
		CommitSHA:    "abc123",
		TriggerType:  "push",
		RunTimestamp: "2026-05-29T12:00:00Z",
		GinkgoReport: IngestGinkgoReportBody{
			SuiteDescription: "suite",
			SuitePath:        "integration/suite",
			SpecReports: []IngestGinkgoSpecReport{{
				LeafNodeText: "spec",
				State:        "passed",
				RunTime:      0.1,
			}},
		},
		Environment: nil,
	}
}

func TestValidateIntegrationIngestInput(t *testing.T) {
	t.Run("valid input passes", func(t *testing.T) {
		if err := ValidateIntegrationIngestInput(validIntegrationIngestInput()); err != nil {
			t.Fatalf("expected valid input, got %v", err)
		}
	})

	t.Run("invalid trigger type is rejected", func(t *testing.T) {
		in := validIntegrationIngestInput()
		in.TriggerType = "unknown"
		err := ValidateIntegrationIngestInput(in)
		if err == nil {
			t.Fatalf("expected error for invalid trigger type")
		}
		appErr, ok := err.(*AppError)
		if !ok {
			t.Fatalf("expected *AppError, got %T", err)
		}
		if field, _ := appErr.Details["field"].(string); field != "triggerType" {
			t.Fatalf("expected triggerType field, got %q", field)
		}
	})

	t.Run("failed state accepts normalized failed states and requires location", func(t *testing.T) {
		in := validIntegrationIngestInput()
		in.GinkgoReport.SpecReports = []IngestGinkgoSpecReport{{
			LeafNodeText: "spec",
			State:        "panicked",
			RunTime:      0.1,
			Failure: &IngestGinkgoFailure{
				Message: "boom",
				Location: &IngestGinkgoLocation{
					FileName:   "spec_test.go",
					LineNumber: 12,
				},
			},
		}}
		if err := ValidateIntegrationIngestInput(in); err != nil {
			t.Fatalf("expected normalized failed state to pass with full failure details, got %v", err)
		}
	})

	t.Run("failed state rejects missing failure location", func(t *testing.T) {
		in := validIntegrationIngestInput()
		in.GinkgoReport.SpecReports = []IngestGinkgoSpecReport{{
			LeafNodeText: "spec",
			State:        "failed",
			RunTime:      0.1,
			Failure: &IngestGinkgoFailure{
				Message: "boom",
			},
		}}
		err := ValidateIntegrationIngestInput(in)
		if err == nil {
			t.Fatalf("expected failure location error")
		}
		appErr, ok := err.(*AppError)
		if !ok {
			t.Fatalf("expected *AppError, got %T", err)
		}
		if field, _ := appErr.Details["field"].(string); field != "ginkgoReport.specReports[0].failure.location" {
			t.Fatalf("expected failure.location field, got %q", field)
		}
	})
}
