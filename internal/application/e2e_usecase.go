package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/arxdsilva/opencoverage/internal/domain"
)

type IngestE2ERunInput struct {
	ProjectKey    string           `json:"projectKey"`
	ProjectName   string           `json:"projectName"`
	ProjectGroup  *string          `json:"projectGroup,omitempty"`
	DefaultBranch string           `json:"defaultBranch"`
	Branch        string           `json:"branch"`
	CommitSHA     string           `json:"commitSha"`
	Author        string           `json:"author"`
	TriggerType   string           `json:"triggerType"`
	RunTimestamp  string           `json:"runTimestamp"`
	Environment   *string          `json:"environment,omitempty"`
	TestReport    IngestReportBody `json:"testReport"`
}

type IngestReportBody struct {
	ReportType                 string             `json:"reportType"`
	FrameworkVersion           string             `json:"frameworkVersion,omitempty"`
	TestFramework              string             `json:"testFramework,omitempty"`
	PlatformType               string             `json:"platformType,omitempty"`
	SuiteDescription           string             `json:"suiteDescription"`
	SuitePath                  string             `json:"suitePath"`
	SuiteSucceeded             bool               `json:"suiteSucceeded,omitempty"`
	SpecialSuiteFailureReasons []string           `json:"specialSuiteFailureReasons,omitempty"`
	SpecReports                []IngestSpecReport `json:"specReports"`
}

type IngestSpecReport struct {
	LeafNodeText            string             `json:"leafNodeText"`
	ContainerHierarchyTexts []string           `json:"containerHierarchyTexts"`
	State                   string             `json:"state"`
	RunTime                 float64            `json:"runTime"`
	Failure                 *IngestTestFailure `json:"failure,omitempty"`
}

type IngestTestFailure struct {
	Message  string              `json:"message"`
	Location *IngestTestLocation `json:"location,omitempty"`
}

type IngestTestLocation struct {
	FileName   string `json:"fileName"`
	LineNumber int    `json:"lineNumber"`
}

type E2ERunResponse struct {
	ID              string  `json:"id"`
	Branch          string  `json:"branch"`
	CommitSHA       string  `json:"commitSha"`
	Author          string  `json:"author,omitempty"`
	TriggerType     string  `json:"triggerType"`
	RunTimestamp    string  `json:"runTimestamp"`
	Environment     *string `json:"environment,omitempty"`
	TotalSpecs      int     `json:"totalSpecs"`
	PassedSpecs     int     `json:"passedSpecs"`
	FailedSpecs     int     `json:"failedSpecs"`
	SkippedSpecs    int     `json:"skippedSpecs"`
	PendingSpecs    int     `json:"pendingSpecs"`
	FlakedSpecs     int     `json:"flakedSpecs"`
	PassRatePercent float64 `json:"passRatePercent"`
	DurationMS      int64   `json:"durationMs"`
	Status          string  `json:"status"`
}

type E2EComparisonResponse struct {
	BaselineSource          string   `json:"baselineSource"`
	PreviousPassRatePercent *float64 `json:"previousPassRatePercent"`
	CurrentPassRatePercent  float64  `json:"currentPassRatePercent"`
	DeltaPercent            *float64 `json:"deltaPercent"`
	Direction               string   `json:"direction"`
	NewFailures             int      `json:"newFailures"`
	ResolvedFailures        int      `json:"resolvedFailures"`
}

type IngestE2ERunOutput struct {
	Project     ProjectResponse       `json:"project"`
	Run         E2ERunResponse        `json:"run"`
	Comparison  E2EComparisonResponse `json:"comparison"`
	FailedSpecs []FailedSpecResponse  `json:"failedSpecs"`
}

type IngestE2ERunUseCase struct {
	projects ProjectRepository
	runs     E2ETestRunRepository
	specs    E2ESpecResultRepository
	tx       TransactionManager
	ids      IDGenerator
	clock    Clock
}

