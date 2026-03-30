-- +goose Up
CREATE TABLE IF NOT EXISTS projects (
  id UUID PRIMARY KEY,
  project_key TEXT NOT NULL UNIQUE,
  name TEXT,
  group_name TEXT DEFAULT NULL,
  default_branch TEXT NOT NULL DEFAULT 'main',
  global_threshold_percent NUMERIC(5,2) NOT NULL DEFAULT 80.00,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS coverage_runs (
  id UUID PRIMARY KEY,
  project_id UUID NOT NULL REFERENCES projects(id),
  branch TEXT NOT NULL,
  commit_sha TEXT NOT NULL,
  author TEXT,
  trigger_type TEXT NOT NULL CHECK (trigger_type IN ('push', 'pr', 'manual')),
  run_timestamp TIMESTAMPTZ NOT NULL,
  total_coverage_percent NUMERIC(5,2) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS coverage_runs_project_branch_ts_idx
  ON coverage_runs(project_id, branch, run_timestamp DESC);

CREATE INDEX IF NOT EXISTS coverage_runs_project_default_lookup_idx
  ON coverage_runs(project_id, run_timestamp DESC);

CREATE TABLE IF NOT EXISTS package_coverages (
  id UUID PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES coverage_runs(id) ON DELETE CASCADE,
  package_import_path TEXT NOT NULL,
  coverage_percent NUMERIC(5,2) NOT NULL
);

CREATE INDEX IF NOT EXISTS package_coverages_run_id_idx ON package_coverages(run_id);
CREATE INDEX IF NOT EXISTS package_coverages_import_path_idx ON package_coverages(package_import_path);

-- +goose Down
DROP TABLE IF EXISTS package_coverages;
DROP TABLE IF EXISTS coverage_runs;
DROP TABLE IF EXISTS projects;
