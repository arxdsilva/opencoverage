package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/arxdsilva/opencoverage/internal/domain"
)

func validE2EIngestInput() IngestE2ERunInput {
	return IngestE2ERunInput{
		ProjectKey:    "test/project",
		ProjectName:   "test-project",
		ProjectGroup:  StringPtr("frontend"),
		DefaultBranch: "main",
		Branch:        "main",
		CommitSHA:     "abc123",
		Author:        "John",
		TriggerType:   "pr",
		RunTimestamp:  "2026-04-01T12:00:00Z",
		Environment:   StringPtr("test"),
		TestReport: IngestReportBody{
			ReportType:                 "playwright",
			TestFramework:              "playwright",
			FrameworkVersion:           "1.0.0",
			PlatformType:               "web",
			SuiteDescription:           "E2E Tests",
			SuitePath:                  "tests/e2e",
			SuiteSucceeded:             true,
			SpecialSuiteFailureReasons: []string{"Auth Failure"},
			SpecReports: []IngestSpecReport{
				{
					LeafNodeText:            "Auth",
					ContainerHierarchyTexts: []string{"Auth Failure"},
					State:                   "Passed",
					RunTime:                 2.00,
					Failure: &IngestTestFailure{
						Message: "Auth Failure",
						Location: &IngestTestLocation{
							FileName:   "Auth",
							LineNumber: 10,
						},
					},
				},
			},
		},
	}
}

func StringPtr(s string) *string {
	return &s
}