func NewIngestE2ERunUseCase(
	projects ProjectRepository,
	runs E2ETestRunRepository,
	specs E2ESpecResultRepository,
	tx TransactionManager,
	ids IDGenerator,
	clock Clock,
) *IngestE2ERunUseCase {
	return &IngestE2ERunUseCase{
		projects: projects,
		runs:     runs,
		specs:    specs,
		tx:       tx,
		ids:      ids,
		clock:    clock,
	}
}

func (uc *IngestE2ERunUseCase) Execute(ctx context.Context, in IngestE2ERunInput) (IngestE2ERunOutput, error) {
	if err := validateE2EIngestInput(in); err != nil {
		return IngestE2ERunOutput{}, err
	}

	runTime, err := time.Parse(time.RFC3339, in.RunTimestamp)
	if err != nil {
		return IngestE2ERunOutput{}, NewInvalidArgument("runTimestamp must be RFC3339", map[string]any{"field": "runTimestamp"})
	}

	project, created, err := uc.resolveOrCreateE2EProject(ctx, in)
	if err != nil {
		return IngestE2ERunOutput{}, err
	}

	var baseline *domain.E2ETestRun
	var baselineFailed []domain.E2ESpecResult
	baseRun, err := uc.runs.GetLatestByProjectAndBranch(ctx, project.ID, project.DefaultBranch)
	if err == nil {
		baseline = &baseRun
		baselineFailed, err = uc.specs.ListFailedByRunID(ctx, baseRun.ID)
		if err != nil {
			return IngestE2ERunOutput{}, NewInternal("failed to load baseline failed specs", err)
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return IngestE2ERunOutput{}, NewInternal("failed to load baseline e2e run", err)
	}

	run, specEntities := uc.buildE2EEntities(project.ID, in, runTime)

	if err := uc.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := uc.runs.Create(txCtx, run); err != nil {
			return err
		}
		if err := uc.specs.CreateBatch(txCtx, specEntities); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return IngestE2ERunOutput{}, NewInternal("failed to persist e2e run", err)
	}

	failedSpecs := failedE2ESpecsFromResults(specEntities)
	passRate := calculatePassRate(run.PassedSpecs, run.TotalSpecs)
	var previousPassRate *float64
	newFailures := 0
	resolvedFailures := 0
	if baseline != nil {
		prev := calculatePassRate(baseline.PassedSpecs, baseline.TotalSpecs)
		previousPassRate = &prev
		newFailures, resolvedFailures = compareFailedSpecs(failedE2ESpecsFromResults(baselineFailed), failedSpecs)
	}

	return IngestE2ERunOutput{
		Project: ProjectResponse{
			ID:                     project.ID,
			ProjectKey:             project.ProjectKey,
			Name:                   project.Name,
			Group:                  project.Group,
			DefaultBranch:          project.DefaultBranch,
			GlobalThresholdPercent: project.GlobalThresholdPercent,
			Created:                created,
		},
		Run: e2eRunResponse(run),
		Comparison: buildE2EComparison(
			passRate,
			previousPassRate,
			newFailures,
			resolvedFailures,
		),
		FailedSpecs: failedSpecs,
	}, nil
}

