package application

import (
	"context"
	"errors"

	"github.com/arxdsilva/coverage-api/internal/domain"
)

type GetProjectUseCase struct {
	projects ProjectRepository
}

type ListProjectsOutput struct {
	Items []ProjectResponse `json:"items"`
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
		DefaultBranch:          project.DefaultBranch,
		GlobalThresholdPercent: project.GlobalThresholdPercent,
		Created:                false,
	}, nil
}

func (uc *ListProjectsUseCase) Execute(ctx context.Context) (ListProjectsOutput, error) {
	projects, err := uc.projects.List(ctx)
	if err != nil {
		return ListProjectsOutput{}, NewInternal("failed to list projects", err)
	}

	items := make([]ProjectResponse, 0, len(projects))
	for _, project := range projects {
		items = append(items, ProjectResponse{
			ID:                     project.ID,
			ProjectKey:             project.ProjectKey,
			Name:                   project.Name,
			DefaultBranch:          project.DefaultBranch,
			GlobalThresholdPercent: project.GlobalThresholdPercent,
			Created:                false,
		})
	}

	return ListProjectsOutput{Items: items}, nil
}
