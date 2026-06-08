package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"
	"github.com/arxdsilva/opencoverage/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type E2ETestRunRepository struct {
	pool *pgxpool.Pool
}

func NewE2ETestRunRepository(pool *pgxpool.Pool) *E2ETestRunRepository {
	return &E2ETestRunRepository{pool: pool}
}

func (r *E2ETestRunRepository) Create(ctx context.Context, run domain.E2ETestRun) (domain.E2ETestRun, error) {
	q := getQuerier(ctx, r.pool)
	_, err := q.Exec(ctx, `
		INSERT INTO e2e_test_runs (
			id, project_id, branch, commit_sha, author, trigger_type, run_timestamp,
			framework_version, test_framework, platform, suite_description, suite_path, total_specs, passed_specs,
			failed_specs, skipped_specs, flaked_specs, pending_specs, interrupted,
			timed_out, duration_ms, status, environment, created_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22, $23, $24
		)
	`,
		run.ID,
		run.ProjectID,
		run.Branch,
		run.CommitSHA,
		run.Author,
		run.TriggerType,
		run.RunTimestamp,
		run.FrameworkVersion,
		run.TestFramework,
		run.PlatformType,
		run.SuiteDescription,
		run.SuitePath,
		run.TotalSpecs,
		run.PassedSpecs,
		run.FailedSpecs,
		run.SkippedSpecs,
		run.FlakedSpecs,
		run.PendingSpecs,
		run.Interrupted,
		run.TimedOut,
		run.DurationMS,
		run.Status,
		run.Environment,
		run.CreatedAt,
	)
	if err != nil {
		return domain.E2ETestRun{}, fmt.Errorf("insert e2e test run: %w", err)
	}
	return run, nil
}

func (r *E2ETestRunRepository) GetLatestByProjectAndBranch(ctx context.Context, projectID string, branch string) (domain.E2ETestRun, error) {
	q := getQuerier(ctx, r.pool)
	var run domain.E2ETestRun
	err := q.QueryRow(ctx, `
		SELECT id, project_id, branch, commit_sha, COALESCE(author, ''), trigger_type, run_timestamp,
			COALESCE(framework_version, ''), COALESCE(test_framework, ''), COALESCE(platform::text, ''), suite_description, suite_path, total_specs, passed_specs,
			failed_specs, skipped_specs, flaked_specs, pending_specs, interrupted, timed_out,
			duration_ms, status, environment, created_at
		FROM e2e_test_runs
		WHERE project_id = $1 AND branch = $2
		ORDER BY run_timestamp DESC, created_at DESC
		LIMIT 1
	`, projectID, branch).Scan(
		&run.ID,
		&run.ProjectID,
		&run.Branch,
		&run.CommitSHA,
		&run.Author,
		&run.TriggerType,
		&run.RunTimestamp,
		&run.FrameworkVersion,
		&run.TestFramework,
		&run.PlatformType,
		&run.SuiteDescription,
		&run.SuitePath,
		&run.TotalSpecs,
		&run.PassedSpecs,
		&run.FailedSpecs,
		&run.SkippedSpecs,
		&run.FlakedSpecs,
		&run.PendingSpecs,
		&run.Interrupted,
		&run.TimedOut,
		&run.DurationMS,
		&run.Status,
		&run.Environment,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.E2ETestRun{}, domain.ErrNotFound
		}
		return domain.E2ETestRun{}, fmt.Errorf("query latest e2e run by project and branch: %w", err)
	}
	return run, nil
}