func (uc *IngestE2ERunUseCase) resolveOrCreateE2EProject(ctx context.Context, in IngestE2ERunInput) (domain.Project, bool, error) {
	project, err := uc.projects.GetByKey(ctx, in.ProjectKey)
	if err == nil {
		return project, false, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.Project{}, false, NewInternal("failed to load project", err)
	}

	defaultBranch := in.DefaultBranch
	if strings.TrimSpace(defaultBranch) == "" {
		defaultBranch = domain.DefaultBranch
	}

	now := uc.clock.Now().UTC()
	created := domain.Project{
		ID:                     uc.ids.NewID(),
		ProjectKey:             in.ProjectKey,
		Name:                   in.ProjectName,
		Group:                  in.ProjectGroup,
		DefaultBranch:          defaultBranch,
		GlobalThresholdPercent: domain.DefaultThresholdPercent,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	project, err = uc.projects.Create(ctx, created)
	if err != nil {
		return domain.Project{}, false, NewInternal("failed to create project", err)
	}
	return project, true, nil
}

func (uc *IngestE2ERunUseCase) buildE2EEntities(projectID string, in IngestE2ERunInput, runTime time.Time) (domain.E2ETestRun, []domain.E2ESpecResult) {
	total := len(in.TestReport.SpecReports)
	passed := 0
	failed := 0
	skipped := 0
	pending := 0
	flaky := 0
	interrupted := false
	timedOut := false
	var totalDurationMS int64

	specResults := make([]domain.E2ESpecResult, 0, total)
	for _, spec := range in.TestReport.SpecReports {
		normalizedState := normalizeTestState(spec.State)
		switch normalizedState {
		case domain.E2ESpecStatePassed:
			passed++
		case domain.E2ESpecStateFailed:
			failed++
		case domain.E2ESpecStateSkipped:
			skipped++
		case domain.E2ESpecStatePending:
			pending++
		case domain.E2ESpecStateFlaky:
			flaky++
		}

		if strings.EqualFold(strings.TrimSpace(spec.State), "interrupted") {
			interrupted = true
		}
		if strings.EqualFold(strings.TrimSpace(spec.State), "timedout") {
			timedOut = true
		}

		durationMS := int64(spec.RunTime * 1000)
		if durationMS < 0 {
			durationMS = 0
		}
		totalDurationMS += durationMS

		specPath := spec.LeafNodeText
		if len(spec.ContainerHierarchyTexts) > 0 {
			specPath = strings.Join(append(spec.ContainerHierarchyTexts, spec.LeafNodeText), " > ")
		}

		var failureMessage *string
		var failureFile *string
		var failureLine *int
		if spec.Failure != nil && strings.TrimSpace(spec.Failure.Message) != "" {
			message := strings.TrimSpace(spec.Failure.Message)
			failureMessage = &message
		}
		if spec.Failure != nil && spec.Failure.Location != nil {
			if file := strings.TrimSpace(spec.Failure.Location.FileName); file != "" {
				failureFile = &file
			}
			if spec.Failure.Location.LineNumber > 0 {
				line := spec.Failure.Location.LineNumber
				failureLine = &line
			}
		}

		specResults = append(specResults, domain.E2ESpecResult{
			ID:                  uc.ids.NewID(),
			SpecPath:            specPath,
			LeafNodeText:        spec.LeafNodeText,
			State:               normalizedState,
			DurationMS:          durationMS,
			FailureMessage:      failureMessage,
			FailureLocationFile: failureFile,
			FailureLocationLine: failureLine,
		})
	}

	runID := uc.ids.NewID()
	for i := range specResults {
		specResults[i].E2ETestRunID = runID
	}

	run := domain.E2ETestRun{
		ID:               runID,
		ProjectID:        projectID,
		Branch:           in.Branch,
		CommitSHA:        in.CommitSHA,
		Author:           in.Author,
		TriggerType:      in.TriggerType,
		RunTimestamp:     runTime,
		FrameworkVersion: in.TestReport.FrameworkVersion,
		TestFramework:    in.TestReport.TestFramework,
		PlatformType:     in.TestReport.PlatformType,
		SuiteDescription: in.TestReport.SuiteDescription,
		SuitePath:        in.TestReport.SuitePath,
		TotalSpecs:       total,
		PassedSpecs:      passed,
		FailedSpecs:      failed,
		SkippedSpecs:     skipped,
		FlakedSpecs:      flaky,
		Interrupted:      interrupted,
		TimedOut:         timedOut,
		DurationMS:       totalDurationMS,
		Status:           domain.EvaluateE2ERunStatus(failed, interrupted, timedOut),
		Environment:      in.Environment,
		CreatedAt:        uc.clock.Now().UTC(),
	}

	return run, specResults
}

func validateE2EIngestInput(in IngestE2ERunInput) error {
	if strings.TrimSpace(in.ProjectKey) == "" {
		return NewInvalidArgument("projectKey is required", map[string]any{"field": "projectKey"})
	}
	if strings.TrimSpace(in.Branch) == "" {
		return NewInvalidArgument("branch is required", map[string]any{"field": "branch"})
	}
	if strings.TrimSpace(in.CommitSHA) == "" {
		return NewInvalidArgument("commitSha is required", map[string]any{"field": "commitSha"})
	}
	if err := domain.ValidateTriggerType(in.TriggerType); err != nil {
		return NewInvalidArgument(err.Error(), map[string]any{"field": "triggerType"})
	}
	if strings.TrimSpace(in.TestReport.SuiteDescription) == "" {
		return NewInvalidArgument("testReport.suiteDescription is required", map[string]any{"field": "testReport.suiteDescription"})
	}
	if strings.TrimSpace(in.TestReport.SuitePath) == "" {
		return NewInvalidArgument("testReport.suitePath is required", map[string]any{"field": "testReport.suitePath"})
	}
	if strings.TrimSpace(in.TestReport.FrameworkVersion) == "" {
		return NewInvalidArgument("testReport.frameworkVersion is required", map[string]any{"field": "testReport.frameworkVersion"})
	}
	if strings.TrimSpace(in.TestReport.TestFramework) == "" {
		return NewInvalidArgument("testReport.testFramework is required", map[string]any{"field": "testReport.testFramework"})
	}
	if strings.TrimSpace(in.TestReport.PlatformType) == "" {
		return NewInvalidArgument("testReport.platformType is required", map[string]any{"field": "testReport.platformType"})
	}
	if len(in.TestReport.SpecReports) == 0 {
		return NewInvalidArgument("testReport.specReports must not be empty", map[string]any{"field": "testReport.specReports"})
	}

	for i, spec := range in.TestReport.SpecReports {
		if !isAcceptedTestState(spec.State) {
			return NewInvalidArgument("state is invalid", map[string]any{"field": fmt.Sprintf("testReport.specReports[%d].state", i)})
		}
		if spec.RunTime < 0 {
			return NewInvalidArgument("runTime must be >= 0", map[string]any{"field": fmt.Sprintf("testReport.specReports[%d].runTime", i)})
		}
		if normalizeTestState(spec.State) == domain.E2ESpecStateFailed && (spec.Failure == nil || strings.TrimSpace(spec.Failure.Message) == "") {
			return NewInvalidArgument("failure.message is required when state is failed", map[string]any{"field": fmt.Sprintf("testReport.specReports[%d].failure.message", i)})
		}
	}

	return nil
}

func normalizeTestState(state string) domain.E2ESpecState {
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch normalized {
	case "passed":
		return domain.E2ESpecStatePassed
	case "failed", "panicked", "interrupted", "timedout":
		return domain.E2ESpecStateFailed
	case "skipped":
		return domain.E2ESpecStateSkipped
	case "pending":
		return domain.E2ESpecStatePending
	case "flaked":
		return domain.E2ESpecStateFlaky
	default:
		return domain.E2ESpecStateFailed
	}
}

func isAcceptedTestState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "passed", "failed", "skipped", "pending", "interrupted", "timedout", "flaked":
		return true
	default:
		return false
	}
}

