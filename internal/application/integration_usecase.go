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

type IngestIntegrationRunInput struct {
	ProjectKey    string                 `json:"projectKey"`
	ProjectName   string                 `json:"projectName"`
	ProjectGroup  *string                `json:"projectGroup,omitempty"`
	DefaultBranch string                 `json:"defaultBranch"`
	Branch        string                 `json:"branch"`
	CommitSHA     string                 `json:"commitSha"`
	Author        string                 `json:"author"`
	TriggerType   string                 `json:"triggerType"`
	RunTimestamp  string                 `json:"runTimestamp"`
	Environment   *string                `json:"environment,omitempty"`
	GinkgoReport  IngestGinkgoReportBody `json:"ginkgoReport"`
}

type IngestGinkgoReportBody struct {
	GinkgoVersion              string                   `json:"ginkgoVersion,omitempty"`
	SuiteDescription           string                   `json:"suiteDescription"`
	SuitePath                  string                   `json:"suitePath"`
	SuiteSucceeded             bool                     `json:"suiteSucceeded,omitempty"`
	SpecialSuiteFailureReasons []string                 `json:"specialSuiteFailureReasons,omitempty"`
	SpecReports                []IngestGinkgoSpecReport `json:"specReports"`
}

type IngestGinkgoSpecReport struct {
	LeafNodeText            string               `json:"leafNodeText"`
	ContainerHierarchyTexts []string             `json:"containerHierarchyTexts"`
	State                   string               `json:"state"`
	RunTime                 float64              `json:"runTime"`
	Failure                 *IngestGinkgoFailure `json:"failure,omitempty"`
}

type IngestGinkgoFailure struct {
	Message  string                `json:"message"`
	Location *IngestGinkgoLocation `json:"location,omitempty"`
}

type IngestGinkgoLocation struct {
	FileName   string `json:"fileName"`
	LineNumber int    `json:"lineNumber"`
}

type IntegrationRunResponse struct {
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
	FlakedSpecs     int     `json:"flakedSpecs"`
	PendingSpecs    int     `json:"pendingSpecs"`
	PassRatePercent float64 `json:"passRatePercent"`
	DurationMS      int64   `json:"durationMs"`
	Status          string  `json:"status"`
}

type IntegrationComparisonResponse struct {
	BaselineSource          string   `json:"baselineSource"`
	PreviousPassRatePercent *float64 `json:"previousPassRatePercent"`
	CurrentPassRatePercent  float64  `json:"currentPassRatePercent"`
	DeltaPercent            *float64 `json:"deltaPercent"`
	Direction               string   `json:"direction"`
	NewFailures             int      `json:"newFailures"`
	ResolvedFailures        int      `json:"resolvedFailures"`
}

type FailedSpecResponse struct {
	SpecPath       string `json:"specPath"`
	SpecType       string `json:"specType,omitempty"`
	FailureMessage string `json:"failureMessage"`
	File           string `json:"file,omitempty"`
	Line           int    `json:"line,omitempty"`
}

type IngestIntegrationRunOutput struct {
	Project     ProjectResponse               `json:"project"`
	Run         IntegrationRunResponse        `json:"run"`
	Comparison  IntegrationComparisonResponse `json:"comparison"`
	FailedSpecs []FailedSpecResponse          `json:"failedSpecs"`
}

type IngestIntegrationRunUseCase struct {
	projects ProjectRepository
	runs     IntegrationTestRunRepository
	specs    IntegrationSpecResultRepository
	tx       TransactionManager
	ids      IDGenerator
	clock    Clock
}

func NewIngestIntegrationRunUseCase(
	projects ProjectRepository,
	runs IntegrationTestRunRepository,
	specs IntegrationSpecResultRepository,
	tx TransactionManager,
	ids IDGenerator,
	clock Clock,
) *IngestIntegrationRunUseCase {
	return &IngestIntegrationRunUseCase{
		projects: projects,
		runs:     runs,
		specs:    specs,
		tx:       tx,
		ids:      ids,
		clock:    clock,
	}
}

