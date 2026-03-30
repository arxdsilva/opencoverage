package application

import (
	"context"
	"errors"

	"github.com/arxdsilva/coverage-api/internal/domain"
)

type GetProjectUseCase struct {
	projects ProjectRepository
}

type ListProjectsInput struct {
	Page     int
	PageSize int
}

type ListProjectsOutput struct {
	Items      []ProjectResponse  `json:"items"`
	Pagination PaginationResponse `json:"pagination"`
}

type ListProjectsUseCase struct {
	projects ProjectRepository
}

func NewGetProjectUseCase(projects ProjectRepository) *GetProjectUseCase {
	return &GetProjectUseCase{projects: projects}
}

func NewListProjectsUseCase(projects ProjectRepository) *ListProjectsUseCase {
	return &ListProjectsUseCase{projects: projects}
}

func (uc *GetProjectUseCase) Execute(ctx context.Context, projectID string) (ProjectResponse, error) {
	project, err := uc.projects.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return ProjectResponse{}, NewNotFound("project not found", map[string]any{"projectId": projectID})
		}
		return ProjectResponse{}, NewInternal("failed to load project", err)
	}

	return ProjectResponse{
		ID:                     project.ID,
		ProjectKey:             project.ProjectKey,
		Name:                   project.Name,
		Group:                  project.Group,
		DefaultBranch:          project.DefaultBranch,
		GlobalThresholdPercent: project.GlobalThresholdPercent,
		Created:                false,
	}, nil
}

func (uc *ListProjectsUseCase) Execute(ctx context.Context, in ListProjectsInput) (ListProjectsOutput, error) {
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

	projects, total, err := uc.projects.List(ctx, page, pageSize)
	if err != nil {
		return ListProjectsOutput{}, NewInternal("failed to list projects", err)
	}

	items := make([]ProjectResponse, 0, len(projects))
	for _, project := range projects {
		items = append(items, ProjectResponse{
			ID:                     project.ID,
			ProjectKey:             project.ProjectKey,
			Name:                   project.Name,
			Group:                  project.Group,
			DefaultBranch:          project.DefaultBranch,
			GlobalThresholdPercent: project.GlobalThresholdPercent,
			Created:                false,
		})
	}

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	return ListProjectsOutput{
		Items: items,
		Pagination: PaginationResponse{
			Page:       page,
			PageSize:   pageSize,
			TotalItems: total,
			TotalPages: totalPages,
		},
	}, nil
}