func buildE2EComparison(current float64, previous *float64, newFailures int, resolvedFailures int) E2EComparisonResponse {
	delta, direction := domain.CompareCoverage(current, previous)
	return E2EComparisonResponse{
		BaselineSource:          "latest_default_branch",
		PreviousPassRatePercent: previous,
		CurrentPassRatePercent:  current,
		DeltaPercent:            delta,
		Direction:               string(direction),
		NewFailures:             newFailures,
		ResolvedFailures:        resolvedFailures,
	}
}

func e2eRunResponse(run domain.E2ETestRun) E2ERunResponse {
	return E2ERunResponse{
		ID:              run.ID,
		Branch:          run.Branch,
		CommitSHA:       run.CommitSHA,
		Author:          run.Author,
		TriggerType:     run.TriggerType,
		RunTimestamp:    run.RunTimestamp.UTC().Format(time.RFC3339),
		Environment:     run.Environment,
		TotalSpecs:      run.TotalSpecs,
		PassedSpecs:     run.PassedSpecs,
		FailedSpecs:     run.FailedSpecs,
		SkippedSpecs:    run.SkippedSpecs,
		FlakedSpecs:     run.FlakedSpecs,
		PassRatePercent: calculatePassRate(run.PassedSpecs, run.TotalSpecs),
		DurationMS:      run.DurationMS,
		Status:          string(run.Status),
	}
}

