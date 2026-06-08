-- +goose Up

CREATE TABLE IF NOT EXISTS e2e_test_runs (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id),
  branch TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  author TEXT,
  trigger_type TEXT NOT NULL,
  run_timestamp TIMESTAMPTZ NOT NULL,
  framework_version TEXT,
  test_framework TEXT,
  platform TEXT,
  suite_description TEXT NOT NULL,
  suite_path TEXT NOT NULL,
  total_specs INTEGER NOT NULL,
  passed_specs INTEGER NOT NULL,
  failed_specs INTEGER NOT NULL,
  skipped_specs INTEGER NOT NULL,
  flaked_specs INTEGER NOT NULL,
  pending_specs INTEGER NOT NULL,
  interrupted BOOLEAN NOT NULL DEFAULT FALSE,
  timed_out BOOLEAN NOT NULL DEFAULT FALSE,
  duration_ms BIGINT NOT NULL,
  status TEXT NOT NULL,
  environment environment_type,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS e2e_test_runs_project_branch_ts_idx
  ON e2e_test_runs(project_id, branch, run_timestamp DESC);

CREATE INDEX IF NOT EXISTS e2e_test_runs_project_default_lookup_idx
  ON e2e_test_runs(project_id, run_timestamp DESC);
  
CREATE INDEX IF NOT EXISTS e2e_test_runs_project_status_ts_idx
  ON e2e_test_runs(project_id, status, run_timestamp DESC);

ALTER TABLE e2e_test_runs
  ADD CONSTRAINT chk_e2e_test_runs_trigger_type CHECK (trigger_type IN ('push', 'pr', 'manual'));

ALTER TABLE e2e_test_runs
  ADD CONSTRAINT chk_e2e_test_runs_platform CHECK (platform IN ('web', 'android', 'ios'));

ALTER TABLE e2e_test_runs
  ADD CONSTRAINT chk_e2e_test_runs_status CHECK (status IN ('passed', 'failed'));

CREATE TABLE IF NOT EXISTS e2e_test_spec_results (
  id UUID PRIMARY KEY,
  e2e_run_id UUID NOT NULL REFERENCES e2e_test_runs(id) ON DELETE CASCADE,
  spec_path TEXT NOT NULL,
  leaf_node_text TEXT NOT NULL,
  state TEXT NOT NULL,
  duration_ms BIGINT NOT NULL,
  failure_message TEXT,
  failure_location_file TEXT,
  failure_location_line INTEGER
);

CREATE INDEX IF NOT EXISTS e2e_test_spec_results_run_id_idx ON e2e_test_spec_results(e2e_run_id);
CREATE INDEX IF NOT EXISTS e2e_test_spec_results_state_idx ON e2e_test_spec_results(state);

ALTER TABLE e2e_test_spec_results
  ADD CONSTRAINT chk_e2e_test_spec_results_state CHECK (state IN ('passed', 'failed', 'skipped', 'pending', 'flaky'));

-- +goose Down
DROP TABLE IF EXISTS e2e_test_spec_results;
DROP TABLE IF EXISTS e2e_test_runs;