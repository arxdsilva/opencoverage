package bootstrap

import (
	"context"

	"github.com/arxdsilva/opencoverage/internal/adapters/auth"
	"github.com/arxdsilva/opencoverage/internal/adapters/postgres"
	"github.com/arxdsilva/opencoverage/internal/application"
	"github.com/arxdsilva/opencoverage/internal/platform/clock"
	"github.com/arxdsilva/opencoverage/internal/platform/config"
	"github.com/arxdsilva/opencoverage/internal/platform/idgen"
	"github.com/arxdsilva/opencoverage/internal/platform/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	Pool                        *pgxpool.Pool
	Authenticator               application.APIKeyAuthenticator
	IngestCoverageRun           *application.IngestCoverageRunUseCase
	IngestIntegrationRun        *application.IngestIntegrationRunUseCase
	ListProjects                *application.ListProjectsUseCase
	GetProject                  *application.GetProjectUseCase
	ListCoverageRuns            *application.ListCoverageRunsUseCase
	ListIntegrationRuns         *application.ListIntegrationRunsUseCase
	GetLatestComparison         *application.GetLatestComparisonUseCase
	GetLatestIntegrationCompare *application.GetLatestIntegrationComparisonUseCase
	GetIntegrationRun           *application.GetIntegrationRunUseCase
	GetIntegrationHeatmap       *application.GetIntegrationHeatmapUseCase
	ListBranches                *application.ListBranchesUseCase
	ListContributors            *application.ListContributorsUseCase
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := migrations.Up(ctx, cfg.DatabaseURL, cfg.MigrationsDir); err != nil {
		pool.Close()
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	projectRepo := postgres.NewProjectRepository(pool)
	runRepo := postgres.NewCoverageRunRepository(pool)
	packageRepo := postgres.NewPackageCoverageRepository(pool)
	integrationRunRepo := postgres.NewIntegrationTestRunRepository(pool)
	integrationSpecRepo := postgres.NewIntegrationSpecResultRepository(pool)
	txManager := postgres.NewTxManager(pool)
	authenticator := auth.NewEnvAPIKeyAuthenticator(cfg.APIKeySecret)

	clockAdapter := clock.NewSystemClock()
	idGenerator := idgen.NewUUIDGenerator()

	return &App{
		Pool:                        pool,
		Authenticator:               authenticator,
		IngestCoverageRun:           application.NewIngestCoverageRunUseCase(projectRepo, runRepo, packageRepo, txManager, idGenerator, clockAdapter),
		IngestIntegrationRun:        application.NewIngestIntegrationRunUseCase(projectRepo, integrationRunRepo, integrationSpecRepo, txManager, idGenerator, clockAdapter),
		ListProjects:                application.NewListProjectsUseCase(projectRepo),
		GetProject:                  application.NewGetProjectUseCase(projectRepo),
		ListCoverageRuns:            application.NewListCoverageRunsUseCase(runRepo),
		ListIntegrationRuns:         application.NewListIntegrationRunsUseCase(integrationRunRepo),
		GetLatestComparison:         application.NewGetLatestComparisonUseCase(projectRepo, runRepo, packageRepo),
		GetLatestIntegrationCompare: application.NewGetLatestIntegrationComparisonUseCase(projectRepo, integrationRunRepo, integrationSpecRepo),
		GetIntegrationRun:           application.NewGetIntegrationRunUseCase(integrationRunRepo, integrationSpecRepo),
		GetIntegrationHeatmap:       application.NewGetIntegrationHeatmapUseCase(integrationRunRepo),
		ListBranches:                application.NewListBranchesUseCase(runRepo),
		ListContributors:            application.NewListContributorsUseCase(projectRepo, runRepo),
	}, nil
}

func (a *App) Close() {
	if a == nil || a.Pool == nil {
		return
	}
	a.Pool.Close()
}