func failedE2ESpecsFromResults(specs []domain.E2ESpecResult) []FailedSpecResponse {
	out := make([]FailedSpecResponse, 0)
	for _, spec := range specs {
		if spec.State != domain.E2ESpecStateFailed && spec.State != domain.E2ESpecStateFlaky {
			continue
		}
		failed := FailedSpecResponse{SpecPath: spec.SpecPath}
		if spec.FailureMessage != nil {
			failed.FailureMessage = *spec.FailureMessage
		}
		if spec.FailureLocationFile != nil {
			failed.File = *spec.FailureLocationFile
		}
		if spec.FailureLocationLine != nil {
			failed.Line = *spec.FailureLocationLine
		}
		out = append(out, failed)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SpecPath < out[j].SpecPath })
	return out
}

type ListE2ERunsInput struct {
	ProjectID   string
	Branch      string
	Status      string
	Environment string
	From        *time.Time
	To          *time.Time
	Page        int
	PageSize    int
}

type E2ERunListItem struct {
	ID              string  `json:"id"`
	Branch          string  `json:"branch"`
	CommitSHA       string  `json:"commitSha"`
	RunTimestamp    string  `json:"runTimestamp"`
	Environment     *string `json:"environment,omitempty"`
	TotalSpecs      int     `json:"totalSpecs"`
	FailedSpecs     int     `json:"failedSpecs"`
	PassRatePercent float64 `json:"passRatePercent"`
	Status          string  `json:"status"`
}

type ListE2ERunsOutput struct {
	Items      []E2ERunListItem   `json:"items"`
	Pagination PaginationResponse `json:"pagination"`
}

type ListE2ERunsUseCase struct {
	runs E2ETestRunRepository
}

func NewListE2ERunsUseCase(runs E2ETestRunRepository) *ListE2ERunsUseCase {
	return &ListE2ERunsUseCase{runs: runs}
}