func (uc *IngestIntegrationRunUseCase) Execute(ctx context.Context, in IngestIntegrationRunInput) (IngestIntegrationRunOutput, error) {
	if err := ValidateIntegrationIngestInput(in); err != nil {
		return IngestIntegrationRunOutput{}, err
	}

	runTime, err := time.Parse(time.RFC3339, in.RunTimestamp)
	if err != nil {
		return IngestIntegrationRunOutput{}, NewInvalidArgument("runTimestamp must be RFC3339", map[string]any{"field": "runTimestamp"})
	}

	project, created, err := uc.resolveOrCreateIntegrationProject(ctx, in)
	if err != nil {
		return IngestIntegrationRunOutput{}, err
	}

	var baseline *domain.IntegrationTestRun
	var baselineFailed []domain.IntegrationSpecResult
	baseRun, err := uc.runs.GetLatestByProjectAndBranch(ctx, project.ID, project.DefaultBranch)
	if err == nil {
		baseline = &baseRun
		baselineFailed, err = uc.specs.ListFailedByRunID(ctx, baseRun.ID)
		if err != nil {
			return IngestIntegrationRunOutput{}, NewInternal("failed to load baseline failed specs", err)
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return IngestIntegrationRunOutput{}, NewInternal("failed to load baseline integration run", err)
	}

	run, specEntities := uc.buildIntegrationEntities(project.ID, in, runTime)

	if err := uc.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := uc.runs.Create(txCtx, run); err != nil {
			return err
		}
		if err := uc.specs.CreateBatch(txCtx, specEntities); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return IngestIntegrationRunOutput{}, NewInternal("failed to persist integration run", err)
	}

	failedSpecs := failedSpecsFromResults(specEntities)
	passRate := calculatePassRate(run.PassedSpecs, run.TotalSpecs)
	var previousPassRate *float64
	newFailures := 0
	resolvedFailures := 0
	if baseline != nil {
		prev := calculatePassRate(baseline.PassedSpecs, baseline.TotalSpecs)
		previousPassRate = &prev
		newFailures, resolvedFailures = compareFailedSpecs(failedSpecsFromResults(baselineFailed), failedSpecs)
	}

	return IngestIntegrationRunOutput{
		Project: ProjectResponse{
			ID:                     project.ID,
			ProjectKey:             project.ProjectKey,
			Name:                   project.Name,
			DefaultBranch:          project.DefaultBranch,
			GlobalThresholdPercent: project.GlobalThresholdPercent,
			Created:                created,
		},
		Run: integrationRunResponse(run),
		Comparison: buildIntegrationComparison(
			passRate,
			previousPassRate,
			newFailures,
			resolvedFailures,
		),
		FailedSpecs: failedSpecs,
	}, nil
}

func (uc *IngestIntegrationRunUseCase) resolveOrCreateIntegrationProject(ctx context.Context, in IngestIntegrationRunInput) (domain.Project, bool, error) {
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

func (uc *IngestIntegrationRunUseCase) buildIntegrationEntities(projectID string, in IngestIntegrationRunInput, runTime time.Time) (domain.IntegrationTestRun, []domain.IntegrationSpecResult) {
	total := len(in.GinkgoReport.SpecReports)
	passed := 0
	failed := 0
	skipped := 0
	pending := 0
	flaky := 0
	interrupted := false
	timedOut := false
	var totalDurationMS int64

	specResults := make([]domain.IntegrationSpecResult, 0, total)
	for _, spec := range in.GinkgoReport.SpecReports {
		normalizedState := normalizeGinkgoState(spec.State)
		switch normalizedState {
		case domain.IntegrationSpecStatePassed:
			passed++
		case domain.IntegrationSpecStateFailed:
			failed++
		case domain.IntegrationSpecStateSkipped:
			skipped++
		case domain.IntegrationSpecStatePending:
			pending++
		case domain.IntegrationSpecStateFlaky:
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

		specResults = append(specResults, domain.IntegrationSpecResult{
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
		specResults[i].IntegrationRunID = runID
	}

	run := domain.IntegrationTestRun{
		ID:               runID,
		ProjectID:        projectID,
		Branch:           in.Branch,
		CommitSHA:        in.CommitSHA,
		Author:           in.Author,
		TriggerType:      in.TriggerType,
		RunTimestamp:     runTime,
		GinkgoVersion:    in.GinkgoReport.GinkgoVersion,
		SuiteDescription: in.GinkgoReport.SuiteDescription,
		SuitePath:        in.GinkgoReport.SuitePath,
		TotalSpecs:       total,
		PassedSpecs:      passed,
		FailedSpecs:      failed,
		SkippedSpecs:     skipped,
		FlakedSpecs:      flaky,
		PendingSpecs:     pending,
		Interrupted:      interrupted,
		TimedOut:         timedOut,
		DurationMS:       totalDurationMS,
		Status:           domain.EvaluateIntegrationRunStatus(failed, interrupted, timedOut),
		Environment:      in.Environment,
		CreatedAt:        uc.clock.Now().UTC(),
	}

	return run, specResults
}

func validateIntegrationIngestInput(in IngestIntegrationRunInput) error {
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
	if strings.TrimSpace(in.GinkgoReport.SuiteDescription) == "" {
		return NewInvalidArgument("ginkgoReport.suiteDescription is required", map[string]any{"field": "ginkgoReport.suiteDescription"})
	}
	if strings.TrimSpace(in.GinkgoReport.SuitePath) == "" {
		return NewInvalidArgument("ginkgoReport.suitePath is required", map[string]any{"field": "ginkgoReport.suitePath"})
	}
	if len(in.GinkgoReport.SpecReports) == 0 {
		return NewInvalidArgument("ginkgoReport.specReports must not be empty", map[string]any{"field": "ginkgoReport.specReports"})
	}

	for i, spec := range in.GinkgoReport.SpecReports {
		if !isAcceptedGinkgoState(spec.State) {
			return NewInvalidArgument("state is invalid", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].state", i)})
		}
		if spec.RunTime < 0 {
			return NewInvalidArgument("runTime must be >= 0", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].runTime", i)})
		}
		if normalizeGinkgoState(spec.State) == domain.IntegrationSpecStateFailed {
			if spec.Failure == nil || strings.TrimSpace(spec.Failure.Message) == "" {
				return NewInvalidArgument("failure.message is required when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.message", i)})
			}
			if spec.Failure.Location == nil {
				return NewInvalidArgument("failure.location is required when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.location", i)})
			}
			if strings.TrimSpace(spec.Failure.Location.FileName) == "" {
				return NewInvalidArgument("failure.location.fileName is required when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.location.fileName", i)})
			}
			if spec.Failure.Location.LineNumber < 0 {
				return NewInvalidArgument("failure.location.lineNumber must be >= 0 when state is failed", map[string]any{"field": fmt.Sprintf("ginkgoReport.specReports[%d].failure.location.lineNumber", i)})
			}
		}
	}

	return nil
}

