package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/arxdsilva/coverage-api/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectRepository struct {
	pool *pgxpool.Pool
}

func NewProjectRepository(pool *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{pool: pool}
}

func (r *ProjectRepository) GetByKey(ctx context.Context, projectKey string) (domain.Project, error) {
	q := getQuerier(ctx, r.pool)
	var p domain.Project
	err := q.QueryRow(ctx, `
		SELECT id, project_key, COALESCE(name, ''), default_branch, global_threshold_percent, created_at, updated_at
		FROM projects
		WHERE project_key = $1
	`, projectKey).Scan(
		&p.ID,
		&p.ProjectKey,
		&p.Name,
		&p.DefaultBranch,
		&p.GlobalThresholdPercent,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Project{}, domain.ErrNotFound
		}
		return domain.Project{}, fmt.Errorf("query project by key: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) GetByID(ctx context.Context, projectID string) (domain.Project, error) {
	q := getQuerier(ctx, r.pool)
	var p domain.Project
	err := q.QueryRow(ctx, `
		SELECT id, project_key, COALESCE(name, ''), default_branch, global_threshold_percent, created_at, updated_at
		FROM projects
		WHERE id = $1
	`, projectID).Scan(
		&p.ID,
		&p.ProjectKey,
		&p.Name,
		&p.DefaultBranch,
		&p.GlobalThresholdPercent,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Project{}, domain.ErrNotFound
		}
		return domain.Project{}, fmt.Errorf("query project by id: %w", err)
	}
	return p, nil
}

func (r *ProjectRepository) Create(ctx context.Context, project domain.Project) (domain.Project, error) {
	q := getQuerier(ctx, r.pool)
	_, err := q.Exec(ctx, `
		INSERT INTO projects (id, project_key, name, default_branch, global_threshold_percent, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		project.ID,
		project.ProjectKey,
		project.Name,
		project.DefaultBranch,
		project.GlobalThresholdPercent,
		project.CreatedAt,
		project.UpdatedAt,
	)
	if err != nil {
		return domain.Project{}, fmt.Errorf("insert project: %w", err)
	}
	return project, nil
}

func (r *ProjectRepository) List(ctx context.Context) ([]domain.Project, error) {
	q := getQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, `
		SELECT id, project_key, COALESCE(name, ''), default_branch, global_threshold_percent, created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]domain.Project, 0)
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(
			&p.ID,
			&p.ProjectKey,
			&p.Name,
			&p.DefaultBranch,
			&p.GlobalThresholdPercent,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}