func (uc *ListE2ERunsUseCase) Execute(ctx context.Context, in ListE2ERunsInput) (ListE2ERunsOutput, error) {
	page := in.Page
	if page <= 0 {
		page = 1
	}
	pageSize := in.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status != "" && status != string(domain.E2ERunStatusPassed) && status != string(domain.E2ERunStatusFailed) {
		return ListE2ERunsOutput{}, NewInvalidArgument("status must be passed or failed", map[string]any{"field": "status"})
	}
	environment := strings.ToLower(strings.TrimSpace(in.Environment))
	if environment != "" && environment != "test" && environment != "stage" && environment != "prod" {
		return ListE2ERunsOutput{}, NewInvalidArgument("environment must be one of: test, stage, prod", map[string]any{"field": "environment"})
	}

	runs, total, err := uc.runs.ListByProject(ctx, in.ProjectID, in.Branch, status, environment, in.From, in.To, page, pageSize)
	if err != nil {
		return ListE2ERunsOutput{}, NewInternal("failed to list E2E runs", err)
	}

	items := make([]E2ERunListItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, E2ERunListItem{
			ID:              run.ID,
			Branch:          run.Branch,
			CommitSHA:       run.CommitSHA,
			RunTimestamp:    run.RunTimestamp.UTC().Format(time.RFC3339),
			Environment:     run.Environment,
			TotalSpecs:      run.TotalSpecs,
			FailedSpecs:     run.FailedSpecs,
			PassRatePercent: calculatePassRate(run.PassedSpecs, run.TotalSpecs),
			Status:          string(run.Status),
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return ListE2ERunsOutput{
		Items: items,
		Pagination: PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: total,
			TotalPages: totalPages,
		},
	}, nil
}

type GetLatestE2EComparisonUseCase struct {
	projects ProjectRepository
	runs     E2ETestRunRepository
	specs    E2ESpecResultRepository
}

func NewGetLatestE2EComparisonUseCase(projects ProjectRepository, runs E2ETestRunRepository, specs E2ESpecResultRepository) *GetLatestE2EComparisonUseCase {
	return &GetLatestE2EComparisonUseCase{projects: projects, runs: runs, specs: specs}
}

func (uc *GetLatestE2EComparisonUseCase) Execute(ctx context.Context, projectID string) (IngestE2ERunOutput, error) {
	project, err := uc.projects.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return IngestE2ERunOutput{}, NewNotFound("project not found", map[string]any{"projectId": projectID})
		}
		return IngestE2ERunOutput{}, NewInternal("failed to load project", err)
	}

	run, err := uc.runs.GetLatestByProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return IngestE2ERunOutput{}, NewNotFound("no E2E runs found", map[string]any{"projectId": projectID})
		}
		return IngestE2ERunOutput{}, NewInternal("failed to load latest E2E run", err)
	}

	failedSpecs, err := uc.specs.ListFailedByRunID(ctx, run.ID)
	if err != nil {
		return IngestE2ERunOutput{}, NewInternal("failed to load failed specs", err)
	}

	baselineRun, err := uc.runs.GetLatestByProjectAndBranch(ctx, projectID, project.DefaultBranch)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return IngestE2ERunOutput{}, NewInternal("failed to load baseline E2E run", err)
	}

	var previousPassRate *float64
	newFailures := 0
	resolvedFailures := 0
	if err == nil && baselineRun.ID != run.ID {
		prevRate := calculatePassRate(baselineRun.PassedSpecs, baselineRun.TotalSpecs)
		previousPassRate = &prevRate
		baselineFailed, bErr := uc.specs.ListFailedByRunID(ctx, baselineRun.ID)
		if bErr != nil {
			return IngestE2ERunOutput{}, NewInternal("failed to load baseline failed specs", bErr)
		}
		newFailures, resolvedFailures = compareFailedSpecs(failedE2ESpecsFromResults(baselineFailed), failedE2ESpecsFromResults(failedSpecs))
	}

	return IngestE2ERunOutput{
		Project: ProjectResponse{
			ID:                     project.ID,
			ProjectKey:             project.ProjectKey,
			Name:                   project.Name,
			Group:                  project.Group,
			DefaultBranch:          project.DefaultBranch,
			GlobalThresholdPercent: project.GlobalThresholdPercent,
			Created:                false,
		},
		Run: e2eRunResponse(run),
		Comparison: buildE2EComparison(
			calculatePassRate(run.PassedSpecs, run.TotalSpecs),
			previousPassRate,
			newFailures,
			resolvedFailures,
		),
		FailedSpecs: failedE2ESpecsFromResults(failedSpecs),
	}, nil
}

type GetE2ERunUseCase struct {
	runs  E2ETestRunRepository
	specs E2ESpecResultRepository
}

func NewGetE2ERunUseCase(runs E2ETestRunRepository, specs E2ESpecResultRepository) *GetE2ERunUseCase {
	return &GetE2ERunUseCase{runs: runs, specs: specs}
}

func (uc *GetE2ERunUseCase) Execute(ctx context.Context, projectID string, runID string) (IngestE2ERunOutput, error) {
	run, err := uc.runs.GetByID(ctx, projectID, runID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return IngestE2ERunOutput{}, NewNotFound("E2E run not found", map[string]any{"projectId": projectID, "runId": runID})
		}
		return IngestE2ERunOutput{}, NewInternal("failed to load E2E run", err)
	}
	specs, err := uc.specs.ListByRunID(ctx, run.ID)
	if err != nil {
		return IngestE2ERunOutput{}, NewInternal("failed to load E2E spec results", err)
	}

	return IngestE2ERunOutput{
		Run:         e2eRunResponse(run),
		Comparison:  buildE2EComparison(calculatePassRate(run.PassedSpecs, run.TotalSpecs), nil, 0, 0),
		FailedSpecs: failedE2ESpecsFromResults(specs),
	}, nil
}