// ValidateIntegrationIngestInput validates an integration ingest payload.
func ValidateIntegrationIngestInput(in IngestIntegrationRunInput) error {
	return validateIntegrationIngestInput(in)
}

func normalizeGinkgoState(state string) domain.IntegrationSpecState {
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch normalized {
	case "passed":
		return domain.IntegrationSpecStatePassed
	case "failed", "panicked", "interrupted", "timedout":
		return domain.IntegrationSpecStateFailed
	case "skipped":
		return domain.IntegrationSpecStateSkipped
	case "pending":
		return domain.IntegrationSpecStatePending
	case "flaked":
		return domain.IntegrationSpecStateFlaky
	default:
		return domain.IntegrationSpecStateFailed
	}
}

func isAcceptedGinkgoState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "passed", "failed", "skipped", "pending", "panicked", "interrupted", "timedout", "flaked":
		return true
	default:
		return false
	}
}

func calculatePassRate(passed int, total int) float64 {
	if total <= 0 {
		return 0
	}
	return roundTo2((float64(passed) / float64(total)) * 100)
}

func buildIntegrationComparison(current float64, previous *float64, newFailures int, resolvedFailures int) IntegrationComparisonResponse {
	delta, direction := domain.CompareCoverage(current, previous)
	return IntegrationComparisonResponse{
		BaselineSource:          "latest_default_branch",
		PreviousPassRatePercent: previous,
		CurrentPassRatePercent:  current,
		DeltaPercent:            delta,
		Direction:               string(direction),
		NewFailures:             newFailures,
		ResolvedFailures:        resolvedFailures,
	}
}

func integrationRunResponse(run domain.IntegrationTestRun) IntegrationRunResponse {
	return IntegrationRunResponse{
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
		PendingSpecs:    run.PendingSpecs,
		PassRatePercent: calculatePassRate(run.PassedSpecs, run.TotalSpecs),
		DurationMS:      run.DurationMS,
		Status:          string(run.Status),
	}
}

