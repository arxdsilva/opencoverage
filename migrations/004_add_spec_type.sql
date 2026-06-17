-- +goose Up

ALTER TABLE e2e_test_spec_results
  ADD COLUMN spec_type TEXT NOT NULL DEFAULT '' CHECK (spec_type IN ('setup', 'happyPath', 'negativePath', '')) ;

CREATE INDEX IF NOT EXISTS e2e_test_spec_results_spec_type_idx
  ON e2e_test_spec_results(e2e_run_id, spec_type) ;

-- +goose Down

DROP INDEX IF EXISTS e2e_test_spec_results_spec_type_idx;

ALTER TABLE e2e_test_spec_results
  DROP COLUMN IF EXISTS spec_type;