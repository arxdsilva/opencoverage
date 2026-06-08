package postgres

import (
	"context"
	"fmt"

	"github.com/arxdsilva/opencoverage/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type E2ESpecResultRepository struct {
	pool *pgxpool.Pool
}

func NewE2ESpecResultRepository(pool *pgxpool.Pool) *E2ESpecResultRepository {
	return &E2ESpecResultRepository{pool: pool}
}

func (r *E2ESpecResultRepository) CreateBatch(ctx context.Context, specs []domain.E2ESpecResult) error {
	if len(specs) == 0 {
		return nil
	}

	q := getQuerier(ctx, r.pool)
	for _, spec := range specs {
		_, err := q.Exec(ctx, `
			INSERT INTO e2e_test_spec_results (
				id, e2e_run_id, spec_path, leaf_node_text, state, duration_ms,
				failure_message, failure_location_file, failure_location_line
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			spec.ID,
			spec.E2ETestRunID,
			spec.SpecPath,
			spec.LeafNodeText,
			spec.State,
			spec.DurationMS,
			spec.FailureMessage,
			spec.FailureLocationFile,
			spec.FailureLocationLine,
		)
		if err != nil {
			return fmt.Errorf("insert e2e spec result: %w", err)
		}
	}

	return nil
}

func (r *E2ESpecResultRepository) ListByRunID(ctx context.Context, runID string) ([]domain.E2ESpecResult, error) {
	q := getQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, `
		SELECT id, e2e_run_id, spec_path, leaf_node_text, state, duration_ms,
			failure_message, failure_location_file, failure_location_line
		FROM e2e_test_spec_results
		WHERE e2e_run_id = $1
		ORDER BY spec_path ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("query e2e spec results: %w", err)
	}
	defer rows.Close()

	specs := make([]domain.E2ESpecResult, 0)
	for rows.Next() {
		var spec domain.E2ESpecResult
		if err := rows.Scan(
			&spec.ID,
			&spec.E2ETestRunID,
			&spec.SpecPath,
			&spec.LeafNodeText,
			&spec.State,
			&spec.DurationMS,
			&spec.FailureMessage,
			&spec.FailureLocationFile,
			&spec.FailureLocationLine,
		); err != nil {
			return nil, fmt.Errorf("scan e2e spec result: %w", err)
		}
		specs = append(specs, spec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate e2e spec rows: %w", err)
	}

	return specs, nil
}

func (r *E2ESpecResultRepository) ListFailedByRunID(ctx context.Context, runID string) ([]domain.E2ESpecResult, error) {
	q := getQuerier(ctx, r.pool)
	rows, err := q.Query(ctx, `
		SELECT id, e2e_run_id, spec_path, leaf_node_text, state, duration_ms,
			failure_message, failure_location_file, failure_location_line
		FROM e2e_test_spec_results
		WHERE e2e_run_id = $1 AND state IN ('failed', 'flaky')
		ORDER BY spec_path ASC
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("query failed e2e spec results: %w", err)
	}
	defer rows.Close()

	specs := make([]domain.E2ESpecResult, 0)
	for rows.Next() {
		var spec domain.E2ESpecResult
		if err := rows.Scan(
			&spec.ID,
			&spec.E2ETestRunID,
			&spec.SpecPath,
			&spec.LeafNodeText,
			&spec.State,
			&spec.DurationMS,
			&spec.FailureMessage,
			&spec.FailureLocationFile,
			&spec.FailureLocationLine,
		); err != nil {
			return nil, fmt.Errorf("scan failed e2e spec result: %w", err)
		}
		specs = append(specs, spec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failed e2e spec rows: %w", err)
	}

	return specs, nil
}