func failedSpecsFromResults(specs []domain.IntegrationSpecResult) []FailedSpecResponse {
	out := make([]FailedSpecResponse, 0)
	for _, spec := range specs {
		if spec.State != domain.IntegrationSpecStateFailed && spec.State != domain.IntegrationSpecStateFlaky {
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

func compareFailedSpecs(previous []FailedSpecResponse, current []FailedSpecResponse) (newFailures int, resolvedFailures int) {
	prevSet := make(map[string]struct{}, len(previous))
	for _, p := range previous {
		prevSet[p.SpecPath] = struct{}{}
	}
	curSet := make(map[string]struct{}, len(current))
	for _, c := range current {
		curSet[c.SpecPath] = struct{}{}
	}

	for specPath := range curSet {
		if _, ok := prevSet[specPath]; !ok {
			newFailures++
		}
	}
	for specPath := range prevSet {
		if _, ok := curSet[specPath]; !ok {
			resolvedFailures++
		}
	}
	return newFailures, resolvedFailures
}

type ListIntegrationRunsInput struct {
	ProjectID   string
	Branch      string
	Status      string
	Environment string
	From        *time.Time
	To          *time.Time
	Page        int
	PageSize    int
}

type IntegrationRunListItem struct {
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

type ListIntegrationRunsOutput struct {
	Items      []IntegrationRunListItem `json:"items"`
	Pagination PaginationResponse       `json:"pagination"`
}

type ListIntegrationRunsUseCase struct {
	runs IntegrationTestRunRepository
}

func NewListIntegrationRunsUseCase(runs IntegrationTestRunRepository) *ListIntegrationRunsUseCase {
	return &ListIntegrationRunsUseCase{runs: runs}
}

func (uc *ListIntegrationRunsUseCase) Execute(ctx context.Context, in ListIntegrationRunsInput) (ListIntegrationRunsOutput, error) {
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
	if status != "" && status != string(domain.IntegrationRunStatusPassed) && status != string(domain.IntegrationRunStatusFailed) {
		return ListIntegrationRunsOutput{}, NewInvalidArgument("status must be passed or failed", map[string]any{"field": "status"})
	}
	environment := strings.ToLower(strings.TrimSpace(in.Environment))
	if environment != "" && environment != "test" && environment != "stage" && environment != "prod" && environment != "none" {
		return ListIntegrationRunsOutput{}, NewInvalidArgument("environment must be one of: test, stage, prod, none", map[string]any{"field": "environment"})
	}

	runs, total, err := uc.runs.ListByProject(ctx, in.ProjectID, in.Branch, status, environment, in.From, in.To, page, pageSize)
	if err != nil {
		return ListIntegrationRunsOutput{}, NewInternal("failed to list integration runs", err)
	}

	items := make([]IntegrationRunListItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, IntegrationRunListItem{
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

	return ListIntegrationRunsOutput{
		Items: items,
		Pagination: PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: total,
			TotalPages: totalPages,
		},
	}, nil
}

type GetLatestIntegrationComparisonUseCase struct {
	projects ProjectRepository
	runs     IntegrationTestRunRepository
	specs    IntegrationSpecResultRepository
}

func NewGetLatestIntegrationComparisonUseCase(projects ProjectRepository, runs IntegrationTestRunRepository, specs IntegrationSpecResultRepository) *GetLatestIntegrationComparisonUseCase {
	return &GetLatestIntegrationComparisonUseCase{projects: projects, runs: runs, specs: specs}
}

func (uc *GetLatestIntegrationComparisonUseCase) Execute(ctx context.Context, projectID string) (IngestIntegrationRunOutput, error) {
	project, err := uc.projects.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return IngestIntegrationRunOutput{}, NewNotFound("project not found", map[string]any{"projectId": projectID})
		}
		return IngestIntegrationRunOutput{}, NewInternal("failed to load project", err)
	}

	run, err := uc.runs.GetLatestByProject(ctx, projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return IngestIntegrationRunOutput{}, NewNotFound("no integration runs found", map[string]any{"projectId": projectID})
		}
		return IngestIntegrationRunOutput{}, NewInternal("failed to load latest integration run", err)
	}

	failedSpecs, err := uc.specs.ListFailedByRunID(ctx, run.ID)
	if err != nil {
		return IngestIntegrationRunOutput{}, NewInternal("failed to load failed specs", err)
	}

	baselineRun, err := uc.runs.GetLatestByProjectAndBranch(ctx, projectID, project.DefaultBranch)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return IngestIntegrationRunOutput{}, NewInternal("failed to load baseline integration run", err)
	}

	var previousPassRate *float64
	newFailures := 0
	resolvedFailures := 0
	if err == nil && baselineRun.ID != run.ID {
		prevRate := calculatePassRate(baselineRun.PassedSpecs, baselineRun.TotalSpecs)
		previousPassRate = &prevRate
		baselineFailed, bErr := uc.specs.ListFailedByRunID(ctx, baselineRun.ID)
		if bErr != nil {
			return IngestIntegrationRunOutput{}, NewInternal("failed to load baseline failed specs", bErr)
		}
		newFailures, resolvedFailures = compareFailedSpecs(failedSpecsFromResults(baselineFailed), failedSpecsFromResults(failedSpecs))
	}

	return IngestIntegrationRunOutput{
		Project: ProjectResponse{
			ID:                     project.ID,
			ProjectKey:             project.ProjectKey,
			Name:                   project.Name,
			DefaultBranch:          project.DefaultBranch,
			GlobalThresholdPercent: project.GlobalThresholdPercent,
			Created:                false,
		},
		Run: integrationRunResponse(run),
		Comparison: buildIntegrationComparison(
			calculatePassRate(run.PassedSpecs, run.TotalSpecs),
			previousPassRate,
			newFailures,
			resolvedFailures,
		),
		FailedSpecs: failedSpecsFromResults(failedSpecs),
	}, nil
}

type GetIntegrationRunUseCase struct {
	runs  IntegrationTestRunRepository
	specs IntegrationSpecResultRepository
}

func NewGetIntegrationRunUseCase(runs IntegrationTestRunRepository, specs IntegrationSpecResultRepository) *GetIntegrationRunUseCase {
	return &GetIntegrationRunUseCase{runs: runs, specs: specs}
}

func (uc *GetIntegrationRunUseCase) Execute(ctx context.Context, projectID string, runID string) (IngestIntegrationRunOutput, error) {
	run, err := uc.runs.GetByID(ctx, projectID, runID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return IngestIntegrationRunOutput{}, NewNotFound("integration run not found", map[string]any{"projectId": projectID, "runId": runID})
		}
		return IngestIntegrationRunOutput{}, NewInternal("failed to load integration run", err)
	}
	specs, err := uc.specs.ListByRunID(ctx, run.ID)
	if err != nil {
		return IngestIntegrationRunOutput{}, NewInternal("failed to load integration spec results", err)
	}

	return IngestIntegrationRunOutput{
		Run:         integrationRunResponse(run),
		Comparison:  buildIntegrationComparison(calculatePassRate(run.PassedSpecs, run.TotalSpecs), nil, 0, 0),
		FailedSpecs: failedSpecsFromResults(specs),
	}, nil
}