func TestIngestE2ERunUseCaseExecute(t *testing.T) {
	runInput := IngestE2ERunInput{
		ProjectKey:    "test/project",
		ProjectName:   "test-project",
		ProjectGroup:  StringPtr("frontend"),
		DefaultBranch: "main",
		Branch:        "main",
		CommitSHA:     "abc123",
		Author:        "John",
		TriggerType:   "pr",
		RunTimestamp:  "2026-04-01T12:00:00Z",
		Environment:   StringPtr("test"),
		TestReport: IngestReportBody{
			ReportType:                 "playwright",
			TestFramework:              "playwright",
			FrameworkVersion:           "1.0.0",
			PlatformType:               "web",
			SuiteDescription:           "E2E Tests",
			SuitePath:                  "tests/e2e",
			SuiteSucceeded:             true,
			SpecialSuiteFailureReasons: []string{"Auth Failure"},
			SpecReports: []IngestSpecReport{
				{
					LeafNodeText:            "Auth",
					ContainerHierarchyTexts: []string{"Auth Failure"},
					State:                   "Passed",
					RunTime:                 2.00,
					SpecType:                "happyPath",
					Failure: &IngestTestFailure{
						Message: "Auth Failure",
						Location: &IngestTestLocation{
							FileName:   "Auth",
							LineNumber: 10,
						},
					},
				},
			},
		},
	}
	t.Run("Execute run with correct data", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			existing: &domain.Project{
				ID:                     "proj1",
				ProjectKey:             "test/project",
				Name:                   "test-project",
				Group:                  StringPtr("frontend"),
				DefaultBranch:          "main",
				GlobalThresholdPercent: 80,
				CreatedAt:              time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:              time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			},
		}
		runRepo := &stubE2ETestRunRepository{}
		specResultRepo := &stubE2ESpecResultRepository{}
		transactionManager := &stubTransactionManager{}
		id := &stubIDGenerator{}
		clock := &stubClock{}
		uc := NewIngestE2ERunUseCase(projectRepo, runRepo, specResultRepo, transactionManager, id, clock)

		out, err := uc.Execute(context.Background(), runInput)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if out.Run.ID == "" {
			t.Fatalf("expected run ID to be set")
		}
		if out.Project.ProjectKey != "test/project" {
			t.Fatalf("expected project key to be test/project, got %s", out.Project.ProjectKey)
		}
		if out.Project.Name != "test-project" {
			t.Fatalf("expected project name to be test-project, got %s", out.Project.Name)
		}
		if out.Project.Group == nil || *out.Project.Group != "frontend" {
			t.Fatalf("expected project group to be frontend, got %v", out.Project.Group)
		}
		if out.Project.DefaultBranch != "main" {
			t.Fatalf("expected default branch to be main, got %s", out.Project.DefaultBranch)
		}
		if out.Run.Branch != "main" {
			t.Fatalf("expected run branch to be main, got %s", out.Run.Branch)
		}
		if out.Run.CommitSHA != "abc123" {
			t.Fatalf("expected commit SHA to be abc123, got %s", out.Run.CommitSHA)
		}
		if out.Run.Author != "John" {
			t.Fatalf("expected author to be John, got %s", out.Run.Author)
		}
		if out.Run.Branch != "main" {
			t.Fatalf("expected branch to be main, got %s", out.Run.Branch)
		}
		if out.Run.Branch != "main" {
			t.Fatalf("expected branch to be main, got %s", out.Run.Branch)
		}
		if out.Run.CommitSHA != "abc123" {
			t.Fatalf("expected commit SHA to be abc123, got %s", out.Run.CommitSHA)
		}
		if out.Run.Author != "John" {
			t.Fatalf("expected author to be John, got %s", out.Run.Author)
		}
		if out.Run.TriggerType != "pr" {
			t.Fatalf("expected trigger type to be pr, got %s", out.Run.TriggerType)
		}
		if out.Run.Environment == nil || *out.Run.Environment != "test" {
			t.Fatalf("expected environment to be test, got %v", out.Run.Environment)
		}
		if out.Run.TotalSpecs != 1 {
			t.Fatalf("expected total specs to be 1, got %d", out.Run.TotalSpecs)
		}
		if out.Run.PassedSpecs != 1 {
			t.Fatalf("expected passed specs to be 1, got %d", out.Run.PassedSpecs)
		}
		if out.Run.FailedSpecs != 0 {
			t.Fatalf("expected failed specs to be 0, got %d", out.Run.FailedSpecs)
		}
		if out.Run.FlakedSpecs != 0 {
			t.Fatalf("expected flaked specs to be 0, got %d", out.Run.FlakedSpecs)
		}
		if out.Run.SkippedSpecs != 0 {
			t.Fatalf("expected skipped specs to be 0, got %d", out.Run.SkippedSpecs)
		}
		if out.Run.PassRatePercent != 100 {
			t.Fatalf("expected pass rate percent to be 100, got %f", out.Run.PassRatePercent)
		}
		if out.Run.Status != "passed" {
			t.Fatalf("expected run status to be passed, got %s", out.Run.Status)
		}
		if out.Comparison.BaselineSource != "latest_default_branch" {
			t.Fatalf("expected baseline source to be latest_default_branch, got %s", out.Comparison.BaselineSource)
		}
		if out.Comparison.CurrentPassRatePercent != 100 {
			t.Fatalf("expected current pass rate percent to be 100, got %f", out.Comparison.CurrentPassRatePercent)
		}
		if out.Comparison.PreviousPassRatePercent != nil {
			t.Fatalf("expected previous pass rate percent to be nil, got %f", *out.Comparison.PreviousPassRatePercent)
		}
		if out.Comparison.DeltaPercent != nil {
			t.Fatalf("expected delta percent to be nil, got %f", *out.Comparison.DeltaPercent)
		}
		if out.Comparison.Direction != "new" {
			t.Fatalf("expected direction to be new, got %s", out.Comparison.Direction)
		}

	})

	t.Run("Execute return an error when validation fails", func(t *testing.T) {
		projectRepo := &stubProjectRepository{}
		runRepo := &stubE2ETestRunRepository{}
		specResultRepo := &stubE2ESpecResultRepository{}
		transactionManager := &stubTransactionManager{}
		id := &stubIDGenerator{}
		clock := &stubClock{}
		uc := NewIngestE2ERunUseCase(projectRepo, runRepo, specResultRepo, transactionManager, id, clock)
		_, err := uc.Execute(context.Background(), IngestE2ERunInput{})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInvalidArgument {
			t.Fatalf("expected code to be invalid_argument, got %s", appErr.Code)
		}
	})

	t.Run("Execute return an error when run time parse fails", func(t *testing.T) {
		ProjectRepo := &stubProjectRepository{existing: &domain.Project{ID: "proj1"}, err: nil}
		runRepo := &stubE2ETestRunRepository{}
		specResultRepo := &stubE2ESpecResultRepository{}
		transactionManager := &stubTransactionManager{}
		id := &stubIDGenerator{}
		clock := &stubClock{}
		uc := NewIngestE2ERunUseCase(ProjectRepo, runRepo, specResultRepo, transactionManager, id, clock)
		input := runInput
		input.RunTimestamp = "invalid-timestamp"
		_, err := uc.Execute(context.Background(), input)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
	})

	t.Run("Execute return an error when create run fails", func(t *testing.T) {
		projectRepo := &stubProjectRepository{existing: nil, err: fmt.Errorf("failed to create project"), project: domain.Project{}}
		runRepo := &stubE2ETestRunRepository{}
		specResultRepo := &stubE2ESpecResultRepository{}
		transactionManager := &stubTransactionManager{}
		id := &stubIDGenerator{}
		clock := &stubClock{}
		uc := NewIngestE2ERunUseCase(projectRepo, runRepo, specResultRepo, transactionManager, id, clock)
		_, err := uc.Execute(context.Background(), runInput)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInternal {
			t.Fatalf("expected code to be internal, got %s", appErr.Code)
		}
	})

	t.Run("Execute return and error when get latest run by branch fails", func(t *testing.T) {
		ProjectRepo := &stubProjectRepository{existing: &domain.Project{ID: "proj1"}, err: nil}
		runRepo := &stubE2ETestRunRepository{latestByBranchErr: fmt.Errorf("failed to get latest run by branch")}
		specResultRepo := &stubE2ESpecResultRepository{}
		transactionManager := &stubTransactionManager{}
		id := &stubIDGenerator{}
		clock := &stubClock{}
		uc := NewIngestE2ERunUseCase(ProjectRepo, runRepo, specResultRepo, transactionManager, id, clock)
		_, err := uc.Execute(context.Background(), runInput)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInternal {
			t.Fatalf("expected code to be internal, got %s", appErr.Code)
		}
	})
}

