package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/arxdsilva/coverage-api/internal/domain"
)

type IngestCoverageRunInput struct {
	ProjectKey           string               `json:"projectKey"`
	ProjectName          string               `json:"projectName"`
	ProjectGroup         *string              `json:"projectGroup,omitempty"`
	DefaultBranch        string               `json:"defaultBranch"`
	Branch               string               `json:"branch"`
	CommitSHA            string               `json:"commitSha"`
	Author               string               `json:"author"`
	TriggerType          string               `json:"triggerType"`
	RunTimestamp         string               `json:"runTimestamp"`
	TotalCoveragePercent float64              `json:"totalCoveragePercent"`
	Packages             []IngestPackageInput `json:"packages"`
}

type IngestPackageInput struct {
	ImportPath      string  `json:"importPath"`
	CoveragePercent float64 `json:"coveragePercent"`
}

type IngestCoverageRunOutput struct {
	Project    ProjectResponse             `json:"project"`
	Run        RunResponse                 `json:"run"`
	Comparison ComparisonResponse          `json:"comparison"`
	Packages   []PackageComparisonResponse `json:"packages"`
}

type IngestCoverageRunUseCase struct {
	projects ProjectRepository
	runs     CoverageRunRepository
	packages PackageCoverageRepository
	tx       TransactionManager
	ids      IDGenerator
	clock    Clock
}

func NewIngestCoverageRunUseCase(
	projects ProjectRepository,
	runs CoverageRunRepository,
	packages PackageCoverageRepository,
	tx TransactionManager,
	ids IDGenerator,
	clock Clock,
) *IngestCoverageRunUseCase {
	return &IngestCoverageRunUseCase{
		projects: projects,
		runs:     runs,
		packages: packages,
		tx:       tx,
		ids:      ids,
		clock:    clock,
	}
}

func (uc *IngestCoverageRunUseCase) Execute(ctx context.Context, in IngestCoverageRunInput) (IngestCoverageRunOutput, error) {
	if err := validateIngestInput(in); err != nil {
		return IngestCoverageRunOutput{}, err
	}

	runTime, err := time.Parse(time.RFC3339, in.RunTimestamp)
	if err != nil {
		return IngestCoverageRunOutput{}, NewInvalidArgument("runTimestamp must be RFC3339", map[string]any{"field": "runTimestamp"})
	}

	project, created, err := uc.resolveOrCreateProject(ctx, in)
	if err != nil {
		return IngestCoverageRunOutput{}, err
	}

	var baselineRun *domain.CoverageRun
	var baselinePackages []domain.PackageCoverage
	baseRun, err := uc.runs.GetLatestByProjectAndBranch(ctx, project.ID, project.DefaultBranch)
	if err == nil {
		baselineRun = &baseRun
		baselinePackages, err = uc.packages.ListByRunID(ctx, baseRun.ID)
		if err != nil {
			return IngestCoverageRunOutput{}, NewInternal("failed to load baseline package coverage", err)
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return IngestCoverageRunOutput{}, NewInternal("failed to load baseline run", err)
	}

	run := domain.CoverageRun{
		ID:                   uc.ids.NewID(),
		ProjectID:            project.ID,
		Branch:               in.Branch,
		CommitSHA:            in.CommitSHA,
		Author:               in.Author,
		TriggerType:          in.TriggerType,
		RunTimestamp:         runTime,
		TotalCoveragePercent: in.TotalCoveragePercent,
		CreatedAt:            uc.clock.Now().UTC(),
	}

	pkgEntities := make([]domain.PackageCoverage, 0, len(in.Packages))
	for _, p := range in.Packages {
		pkgEntities = append(pkgEntities, domain.PackageCoverage{
			ID:                uc.ids.NewID(),
			RunID:             run.ID,
			PackageImportPath: p.ImportPath,
			CoveragePercent:   p.CoveragePercent,
		})
	}

	if err := uc.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := uc.runs.Create(txCtx, run); err != nil {
			return err
		}
		if err := uc.packages.CreateBatch(txCtx, pkgEntities); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return IngestCoverageRunOutput{}, NewInternal("failed to persist coverage run", err)
	}

	var previousTotal *float64
	if baselineRun != nil {
		p := baselineRun.TotalCoveragePercent
		previousTotal = &p
	}

	packageComparisons := buildPackageComparisons(pkgEntities, baselinePackages)
	sort.Slice(packageComparisons, func(i, j int) bool {
		return packageComparisons[i].ImportPath < packageComparisons[j].ImportPath
	})

	return IngestCoverageRunOutput{
		Project: ProjectResponse{
			ID:                     project.ID,
			ProjectKey:             project.ProjectKey,
			Name:                   project.Name,
			DefaultBranch:          project.DefaultBranch,
			GlobalThresholdPercent: project.GlobalThresholdPercent,
			Created:                created,
		},
		Run: RunResponse{
			ID:                   run.ID,
			Branch:               run.Branch,
			CommitSHA:            run.CommitSHA,
			Author:               run.Author,
			TriggerType:          run.TriggerType,
			RunTimestamp:         run.RunTimestamp.UTC().Format(time.RFC3339),
			TotalCoveragePercent: run.TotalCoveragePercent,
		},
		Comparison: buildComparison(run.TotalCoveragePercent, previousTotal, project.GlobalThresholdPercent),
		Packages:   packageComparisons,
	}, nil
}

func (uc *IngestCoverageRunUseCase) resolveOrCreateProject(ctx context.Context, in IngestCoverageRunInput) (domain.Project, bool, error) {
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

func validateIngestInput(in IngestCoverageRunInput) error {
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
	if err := domain.ValidateCoveragePercent(in.TotalCoveragePercent); err != nil {
		return NewInvalidArgument(err.Error(), map[string]any{"field": "totalCoveragePercent"})
	}
	if len(in.Packages) == 0 {
		return NewInvalidArgument("packages is required", map[string]any{"field": "packages"})
	}

	seen := make(map[string]struct{}, len(in.Packages))
	for i, p := range in.Packages {
		if strings.TrimSpace(p.ImportPath) == "" {
			return NewInvalidArgument("package importPath is required", map[string]any{"field": fmt.Sprintf("packages[%d].importPath", i)})
		}
		if _, exists := seen[p.ImportPath]; exists {
			return NewInvalidArgument("duplicate package importPath", map[string]any{"field": p.ImportPath})
		}
		seen[p.ImportPath] = struct{}{}
		if err := domain.ValidateCoveragePercent(p.CoveragePercent); err != nil {
			return NewInvalidArgument(err.Error(), map[string]any{"field": fmt.Sprintf("packages[%d].coveragePercent", i)})
		}
	}

	return nil
}