func (r *E2ETestRunRepository) GetLatestByProject(ctx context.Context, projectID string) (domain.E2ETestRun, error) {
	q := getQuerier(ctx, r.pool)
	var run domain.E2ETestRun
	err := q.QueryRow(ctx, `
		SELECT id, project_id, branch, commit_sha, COALESCE(author, ''), trigger_type, run_timestamp,
			COALESCE(framework_version, ''), COALESCE(test_framework, ''), COALESCE(platform::text, ''), suite_description, suite_path, total_specs, passed_specs,
			failed_specs, skipped_specs, flaked_specs, pending_specs, interrupted, timed_out,
			duration_ms, status, environment, created_at
		FROM e2e_test_runs
		WHERE project_id = $1
		ORDER BY run_timestamp DESC, created_at DESC
		LIMIT 1
	`, projectID).Scan(
		&run.ID,
		&run.ProjectID,
		&run.Branch,
		&run.CommitSHA,
		&run.Author,
		&run.TriggerType,
		&run.RunTimestamp,
		&run.FrameworkVersion,
		&run.TestFramework,
		&run.PlatformType,
		&run.SuiteDescription,
		&run.SuitePath,
		&run.TotalSpecs,
		&run.PassedSpecs,
		&run.FailedSpecs,
		&run.SkippedSpecs,
		&run.FlakedSpecs,
		&run.PendingSpecs,
		&run.Interrupted,
		&run.TimedOut,
		&run.DurationMS,
		&run.Status,
		&run.Environment,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.E2ETestRun{}, domain.ErrNotFound
		}
		return domain.E2ETestRun{}, fmt.Errorf("query latest e2e run by project: %w", err)
	}
	return run, nil
}

func (r *E2ETestRunRepository) GetByID(ctx context.Context, projectID string, runID string) (domain.E2ETestRun, error) {
	q := getQuerier(ctx, r.pool)
	var run domain.E2ETestRun
	err := q.QueryRow(ctx, `
		SELECT id, project_id, branch, commit_sha, COALESCE(author, ''), trigger_type, run_timestamp,
			COALESCE(framework_version, ''), COALESCE(test_framework, ''), COALESCE(platform::text, ''), suite_description, suite_path, total_specs, passed_specs,
			failed_specs, skipped_specs, flaked_specs, pending_specs, interrupted, timed_out,
			duration_ms, status, environment, created_at
		FROM e2e_test_runs
		WHERE project_id = $1 AND id = $2
		LIMIT 1
	`, projectID, runID).Scan(
		&run.ID,
		&run.ProjectID,
		&run.Branch,
		&run.CommitSHA,
		&run.Author,
		&run.TriggerType,
		&run.RunTimestamp,
		&run.FrameworkVersion,
		&run.TestFramework,
		&run.PlatformType,
		&run.SuiteDescription,
		&run.SuitePath,
		&run.TotalSpecs,
		&run.PassedSpecs,
		&run.FailedSpecs,
		&run.SkippedSpecs,
		&run.FlakedSpecs,
		&run.PendingSpecs,
		&run.Interrupted,
		&run.TimedOut,
		&run.DurationMS,
		&run.Status,
		&run.Environment,
		&run.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.E2ETestRun{}, domain.ErrNotFound
		}
		return domain.E2ETestRun{}, fmt.Errorf("query e2e run by id: %w", err)
	}
	return run, nil
}