func TestIngestE2ERunResolveOrCreateE2EProject(t *testing.T) {
	t.Run("Resolve existing project", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			existing: &domain.Project{
				ID:                     "proj1",
				ProjectKey:             "test/project",
				Name:                   "test-project",
				Group:                  StringPtr("frontend"),
				DefaultBranch:          "main",
				GlobalThresholdPercent: 80,
				CreatedAt:              time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:              time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			},
		}
		uc := NewIngestE2ERunUseCase(projectRepo, nil, nil, nil, nil, nil)
		project, created, err := uc.resolveOrCreateE2EProject(context.Background(), IngestE2ERunInput{ProjectKey: "test/project"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if project != *projectRepo.existing {
			t.Fatalf("expected project to be %+v, got %+v", *projectRepo.existing, project)
		}
		if created {
			t.Fatalf("expected created to be false, got true")
		}
	})
	t.Run("Create project when do not exist", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			existing: nil,
			err:      nil,
		}
		uc := NewIngestE2ERunUseCase(projectRepo, nil, nil, nil, &stubIDGenerator{}, &stubClock{})
		project, created, err := uc.resolveOrCreateE2EProject(context.Background(), IngestE2ERunInput{
			ProjectKey:    "test/project",
			ProjectName:   "test-project",
			ProjectGroup:  StringPtr("frontend"),
			DefaultBranch: "main",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !created {
			t.Fatalf("expected created to be true, got false")
		}
		if project.ProjectKey != "test/project" {
			t.Fatalf("expected project key test/project, got %s", project.ProjectKey)
		}
		if project.Name != "test-project" {
			t.Fatalf("expected name test-project, got %s", project.Name)
		}
		if project.Group == nil || *project.Group != "frontend" {
			t.Fatalf("expected group frontend, got %v", project.Group)
		}
		if project.DefaultBranch != "main" {
			t.Fatalf("expected default branch main, got %s", project.DefaultBranch)
		}
		if project.ID == "" {
			t.Fatal("expected project ID to be set")
		}
	})
	t.Run("Returns error when failed to load prpoject", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			existing: nil,
			err:      fmt.Errorf("failed to load project"),
		}
		uc := NewIngestE2ERunUseCase(projectRepo, nil, nil, nil, &stubIDGenerator{}, &stubClock{})
		_, _, err := uc.resolveOrCreateE2EProject(context.Background(), IngestE2ERunInput{ProjectKey: "test/project"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
	t.Run("Returns error when failed to create a project", func(t *testing.T) {
		projectRep := &stubProjectRepository{
			existing: nil,
			err:      fmt.Errorf("failed to create project"),
		}
		uc := NewIngestE2ERunUseCase(projectRep, nil, nil, nil, &stubIDGenerator{}, &stubClock{})
		_, _, err := uc.resolveOrCreateE2EProject(context.Background(), IngestE2ERunInput{
			ProjectKey:    "test/project",
			ProjectName:   "test-project",
			ProjectGroup:  StringPtr("frontend"),
			DefaultBranch: "main",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestIngestE2ERunBuildE2EEntities(t *testing.T) {
	runInput := IngestE2ERunInput{
		ProjectKey:    "test/project",
		ProjectName:   "test-project",
		ProjectGroup:  StringPtr("frontend"),
		DefaultBranch: "main",
		Branch:        "main",
		CommitSHA:     "abc123",
		Author:        "John",
		TriggerType:   "pr",
		RunTimestamp:  "2026-04-01T12:00:00Z",
		Environment:   StringPtr("test"),
		TestReport: IngestReportBody{
			ReportType:                 "playwright",
			TestFramework:              "playwright",
			FrameworkVersion:           "1.0.0",
			PlatformType:               "web",
			SuiteDescription:           "E2E Tests",
			SuitePath:                  "tests/e2e",
			SuiteSucceeded:             true,
			SpecialSuiteFailureReasons: []string{"Auth Failure"},
			SpecReports: []IngestSpecReport{
				{
					LeafNodeText:            "Auth",
					ContainerHierarchyTexts: []string{"Auth Failure"},
					State:                   "Passed",
					RunTime:                 2.00,
					Failure: &IngestTestFailure{
						Message: "Auth Failure",
						Location: &IngestTestLocation{
							FileName:   "Auth",
							LineNumber: 10,
						},
					},
				},
			},
		},
	}

	t.Run("Build the e2e entities with correct data", func(t *testing.T) {
		projectRep := &stubProjectRepository{
			existing: nil,
			project: domain.Project{
				ProjectKey:    "test/project",
				Name:          "test-project",
				Group:         StringPtr("frontend"),
				DefaultBranch: "main",
				ID:            "project-id",
			},
		}
		uc := NewIngestE2ERunUseCase(projectRep, nil, nil, nil, &stubIDGenerator{}, &stubClock{})
		run, specs := uc.buildE2EEntities("project-id", runInput, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))

		if run.ProjectID != "project-id" {
			t.Fatalf("expected project ID project-id, got %s", run.ProjectID)
		}
		if run.Branch != "main" {
			t.Fatalf("expected branch main, got %s", run.Branch)
		}
		if run.CommitSHA != "abc123" {
			t.Fatalf("expected commit SHA abc123, got %s", run.CommitSHA)
		}
		if run.Author != "John" {
			t.Fatalf("expected author John, got %s", run.Author)
		}
		if run.TriggerType != "pr" {
			t.Fatalf("expected trigger type pr, got %s", run.TriggerType)
		}
		if run.FrameworkVersion != "1.0.0" {
			t.Fatalf("expected framework version 1.0.0, got %s", run.FrameworkVersion)
		}
		if run.TestFramework != "playwright" {
			t.Fatalf("expected test framework playwright, got %s", run.TestFramework)
		}
		if run.PlatformType != "web" {
			t.Fatalf("expected platform type web, got %s", run.PlatformType)
		}
		if run.SuiteDescription != "E2E Tests" {
			t.Fatalf("expected suite description E2E Tests, got %s", run.SuiteDescription)
		}
		if run.SuitePath != "tests/e2e" {
			t.Fatalf("expected suite path tests/e2e, got %s", run.SuitePath)
		}
		if run.TotalSpecs != 1 {
			t.Fatalf("expected total specs 1, got %d", run.TotalSpecs)
		}
		if run.PassedSpecs != 1 {
			t.Fatalf("expected passed specs 1, got %d", run.PassedSpecs)
		}
		if run.FailedSpecs != 0 {
			t.Fatalf("expected failed specs 0, got %d", run.FailedSpecs)
		}
		if run.Status != domain.E2ERunStatusPassed {
			t.Fatalf("expected status passed, got %s", run.Status)
		}
		if run.DurationMS != 2000 {
			t.Fatalf("expected duration 2000ms, got %d", run.DurationMS)
		}
		if run.ID == "" {
			t.Fatal("expected run ID to be set")
		}

		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].LeafNodeText != "Auth" {
			t.Fatalf("expected leaf node text Auth, got %s", specs[0].LeafNodeText)
		}
		if specs[0].SpecPath != "Auth Failure > Auth" {
			t.Fatalf("expected spec path 'Auth Failure > Auth', got %s", specs[0].SpecPath)
		}
		if specs[0].State != domain.E2ESpecStatePassed {
			t.Fatalf("expected state passed, got %s", specs[0].State)
		}
		if specs[0].DurationMS != 2000 {
			t.Fatalf("expected duration 2000ms, got %d", specs[0].DurationMS)
		}
		if specs[0].E2ETestRunID != run.ID {
			t.Fatalf("expected spec run ID %s, got %s", run.ID, specs[0].E2ETestRunID)
		}
	})

	t.Run("Build entities with explicit specType", func(t *testing.T) {
		uc := NewIngestE2ERunUseCase(nil, nil, nil, nil, &stubIDGenerator{}, &stubClock{})
		input := runInput
		input.TestReport.SpecReports = []IngestSpecReport{
			{
				LeafNodeText: "Error handling",
				State:        "Passed",
				SpecType:     "negativePath",
				RunTime:      1.0,
			},
		}
		_, specs := uc.buildE2EEntities("project-id", input, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC))
		if len(specs) != 1 {
			t.Fatalf("expected 1 spec, got %d", len(specs))
		}
		if specs[0].SpecType != "negativePath" {
			t.Fatalf("expected spec type negativePath, got %s", specs[0].SpecType)
		}
	})
}

func TestValidateE2EIngestInput(t *testing.T) {
	test := []struct {
		name      string
		mutate    func(in *IngestE2ERunInput)
		wantErr   bool
		wantField string
	}{
		{
			name:    "valid input passes",
			mutate:  func(in *IngestE2ERunInput) {},
			wantErr: false,
		},
		{
			name:      "empty projectKey returns error",
			mutate:    func(in *IngestE2ERunInput) { in.ProjectKey = "" },
			wantErr:   true,
			wantField: "projectKey",
		},
		{
			name:      "empty branch returns error",
			mutate:    func(in *IngestE2ERunInput) { in.Branch = "" },
			wantErr:   true,
			wantField: "branch",
		},
		{
			name:      "empty commitSha returns error",
			mutate:    func(in *IngestE2ERunInput) { in.CommitSHA = "" },
			wantErr:   true,
			wantField: "commitSha",
		},
		{
			name:      "invalid triggerType returns error",
			mutate:    func(in *IngestE2ERunInput) { in.TriggerType = "invalid" },
			wantErr:   true,
			wantField: "triggerType",
		},
		{
			name:      "empty frameworkVersion returns error",
			mutate:    func(in *IngestE2ERunInput) { in.TestReport.FrameworkVersion = "" },
			wantErr:   true,
			wantField: "testReport.frameworkVersion",
		},
		{
			name:      "empty testFramework returns error",
			mutate:    func(in *IngestE2ERunInput) { in.TestReport.TestFramework = "" },
			wantErr:   true,
			wantField: "testReport.testFramework",
		},
		{
			name:      "empty platformType returns error",
			mutate:    func(in *IngestE2ERunInput) { in.TestReport.PlatformType = "" },
			wantErr:   true,
			wantField: "testReport.platformType",
		},
		{
			name:      "empty specReports returns error",
			mutate:    func(in *IngestE2ERunInput) { in.TestReport.SpecReports = nil },
			wantErr:   true,
			wantField: "testReport.specReports",
		},
		{
			name: "invalid spec state returns error",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].State = "invalid"
			},
			wantErr:   true,
			wantField: "testReport.specReports[0].state",
		},
		{
			name: "negative runTime returns error",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].RunTime = -1
			},
			wantErr:   true,
			wantField: "testReport.specReports[0].runTime",
		},
		{
			name: "failed state without failure message returns error",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].State = "failed"
				in.TestReport.SpecReports[0].Failure = nil
			},
			wantErr:   true,
			wantField: "testReport.specReports[0].failure.message",
		},
		{
			name: "invalid specType returns error",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].SpecType = "invalid"
			},
			wantErr:   true,
			wantField: "testReport.specReports[0].specType",
		},
		{
			name: "happyPath specType passes validation",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].SpecType = "happyPath"
			},
			wantErr: false,
		},
		{
			name: "negativePath specType passes validation",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].SpecType = "negativePath"
			},
			wantErr: false,
		},
		{
			name: "empty specType passes validation",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].SpecType = ""
			},
			wantErr: false,
		},
		{
			name: "setup specType passes validation",
			mutate: func(in *IngestE2ERunInput) {
				in.TestReport.SpecReports[0].SpecType = "setup"
			},
			wantErr: false,
		},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			in := validE2EIngestInput()
			tt.mutate(&in)
			err := validateE2EIngestInput(in)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr && tt.wantField != "" {
				appErr, ok := err.(*AppError)
				if !ok {
					t.Fatalf("expected *AppError, got %T", err)
				}
				field, _ := appErr.Details["field"].(string)
				if field != tt.wantField {
					t.Errorf("expected field=%q, got %q", tt.wantField, field)
				}
			}
		})
	}
}