// GetE2EHeatmapUseCase returns recent runs for all projects grouped by project group.

type E2EHeatmapInput struct {
	Branch         string
	Status         string
	RunsPerProject int
}

type GetE2EHeatmapOutput struct {
	Groups []HeatmapGroupItem `json:"groups"`
}

type GetE2EHeatmapUseCase struct {
	runs E2ETestRunRepository
}

func NewGetE2EHeatmapUseCase(runs E2ETestRunRepository) *GetE2EHeatmapUseCase {
	return &GetE2EHeatmapUseCase{runs: runs}
}

func (uc *GetE2EHeatmapUseCase) Execute(ctx context.Context, in E2EHeatmapInput) (GetE2EHeatmapOutput, error) {
	runsPerProject := in.RunsPerProject
	if runsPerProject <= 0 {
		runsPerProject = 10
	}
	if runsPerProject > 30 {
		runsPerProject = 30
	}

	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status != "" && status != string(domain.E2ERunStatusPassed) && status != string(domain.E2ERunStatusFailed) {
		return GetE2EHeatmapOutput{}, NewInvalidArgument("status must be passed or failed", map[string]any{"field": "status"})
	}

	rows, err := uc.runs.HeatmapData(ctx, in.Branch, status, runsPerProject)
	if err != nil {
		return GetE2EHeatmapOutput{}, NewInternal("failed to load heatmap data", err)
	}

	// Rows arrive ordered: non-empty groups first (alpha), then empty group last,
	// within each group projects alpha, within each project newest runs first.
	// We preserve insertion order to match SQL ordering.
	groupOrder := make([]string, 0)
	groupSeen := make(map[string]bool)
	projectOrder := make(map[string][]string)
	projectSeen := make(map[string]bool)
	projectMeta := make(map[string]HeatmapProjectItem)

	for _, row := range rows {
		if !groupSeen[row.ProjectGroup] {
			groupSeen[row.ProjectGroup] = true
			groupOrder = append(groupOrder, row.ProjectGroup)
		}
		if !projectSeen[row.ProjectID] {
			projectSeen[row.ProjectID] = true
			projectOrder[row.ProjectGroup] = append(projectOrder[row.ProjectGroup], row.ProjectID)
			projectMeta[row.ProjectID] = HeatmapProjectItem{
				ProjectID:   row.ProjectID,
				ProjectName: row.ProjectName,
				ProjectKey:  row.ProjectKey,
				Runs:        []HeatmapRunItem{},
			}
		}
		p := projectMeta[row.ProjectID]
		p.Runs = append(p.Runs, HeatmapRunItem{
			ID:              row.RunID,
			Branch:          row.Branch,
			CommitSHA:       row.CommitSHA,
			RunTimestamp:    row.RunTimestamp.UTC().Format(time.RFC3339),
			PassRatePercent: calculatePassRate(row.PassedSpecs, row.TotalSpecs),
			Status:          row.Status,
			Environment:     row.Environment,
		})
		projectMeta[row.ProjectID] = p
	}

	groups := make([]HeatmapGroupItem, 0, len(groupOrder))
	for _, groupName := range groupOrder {
		projectIDs := projectOrder[groupName]
		projects := make([]HeatmapProjectItem, 0, len(projectIDs))
		for _, pid := range projectIDs {
			projects = append(projects, projectMeta[pid])
		}
		groups = append(groups, HeatmapGroupItem{
			GroupName: groupName,
			Projects:  projects,
		})
	}

	return GetE2EHeatmapOutput{Groups: groups}, nil
}
