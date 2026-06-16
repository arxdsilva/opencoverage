package application

import (
	"context"
	"time"

	"github.com/arxdsilva/opencoverage/internal/domain"
)

// --- E2ETestRunRepository stub ---

type stubE2ETestRunRepository struct {
	created   domain.E2ETestRun
	createErr error

	latestByBranch    *domain.E2ETestRun
	latestByBranchErr error

	latestByProject    *domain.E2ETestRun
	latestByProjectErr error

	byID    *domain.E2ETestRun
	byIDErr error

	listed    []domain.E2ETestRun
	listTotal int
	listErr   error

	heatmapRows []TestHeatmapRow
	heatmapErr  error

	// captured args for assertions
	capturedBranch string
	capturedStatus string
}

func (s *stubE2ETestRunRepository) Create(ctx context.Context, run domain.E2ETestRun) (domain.E2ETestRun, error) {
	if s.createErr != nil {
		return domain.E2ETestRun{}, s.createErr
	}
	s.created = run
	return run, nil
}

func (s *stubE2ETestRunRepository) GetLatestByProjectAndBranch(ctx context.Context, projectID string, branch string) (domain.E2ETestRun, error) {
	s.capturedBranch = branch
	if s.latestByBranchErr != nil {
		return domain.E2ETestRun{}, s.latestByBranchErr
	}
	if s.latestByBranch == nil {
		return domain.E2ETestRun{}, domain.ErrNotFound
	}
	return *s.latestByBranch, nil
}

func (s *stubE2ETestRunRepository) GetLatestByProject(ctx context.Context, projectID string) (domain.E2ETestRun, error) {
	if s.latestByProjectErr != nil {
		return domain.E2ETestRun{}, s.latestByProjectErr
	}
	if s.latestByProject == nil {
		return domain.E2ETestRun{}, domain.ErrNotFound
	}
	return *s.latestByProject, nil
}

func (s *stubE2ETestRunRepository) GetByID(ctx context.Context, projectID string, runID string) (domain.E2ETestRun, error) {
	if s.byIDErr != nil {
		return domain.E2ETestRun{}, s.byIDErr
	}
	if s.byID == nil {
		return domain.E2ETestRun{}, domain.ErrNotFound
	}
	return *s.byID, nil
}

func (s *stubE2ETestRunRepository) ListByProject(ctx context.Context, projectID string, branch string, status string, environment string, specType string, from *time.Time, to *time.Time, page int, pageSize int) ([]domain.E2ETestRun, int, error) {
	s.capturedBranch = branch
	s.capturedStatus = status
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.listed, s.listTotal, nil
}

func (s *stubE2ETestRunRepository) HeatmapData(ctx context.Context, branch string, status string, specType string, runsPerProject int) ([]TestHeatmapRow, error) {
	s.capturedBranch = branch
	s.capturedStatus = status
	if s.heatmapErr != nil {
		return nil, s.heatmapErr
	}
	return s.heatmapRows, nil
}

// --- E2ESpecResultRepository stub ---

type stubE2ESpecResultRepository struct {
	createBatchErr error
	createdSpecs   []domain.E2ESpecResult

	byRunID    []domain.E2ESpecResult
	byRunIDErr error

	failedByRunID    []domain.E2ESpecResult
	failedByRunIDErr error
}

func (s *stubE2ESpecResultRepository) CreateBatch(ctx context.Context, specs []domain.E2ESpecResult) error {
	if s.createBatchErr != nil {
		return s.createBatchErr
	}
	s.createdSpecs = specs
	return nil
}

func (s *stubE2ESpecResultRepository) ListByRunID(ctx context.Context, runID string) ([]domain.E2ESpecResult, error) {
	if s.byRunIDErr != nil {
		return nil, s.byRunIDErr
	}
	return s.byRunID, nil
}

func (s *stubE2ESpecResultRepository) ListFailedByRunID(ctx context.Context, runID string) ([]domain.E2ESpecResult, error) {
	if s.failedByRunIDErr != nil {
		return nil, s.failedByRunIDErr
	}
	return s.failedByRunID, nil
}
