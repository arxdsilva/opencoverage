package application

import (
	"context"
	"errors"
	"time"

	"github.com/arxdsilva/coverage-api/internal/domain"
)

type ListCoverageRunsInput struct {
	ProjectID string
	Branch    string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}

type CoverageRunListItem struct {
	ID                   string  `json:"id"`
	Branch               string  `json:"branch"`
	CommitSHA            string  `json:"commitSha"`
	RunTimestamp         string  `json:"runTimestamp"`
	TotalCoveragePercent float64 `json:"totalCoveragePercent"`
}

type PaginationResponse struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

type ListCoverageRunsOutput struct {
	Items      []CoverageRunListItem `json:"items"`
	Pagination PaginationResponse    `json:"pagination"`
}

type ListCoverageRunsUseCase struct {
	runs CoverageRunRepository
}

func NewListCoverageRunsUseCase(runs CoverageRunRepository) *ListCoverageRunsUseCase {
	return &ListCoverageRunsUseCase{runs: runs}
}

func (uc *ListCoverageRunsUseCase) Execute(ctx context.Context, in ListCoverageRunsInput) (ListCoverageRunsOutput, error) {
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

	runs, total, err := uc.runs.ListByProject(ctx, in.ProjectID, in.Branch, in.From, in.To, page, pageSize)
	if err != nil {
		return ListCoverageRunsOutput{}, NewInternal("failed to list coverage runs", err)
	}

	items := make([]CoverageRunListItem, 0, len(runs))
	for _, run := range runs {
		items = append(items, CoverageRunListItem{
			ID:                   run.ID,
			Branch:               run.Branch,
			CommitSHA:            run.CommitSHA,
			RunTimestamp:         run.RunTimestamp.UTC().Format(time.RFC3339),
			TotalCoveragePercent: run.TotalCoveragePercent,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return ListCoverageRunsOutput{
		Items: items,
		Pagination: PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: total,
			TotalPages: totalPages,
		},
	}, nil
}

type GetLatestComparisonInput struct {
	ProjectID string
	Branch    string // optional; if empty, uses latest run across all branches
}

type LatestComparisonOutput struct {
	Run        RunResponse                 `json:"run"`
	Comparison ComparisonResponse          `json:"comparison"`
	Packages   []PackageComparisonResponse `json:"packages"`
}

type GetLatestComparisonUseCase struct {
	projects ProjectRepository
	runs     CoverageRunRepository
	packages PackageCoverageRepository
}

func NewGetLatestComparisonUseCase(
	projects ProjectRepository,
	runs CoverageRunRepository,
	packages PackageCoverageRepository,
) *GetLatestComparisonUseCase {
	return &GetLatestComparisonUseCase{projects: projects, runs: runs, packages: packages}
}

func (uc *GetLatestComparisonUseCase) Execute(ctx context.Context, in GetLatestComparisonInput) (LatestComparisonOutput, error) {
	project, err := uc.projects.GetByID(ctx, in.ProjectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return LatestComparisonOutput{}, NewNotFound("project not found", map[string]any{"projectId": in.ProjectID})
		}
		return LatestComparisonOutput{}, NewInternal("failed to load project", err)
	}

	var run domain.CoverageRun
	if in.Branch != "" {
		run, err = uc.runs.GetLatestByProjectAndBranch(ctx, in.ProjectID, in.Branch)
	} else {
		run, err = uc.runs.GetLatestByProject(ctx, in.ProjectID)
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return LatestComparisonOutput{}, NewNotFound("no coverage runs found", map[string]any{"projectId": in.ProjectID, "branch": in.Branch})
		}
		return LatestComparisonOutput{}, NewInternal("failed to load latest run", err)
	}

	currentPackages, err := uc.packages.ListByRunID(ctx, run.ID)
	if err != nil {
		return LatestComparisonOutput{}, NewInternal("failed to load package coverage", err)
	}

	baselineRun, err := uc.runs.GetLatestByProjectAndBranch(ctx, in.ProjectID, project.DefaultBranch)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return LatestComparisonOutput{}, NewInternal("failed to load baseline run", err)
	}

	var previousTotal *float64
	var baselinePackages []domain.PackageCoverage
	if err == nil {
		if baselineRun.ID != run.ID {
			p := baselineRun.TotalCoveragePercent
			previousTotal = &p
			baselinePackages, err = uc.packages.ListByRunID(ctx, baselineRun.ID)
			if err != nil {
				return LatestComparisonOutput{}, NewInternal("failed to load baseline package coverage", err)
			}
		}
	}

	return LatestComparisonOutput{
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
		Packages:   buildPackageComparisons(currentPackages, baselinePackages),
	}, nil
}

type ListBranchesOutput struct {
	Branches []string `json:"branches"`
}

type ListBranchesUseCase struct {
	runs CoverageRunRepository
}

func NewListBranchesUseCase(runs CoverageRunRepository) *ListBranchesUseCase {
	return &ListBranchesUseCase{runs: runs}
}

func (uc *ListBranchesUseCase) Execute(ctx context.Context, projectID string) (ListBranchesOutput, error) {
	branches, err := uc.runs.ListBranchesByProject(ctx, projectID)
	if err != nil {
		return ListBranchesOutput{}, NewInternal("failed to list branches", err)
	}

	return ListBranchesOutput{
		Branches: branches,
	}, nil
}