func (r *E2ETestRunRepository) ListByProject(ctx context.Context, projectID string, branch string, status string, environment string, from *time.Time, to *time.Time, page int, pageSize int) ([]domain.E2ETestRun, int, error) {
	q := getQuerier(ctx, r.pool)
	offset := (page - 1) * pageSize

	where := "WHERE project_id = $1"
	args := []any{projectID}
	idx := 2

	if branch != "" {
		where += fmt.Sprintf(" AND branch = $%d", idx)
		args = append(args, branch)
		idx++
	}
	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if environment != "" {
		if environment == "none" {
			where += " AND environment IS NULL"
		} else {
			where += fmt.Sprintf(" AND environment = $%d", idx)
			args = append(args, environment)
			idx++
		}
	}
	if from != nil {
		where += fmt.Sprintf(" AND run_timestamp >= $%d", idx)
		args = append(args, *from)
		idx++
	}
	if to != nil {
		where += fmt.Sprintf(" AND run_timestamp <= $%d", idx)
		args = append(args, *to)
		idx++
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM e2e_test_runs %s", where)
	var total int
	if err := q.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count e2e runs: %w", err)
	}

	listSQL := fmt.Sprintf(`
		SELECT id, project_id, branch, commit_sha, COALESCE(author, ''), trigger_type, run_timestamp,
			COALESCE(framework_version, ''), COALESCE(test_framework, ''), COALESCE(platform::text, ''), suite_description, suite_path, total_specs, passed_specs,
			failed_specs, skipped_specs, flaked_specs, pending_specs, interrupted, timed_out,
			duration_ms, status, environment, created_at
		FROM e2e_test_runs
		%s
		ORDER BY run_timestamp DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)
	args = append(args, pageSize, offset)

	rows, err := q.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list e2e runs: %w", err)
	}
	defer rows.Close()

	runs := make([]domain.E2ETestRun, 0)
	for rows.Next() {
		var run domain.E2ETestRun
		if err := rows.Scan(
			&run.ID,
			&run.ProjectID,
			&run.Branch,
			&run.CommitSHA,
			&run.Author,
			&run.TriggerType,
			&run.RunTimestamp,
			&run.FrameworkVersion,
			&run.TestFramework,
			&run.PlatformType,
			&run.SuiteDescription,
			&run.SuitePath,
			&run.TotalSpecs,
			&run.PassedSpecs,
			&run.FailedSpecs,
			&run.SkippedSpecs,
			&run.FlakedSpecs,
			&run.PendingSpecs,
			&run.Interrupted,
			&run.TimedOut,
			&run.DurationMS,
			&run.Status,
			&run.Environment,
			&run.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan e2e run: %w", err)
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate e2e run rows: %w", err)
	}

	return runs, total, nil
}

func (r *E2ETestRunRepository) HeatmapData(ctx context.Context, branch string, status string, runsPerProject int) ([]application.TestHeatmapRow, error) {
	q := getQuerier(ctx, r.pool)

	where := "WHERE 1=1"
	args := []any{}
	idx := 1

	if branch != "" {
		where += fmt.Sprintf(" AND itr.branch = $%d", idx)
		args = append(args, branch)
		idx++
	}
	if status != "" {
		where += fmt.Sprintf(" AND itr.status = $%d", idx)
		args = append(args, status)
		idx++
	}

	args = append(args, runsPerProject)

	sql := fmt.Sprintf(`
		WITH ranked AS (
			SELECT
				itr.id              AS run_id,
				itr.project_id,
				itr.branch,
				itr.commit_sha,
				itr.run_timestamp,
				itr.passed_specs,
				itr.total_specs,
				itr.status,
				itr.environment,
				COALESCE(p.name, '')        AS project_name,
				p.project_key,
				COALESCE(p.group_name, '')  AS project_group,
				ROW_NUMBER() OVER (
					PARTITION BY itr.project_id
					ORDER BY itr.run_timestamp DESC, itr.created_at DESC
				) AS rn
			FROM e2e_test_runs itr
			JOIN projects p ON p.id = itr.project_id
			%s
		)
		SELECT run_id, project_id, project_name, project_key, project_group,
		       branch, commit_sha, run_timestamp, passed_specs, total_specs, status, environment
		FROM ranked
		WHERE rn <= $%d
		ORDER BY
			CASE WHEN project_group = '' THEN 1 ELSE 0 END ASC,
			project_group ASC,
			project_name ASC,
			run_timestamp DESC
	`, where, idx)

	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query heatmap data: %w", err)
	}
	defer rows.Close()

	result := make([]application.TestHeatmapRow, 0)
	for rows.Next() {
		var row application.TestHeatmapRow
		if err := rows.Scan(
			&row.RunID,
			&row.ProjectID,
			&row.ProjectName,
			&row.ProjectKey,
			&row.ProjectGroup,
			&row.Branch,
			&row.CommitSHA,
			&row.RunTimestamp,
			&row.PassedSpecs,
			&row.TotalSpecs,
			&row.Status,
			&row.Environment,
		); err != nil {
			return nil, fmt.Errorf("scan heatmap row: %w", err)
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate heatmap rows: %w", err)
	}

	return result, nil
}