func TestListE2ERunsExecute(t *testing.T) {
	listed := []domain.E2ETestRun{
		{
			ID:               "run1",
			ProjectID:        "proj1",
			Branch:           "main",
			CommitSHA:        "abc123",
			Author:           "John",
			TriggerType:      "pr",
			RunTimestamp:     time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			FrameworkVersion: "1.0.0",
			TestFramework:    "playwright",
			PlatformType:     "web",
			SuiteDescription: "e2e test",
			SuitePath:        "tests/e2e",
			TotalSpecs:       10,
			PassedSpecs:      10,
			FailedSpecs:      0,
			FlakedSpecs:      0,
			SkippedSpecs:     0,
			Interrupted:      false,
			TimedOut:         false,
			DurationMS:       2000,
			Status:           "passed",
			Environment:      StringPtr("test"),
			CreatedAt:        time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:               "run2",
			ProjectID:        "proj2",
			Branch:           "main",
			CommitSHA:        "abc123",
			Author:           "Alex",
			TriggerType:      "pr",
			RunTimestamp:     time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			FrameworkVersion: "1.0.0",
			TestFramework:    "playwright",
			PlatformType:     "web",
			SuiteDescription: "e2e test",
			SuitePath:        "tests/e2e",
			TotalSpecs:       10,
			PassedSpecs:      10,
			FailedSpecs:      0,
			FlakedSpecs:      0,
			SkippedSpecs:     0,
			Interrupted:      false,
			TimedOut:         false,
			DurationMS:       2000,
			Status:           "passed",
			Environment:      StringPtr("test"),
			CreatedAt:        time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:               "run3",
			ProjectID:        "proj3",
			Branch:           "main",
			CommitSHA:        "abc123",
			Author:           "Alice",
			TriggerType:      "pr",
			RunTimestamp:     time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			FrameworkVersion: "1.0.0",
			TestFramework:    "playwright",
			PlatformType:     "web",
			SuiteDescription: "e2e test",
			SuitePath:        "tests/e2e",
			TotalSpecs:       10,
			PassedSpecs:      9,
			FailedSpecs:      1,
			FlakedSpecs:      0,
			SkippedSpecs:     0,
			Interrupted:      false,
			TimedOut:         false,
			DurationMS:       2000,
			Status:           "failed",
			Environment:      StringPtr("test"),
			CreatedAt:        time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)},
	}
	t.Run("List runs with correct data", func(t *testing.T) {
		from := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC).Add(-time.Hour)
		to := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC).Add(time.Hour)
		runRepo := &stubE2ETestRunRepository{listed: listed, listTotal: 3}
		lc := NewListE2ERunsUseCase(runRepo)
		out, err := lc.Execute(context.Background(), ListE2ERunsInput{
			ProjectID:   "proj1",
			Branch:      "main",
			Status:      "passed",
			Environment: "test",
			SpecType:    "happyPath",
			From:        &from,
			To:          &to,
			Page:        1,
			PageSize:    3,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(out.Items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(out.Items))
		}
		if out.Items[0].ID != "run1" {
			t.Fatalf("expected first item ID run1, got %s", out.Items[0].ID)
		}
		if out.Items[0].Branch != "main" {
			t.Fatalf("expected branch main, got %s", out.Items[0].Branch)
		}
		if out.Items[0].CommitSHA != "abc123" {
			t.Fatalf("expected commit SHA abc123, got %s", out.Items[0].CommitSHA)
		}
		if out.Items[0].RunTimestamp != "2026-04-01T12:00:00Z" {
			t.Fatalf("expected run timestamp 2026-04-01T12:00:00Z, got %s", out.Items[0].RunTimestamp)
		}
		if out.Items[0].Environment == nil || *out.Items[0].Environment != "test" {
			t.Fatalf("expected environment test, got %v", out.Items[0].Environment)
		}
		if out.Items[0].TotalSpecs != 10 {
			t.Fatalf("expected total specs 10, got %d", out.Items[0].TotalSpecs)
		}
		if out.Items[0].FailedSpecs != 0 {
			t.Fatalf("expected failed specs 0, got %d", out.Items[0].FailedSpecs)
		}
		if out.Items[0].PassRatePercent != 100 {
			t.Fatalf("expected pass rate percent 100, got %f", out.Items[0].PassRatePercent)
		}
		if out.Items[0].Status != "passed" {
			t.Fatalf("expected status passed, got %s", out.Items[0].Status)
		}
		if out.Items[2].ID != "run3" {
			t.Fatalf("expected third item ID run3, got %s", out.Items[2].ID)
		}
		if out.Items[2].FailedSpecs != 1 {
			t.Fatalf("expected failed specs 1, got %d", out.Items[2].FailedSpecs)
		}
		if out.Items[2].PassRatePercent != 90 {
			t.Fatalf("expected pass rate percent 90, got %f", out.Items[2].PassRatePercent)
		}
		if out.Items[2].Status != "failed" {
			t.Fatalf("expected status failed, got %s", out.Items[2].Status)
		}
		if out.Pagination.Page != 1 {
			t.Fatalf("expected page 1, got %d", out.Pagination.Page)
		}
		if out.Pagination.PageSize != 3 {
			t.Fatalf("expected page size 3, got %d", out.Pagination.PageSize)
		}
		if out.Pagination.TotalItems != 3 {
			t.Fatalf("expected total items 3, got %d", out.Pagination.TotalItems)
		}
		if out.Pagination.TotalPages != 1 {
			t.Fatalf("expected total pages 1, got %d", out.Pagination.TotalPages)
		}
	})
	t.Run("Returns an error when status is not passed or failed", func(t *testing.T) {
		runRepo := &stubE2ETestRunRepository{}
		uc := NewListE2ERunsUseCase(runRepo)
		_, err := uc.Execute(context.Background(), ListE2ERunsInput{
			ProjectID: "proj1",
			Status:    "invalid",
			SpecType:  "happyPath",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInvalidArgument {
			t.Fatalf("expected code to be INVALID_ARGUMENT, got %s", appErr.Code)
		}
		field, _ := appErr.Details["field"].(string)
		if field != "status" {
			t.Fatalf("expected field status, got %s", field)
		}
	})
	t.Run("Returns an error when environment is not test, stage or prod", func(t *testing.T) {
		runRepo := &stubE2ETestRunRepository{}
		uc := NewListE2ERunsUseCase(runRepo)
		_, err := uc.Execute(context.Background(), ListE2ERunsInput{
			ProjectID:   "proj1",
			Environment: "invalid",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInvalidArgument {
			t.Fatalf("expected code to be INVALID_ARGUMENT, got %s", appErr.Code)
		}
		field, _ := appErr.Details["field"].(string)
		if field != "environment" {
			t.Fatalf("expected field environment, got %s", field)
		}
	})
	t.Run("Returns an error when specType is invalid", func(t *testing.T) {
		runRepo := &stubE2ETestRunRepository{}
		uc := NewListE2ERunsUseCase(runRepo)
		_, err := uc.Execute(context.Background(), ListE2ERunsInput{
			ProjectID: "proj1",
			SpecType:  "invalid",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInvalidArgument {
			t.Fatalf("expected code to be INVALID_ARGUMENT, got %s", appErr.Code)
		}
		field, _ := appErr.Details["field"].(string)
		if field != "specType" {
			t.Fatalf("expected field specType, got %s", field)
		}
	})
	t.Run("Returns an error when fails to list the runs", func(t *testing.T) {
		runRepo := &stubE2ETestRunRepository{listErr: fmt.Errorf("db error")}
		uc := NewListE2ERunsUseCase(runRepo)
		_, err := uc.Execute(context.Background(), ListE2ERunsInput{
			ProjectID: "proj1",
		})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInternal {
			t.Fatalf("expected code to be INTERNAL, got %s", appErr.Code)
		}
	})
}

func TestGetLatestE2EComparisonExecute(t *testing.T) {
	t.Run("Execute successfully with baseline comparison", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			project: domain.Project{
				ID:                     "proj1",
				ProjectKey:             "test/project",
				Name:                   "test-project",
				Group:                  StringPtr("frontend"),
				DefaultBranch:          "main",
				GlobalThresholdPercent: 80,
				CreatedAt:              time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:              time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			},
		}
		runRepo := &stubE2ETestRunRepository{
			latestByProject: &domain.E2ETestRun{
				ID:               "run1",
				ProjectID:        "proj1",
				Branch:           "feature",
				CommitSHA:        "abc123",
				Author:           "John",
				TriggerType:      "pr",
				RunTimestamp:     time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
				FrameworkVersion: "1.0.0",
				TestFramework:    "playwright",
				PlatformType:     "web",
				SuiteDescription: "e2e test",
				SuitePath:        "tests/e2e",
				TotalSpecs:       10,
				PassedSpecs:      9,
				FailedSpecs:      1,
				FlakedSpecs:      0,
				SkippedSpecs:     0,
				Interrupted:      false,
				TimedOut:         false,
				DurationMS:       2000,
				Status:           domain.E2ERunStatusFailed,
				Environment:      StringPtr("test"),
				CreatedAt:        time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
			},
			latestByBranch: &domain.E2ETestRun{
				ID:               "run2",
				ProjectID:        "proj1",
				Branch:           "main",
				CommitSHA:        "def456",
				Author:           "John",
				TriggerType:      "push",
				RunTimestamp:     time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
				FrameworkVersion: "1.0.0",
				TestFramework:    "playwright",
				PlatformType:     "web",
				SuiteDescription: "e2e test",
				SuitePath:        "tests/e2e",
				TotalSpecs:       10,
				PassedSpecs:      10,
				FailedSpecs:      0,
				FlakedSpecs:      0,
				SkippedSpecs:     0,
				Interrupted:      false,
				TimedOut:         false,
				DurationMS:       2000,
				Status:           domain.E2ERunStatusPassed,
				Environment:      StringPtr("test"),
				CreatedAt:        time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
			},
		}
		specRepo := &stubE2ESpecResultRepository{
			failedByRunID: []domain.E2ESpecResult{
				{
					ID:             "spec1",
					E2ETestRunID:   "run1",
					SpecPath:       "tests/e2e/auth.spec.ts",
					LeafNodeText:   "should login",
					State:          domain.E2ESpecStateFailed,
					DurationMS:     1000,
					FailureMessage: StringPtr("auth failed"),
				},
			},
		}
		uc := NewGetLatestE2EComparisonUseCase(projectRepo, runRepo, specRepo)

		out, err := uc.Execute(context.Background(), "proj1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if out.Project.ID != "proj1" {
			t.Fatalf("expected project ID proj1, got %s", out.Project.ID)
		}
		if out.Project.ProjectKey != "test/project" {
			t.Fatalf("expected project key test/project, got %s", out.Project.ProjectKey)
		}
		if out.Run.ID != "run1" {
			t.Fatalf("expected run ID run1, got %s", out.Run.ID)
		}
		if out.Run.PassRatePercent != 90 {
			t.Fatalf("expected pass rate 90, got %f", out.Run.PassRatePercent)
		}
		if out.Run.Status != "failed" {
			t.Fatalf("expected status failed, got %s", out.Run.Status)
		}
		if out.Comparison.BaselineSource != "latest_default_branch" {
			t.Fatalf("expected baseline source latest_default_branch, got %s", out.Comparison.BaselineSource)
		}
		if out.Comparison.CurrentPassRatePercent != 90 {
			t.Fatalf("expected current pass rate 90, got %f", out.Comparison.CurrentPassRatePercent)
		}
		if out.Comparison.PreviousPassRatePercent == nil {
			t.Fatal("expected previous pass rate to be set")
		}
		if *out.Comparison.PreviousPassRatePercent != 100 {
			t.Fatalf("expected previous pass rate 100, got %f", *out.Comparison.PreviousPassRatePercent)
		}
		if out.Comparison.DeltaPercent == nil {
			t.Fatal("expected delta to be set")
		}
		if *out.Comparison.DeltaPercent != -10 {
			t.Fatalf("expected delta -10, got %f", *out.Comparison.DeltaPercent)
		}
		if out.Comparison.Direction != "down" {
			t.Fatalf("expected direction down, got %s", out.Comparison.Direction)
		}
		if out.Comparison.NewFailures != 0 {
			t.Fatalf("expected new failures 0, got %d", out.Comparison.NewFailures)
		}
		if out.Comparison.ResolvedFailures != 0 {
			t.Fatalf("expected resolved failures 0, got %d", out.Comparison.ResolvedFailures)
		}
		if len(out.FailedSpecs) != 1 {
			t.Fatalf("expected 1 failed spec, got %d", len(out.FailedSpecs))
		}
		if out.FailedSpecs[0].SpecPath != "tests/e2e/auth.spec.ts" {
			t.Fatalf("expected spec path tests/e2e/auth.spec.ts, got %s", out.FailedSpecs[0].SpecPath)
		}
		if out.FailedSpecs[0].FailureMessage != "auth failed" {
			t.Fatalf("expected failure message auth failed, got %s", out.FailedSpecs[0].FailureMessage)
		}
	})
	t.Run("Return an error when failure to get project", func(t *testing.T) {
		projectRepo := &stubProjectRepository{err: fmt.Errorf("db error")}
		runRepo := &stubE2ETestRunRepository{}
		specRepo := &stubE2ESpecResultRepository{}
		uc := NewGetLatestE2EComparisonUseCase(projectRepo, runRepo, specRepo)

		_, err := uc.Execute(context.Background(), "proj1")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInternal {
			t.Fatalf("expected code to be INTERNAL, got %s", appErr.Code)
		}
	})
	t.Run("Return an error when failure to get latest by project", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			project: domain.Project{ID: "proj1", DefaultBranch: "main"},
		}
		runRepo := &stubE2ETestRunRepository{latestByProjectErr: fmt.Errorf("db error")}
		specRepo := &stubE2ESpecResultRepository{}
		uc := NewGetLatestE2EComparisonUseCase(projectRepo, runRepo, specRepo)

		_, err := uc.Execute(context.Background(), "proj1")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInternal {
			t.Fatalf("expected code to be INTERNAL, got %s", appErr.Code)
		}
	})
	t.Run("Return an error when failure to get latest by project and branch", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			project: domain.Project{ID: "proj1", DefaultBranch: "main"},
		}
		runRepo := &stubE2ETestRunRepository{
			latestByProject: &domain.E2ETestRun{
				ID:          "run1",
				ProjectID:   "proj1",
				TotalSpecs:  10,
				PassedSpecs: 9,
				FailedSpecs: 1,
				Status:      domain.E2ERunStatusFailed,
			},
			latestByBranchErr: fmt.Errorf("db error"),
		}
		specRepo := &stubE2ESpecResultRepository{}
		uc := NewGetLatestE2EComparisonUseCase(projectRepo, runRepo, specRepo)

		_, err := uc.Execute(context.Background(), "proj1")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInternal {
			t.Fatalf("expected code to be INTERNAL, got %s", appErr.Code)
		}
	})
	t.Run("Return an error when failure to list failed by run ID", func(t *testing.T) {
		projectRepo := &stubProjectRepository{
			project: domain.Project{ID: "proj1", DefaultBranch: "main"},
		}
		runRepo := &stubE2ETestRunRepository{
			latestByProject: &domain.E2ETestRun{
				ID:          "run1",
				ProjectID:   "proj1",
				TotalSpecs:  10,
				PassedSpecs: 9,
				FailedSpecs: 1,
				Status:      domain.E2ERunStatusFailed,
			},
		}
		specRepo := &stubE2ESpecResultRepository{failedByRunIDErr: fmt.Errorf("db error")}
		uc := NewGetLatestE2EComparisonUseCase(projectRepo, runRepo, specRepo)

		_, err := uc.Execute(context.Background(), "proj1")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var appErr *AppError
		if !errors.As(err, &appErr) {
			t.Fatalf("expected AppError, got %T", err)
		}
		if appErr.Code != CodeInternal {
			t.Fatalf("expected code to be INTERNAL, got %s", appErr.Code)
		}
	})
}