func roundTo2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// GetIntegrationHeatmapUseCase returns recent runs for all projects grouped by project group.

type IntegrationHeatmapInput struct {
	Branch         string
	Status         string
	RunsPerProject int
}

type HeatmapRunItem struct {
	ID              string  `json:"id"`
	Branch          string  `json:"branch"`
	CommitSHA       string  `json:"commitSha"`
	RunTimestamp    string  `json:"runTimestamp"`
	PassRatePercent float64 `json:"passRatePercent"`
	Status          string  `json:"status"`
	Environment     *string `json:"environment,omitempty"`
}

type HeatmapProjectItem struct {
	ProjectID   string           `json:"projectId"`
	ProjectName string           `json:"projectName"`
	ProjectKey  string           `json:"projectKey"`
	Runs        []HeatmapRunItem `json:"runs"`
}

type HeatmapGroupItem struct {
	GroupName string               `json:"groupName"`
	Projects  []HeatmapProjectItem `json:"projects"`
}

type GetIntegrationHeatmapOutput struct {
	Groups []HeatmapGroupItem `json:"groups"`
}

type GetIntegrationHeatmapUseCase struct {
	runs IntegrationTestRunRepository
}

func NewGetIntegrationHeatmapUseCase(runs IntegrationTestRunRepository) *GetIntegrationHeatmapUseCase {
	return &GetIntegrationHeatmapUseCase{runs: runs}
}

func (uc *GetIntegrationHeatmapUseCase) Execute(ctx context.Context, in IntegrationHeatmapInput) (GetIntegrationHeatmapOutput, error) {
	runsPerProject := in.RunsPerProject
	if runsPerProject <= 0 {
		runsPerProject = 10
	}
	if runsPerProject > 30 {
		runsPerProject = 30
	}

	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status != "" && status != string(domain.IntegrationRunStatusPassed) && status != string(domain.IntegrationRunStatusFailed) {
		return GetIntegrationHeatmapOutput{}, NewInvalidArgument("status must be passed or failed", map[string]any{"field": "status"})
	}

	rows, err := uc.runs.HeatmapData(ctx, in.Branch, status, runsPerProject)
	if err != nil {
		return GetIntegrationHeatmapOutput{}, NewInternal("failed to load heatmap data", err)
	}

	// Rows arrive ordered: non-empty groups first (alpha), then empty group last,
	// within each group projects alpha, within each project newest runs first.
	// We preserve insertion order to match SQL ordering.
	groupOrder := make([]string, 0)
	groupSeen := make(map[string]bool)
	projectOrder := make(map[string][]string) // groupName -> ordered project IDs
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

	return GetIntegrationHeatmapOutput{Groups: groups}, nil
}
