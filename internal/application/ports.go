package application

import (
	"context"
	"time"

	"github.com/arxdsilva/coverage-api/internal/domain"
)

type ProjectRepository interface {
	GetByKey(ctx context.Context, projectKey string) (domain.Project, error)
	GetByID(ctx context.Context, projectID string) (domain.Project, error)
	List(ctx context.Context) ([]domain.Project, error)
	Create(ctx context.Context, project domain.Project) (domain.Project, error)
}

type CoverageRunRepository interface {
	Create(ctx context.Context, run domain.CoverageRun) (domain.CoverageRun, error)
	GetLatestByProjectAndBranch(ctx context.Context, projectID string, branch string) (domain.CoverageRun, error)
	GetLatestByProject(ctx context.Context, projectID string) (domain.CoverageRun, error)
	ListByProject(
		ctx context.Context,
		projectID string,
		branch string,
		from *time.Time,
		to *time.Time,
		page int,
		pageSize int,
	) ([]domain.CoverageRun, int, error)
}

type PackageCoverageRepository interface {
	CreateBatch(ctx context.Context, packages []domain.PackageCoverage) error
	ListByRunID(ctx context.Context, runID string) ([]domain.PackageCoverage, error)
}

type APIKeyAuthenticator interface {
	Authenticate(ctx context.Context, apiKey string) error
	WantedAPIKey() string
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type TransactionManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