func TestGetE2EHeatmapExecute(t *testing.T) {
	t.Run("Execute heatmap successfully", func(t *testing.T) {
		uc := NewGetE2EHeatmapUseCase(&stubE2ETestRunRepository{
			heatmapRows: []TestHeatmapRow{
				{
					RunID:        "abc123",
					ProjectID:    "Proj1",
					ProjectName:  "project-1",
					ProjectGroup: "frontend",
					ProjectKey:   "project/project-1",
					Branch:       "main",
					CommitSHA:    "abc123",
					RunTimestamp: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
					PassedSpecs:  1,
					TotalSpecs:   2,
					Status:       "failed",
					Environment:  StringPtr("test"),
				},
			},
		})
		out, err := uc.Execute(context.Background(), E2EHeatmapInput{
			Branch:         "main",
			Status:         "failed",
			SpecType:       "happyPath",
			RunsPerProject: 10,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		fmt.Println(out.Groups)
		if len(out.Groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(out.Groups))
		}
		if out.Groups[0].GroupName != "frontend" {
			t.Fatalf("expected group name frontend, got %s", out.Groups[0].GroupName)
		}
		if out.Groups[0].Projects[0].ProjectName != "project-1" {
			t.Fatalf("expected project name project-1, got %s", out.Groups[0].Projects[0].ProjectName)
		}
		if out.Groups[0].Projects[0].ProjectKey != "project/project-1" {
			t.Fatalf("expected project key project/project-1, got %s", out.Groups[0].Projects[0].ProjectKey)
		}
		if out.Groups[0].Projects[0].Runs[0].ID != "abc123" {
			t.Fatalf("expected run ID abc123, got %s", out.Groups[0].Projects[0].Runs[0].ID)
		}
		if out.Groups[0].Projects[0].Runs[0].PassRatePercent != 50 {
			t.Fatalf("expected pass rate percent 50, got %f", out.Groups[0].Projects[0].Runs[0].PassRatePercent)
		}
		if out.Groups[0].Projects[0].Runs[0].Environment == nil || *out.Groups[0].Projects[0].Runs[0].Environment != "test" {
			t.Fatalf("expected environment test, got %v", out.Groups[0].Projects[0].Runs[0].Environment)
		}
		if out.Groups[0].Projects[0].Runs[0].RunTimestamp != "2026-04-01T12:00:00Z" {
			t.Fatalf("expected run timestamp 2026-04-01T12:00:00Z, got %s", out.Groups[0].Projects[0].Runs[0].RunTimestamp)
		}
		if out.Groups[0].Projects[0].Runs[0].Status != "failed" {
			t.Fatalf("expected status failed, got %s", out.Groups[0].Projects[0].Runs[0].Status)
		}
	})
}
