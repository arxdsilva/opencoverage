# NPM/Vitest Coverage Ingestion Specification

## 1. Overview

### 1.1 Purpose

Define how coverage-api ingests JavaScript/TypeScript test coverage produced by Vitest using `json-summary`, while preserving the current REST contract and hexagonal architecture.

### 1.2 Primary Outcome

Allow teams running npm-based tests to publish coverage into the same project/run history model currently used for Go coverage runs.

### 1.3 Canonical Producer Command

The source artifact for this spec is generated with:

```bash
npx vitest run --coverage --coverage.reporter=lcov --coverage.reporter=json-summary
```

Expected artifact:

```text
coverage/coverage-summary.json
```

## 2. Scope

### 2.1 In Scope (Phase 1)

1. Ingest `coverage-summary.json` from Vitest/Istanbul.
2. Convert summary content into existing `POST /v1/coverage-runs` request shape.
3. Reuse baseline comparison logic, threshold evaluation, and package delta response model.
4. Extend existing CLI tooling to support npm coverage upload flow.
5. Document CI usage for npm projects.

### 2.2 Out of Scope (Phase 1)

1. File-level diff API responses.
2. Function/branch delta visualization in frontend.
3. Mixed-language single-run uploads (Go + npm in one payload).
4. Raw LCOV ingestion at API boundary.

## 3. Current-State Constraints

1. `POST /v1/coverage-runs` requires:
   - `totalCoveragePercent`
   - non-empty `packages[]` with unique `importPath`
2. Existing comparison engine is metric-agnostic and only needs percentages in `[0, 100]`.
3. Existing package concept uses `importPath` as a stable key; for npm this will represent normalized module path segments.

Implication: Phase 1 can be implemented without API schema or database changes.

## 4. Input Contract: Vitest `json-summary`

### 4.1 Shape

Top-level object contains:

1. `total`: aggregate metrics (`lines`, `statements`, `functions`, `branches`, optional `branchesTrue`).
2. Per-file keys: absolute or relative file paths, each containing the same metric objects.

### 4.2 Metric Object

Each metric contains:

1. `total`
2. `covered`
3. `skipped`
4. `pct`

### 4.3 Observed Real-World Characteristics

1. File keys may be absolute paths (machine-specific).
2. Non-source files can appear (setup scripts, generated route files, e2e helpers).
3. Some files can have `total=0` for a metric.

## 5. Mapping to Existing Coverage API Payload

### 5.1 Mapping Strategy (Phase 1 Default)

1. `totalCoveragePercent` := `summary.total.lines.pct`
2. `packages[]` built from per-file entries, grouped by normalized directory path.
3. Each package coverage is weighted by line totals.

Weighted formula for each package:

$$
packagePct = \frac{\sum coveredLines}{\sum totalLines} \times 100
$$

### 5.2 Why `lines.pct` First

1. Compatible with existing UI assumptions (single overall percentage).
2. Most intuitive and stable for threshold gating.
3. Avoids early backend contract changes.

### 5.3 Normalizing Package Keys for npm

1. Convert Windows separators to `/`.
2. Trim repository root prefix when present.
3. Convert to directory key (for example `src/features/selectPlan`).
4. Exclude top-level `total` pseudo-key.
5. Deduplicate after normalization.

### 5.4 Filtering Rules (Default)

1. Ignore files outside configured include globs.
2. Ignore files matching exclude globs.
3. Ignore entries with `lines.total == 0`.
4. Fail if no valid files remain after filtering.

## 6. Tooling Adaptation Plan

## 6.1 Extend Existing CLI (Preferred)

Extend `coveragecli` with a new subcommand:

```bash
go run ./cmd/coveragecli npm-upload \
  -vitest-summary coverage/coverage-summary.json \
  -api-url http://localhost:8080/v1/coverage-runs \
  -api-key dev-local-key \
  -project-key org/customer-portal \
  -project-name customer-portal \
  -project-group frontend \
  -default-branch main \
  -branch main \
  -commit-sha a1b2c3d4 \
  -author alice \
  -trigger-type push
```

### 6.2 Required Flags for `npm-upload`

1. `-vitest-summary` (required)
2. Shared metadata flags reused from current coverage flow:
   - `-project-key`
   - `-project-name`
   - `-project-group`
   - `-default-branch`
   - `-branch`
   - `-commit-sha`
   - `-author`
   - `-trigger-type`
3. Upload flags:
   - `-api-url`
   - `-api-key`
   - `-api-key-header`

### 6.3 New Optional Flags

1. `-metric` with values: `lines|statements|functions|branches` (default `lines`).
2. `-group-by` with values: `dir|file` (default `dir`).
3. `-path-strip-prefix` for deterministic key normalization.
4. `-include-glob` (repeatable).
5. `-exclude-glob` (repeatable).
6. `-dry-run` to write payload locally without upload.

### 6.4 CLI Behavior Requirements

1. Parse and validate JSON shape before upload.
2. Print summary before send:
   - selected metric
   - total pct
   - considered file count
   - generated package count
3. Return non-zero on validation/auth/transport failures.
4. Reuse existing API key header semantics.

## 7. API Changes

### 7.1 Phase 1

No API request/response schema changes required.

### 7.2 Optional Phase 2 (If Needed)

Add optional metadata fields to coverage run ingest input:

1. `coverageTool` (for example `vitest`)
2. `coverageMetric` (for example `lines`)
3. `coverageFormat` (for example `json-summary`)

These fields would be informational and must not alter comparison math for already-ingested runs.

## 8. Validation Rules for `npm-upload`

1. JSON must contain `total` object and selected metric with `pct`.
2. Selected metric `pct` must be within `[0, 100]`.
3. At least one non-`total` file entry must exist before filtering.
4. At least one file must remain after filtering.
5. Generated `packages[]` must be non-empty and have unique keys.
6. Each generated package percentage must be within `[0, 100]`.

## 9. Error Model

The CLI should emit actionable errors with stable codes:

1. `ERR_INPUT_READ`
2. `ERR_INPUT_PARSE`
3. `ERR_INPUT_SCHEMA`
4. `ERR_EMPTY_DATASET`
5. `ERR_UPLOAD_FAILED`

Server-side error payload remains unchanged:

```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "packages is required",
    "details": {
      "field": "packages"
    }
  }
}
```

## 10. CI/CD Contract for npm Projects

### 10.1 Recommended CI Steps

1. Install dependencies.
2. Run Vitest with both reporters (`lcov` + `json-summary`).
3. Run `coveragecli npm-upload`.
4. Optionally gate pipeline using API response threshold status.

### 10.2 Example GitHub Actions Fragment

```yaml
- name: Run npm tests with coverage
  run: npx vitest run --coverage --coverage.reporter=lcov --coverage.reporter=json-summary

- name: Upload coverage summary to coverage-api
  run: |
    go run ./cmd/coveragecli npm-upload \
      -vitest-summary coverage/coverage-summary.json \
      -api-url "${{ secrets.COVERAGE_API_URL }}" \
      -api-key "${{ secrets.COVERAGE_API_KEY }}" \
      -project-key "${{ github.repository }}" \
      -project-name "${{ github.event.repository.name }}" \
      -project-group "frontend" \
      -default-branch "main" \
      -branch "${{ github.ref_name }}" \
      -commit-sha "${{ github.sha }}" \
      -author "${{ github.actor }}" \
      -trigger-type "push"
```

## 11. Security and Data Hygiene

1. Do not transmit raw absolute host paths when avoidable; normalize to repo-relative keys.
2. Never log API keys.
3. Cap maximum accepted file entries to prevent accidental oversized payloads.
4. Reject malformed numeric fields rather than silently coercing.

## 12. Testing Strategy

1. Unit tests for JSON parsing and schema validation.
2. Unit tests for path normalization across macOS/Linux/Windows separators.
3. Unit tests for grouping/weighting math.
4. Unit tests for include/exclude glob behavior.
5. CLI integration tests with sample `coverage-summary.json` fixtures.
6. End-to-end test against local API to verify response compatibility.

## 13. Acceptance Criteria

1. A Vitest `coverage/coverage-summary.json` file can be uploaded through CLI to `POST /v1/coverage-runs`.
2. Uploaded npm runs appear in existing history and comparison endpoints without backend schema changes.
3. Baseline and delta calculations remain deterministic and unchanged.
4. CLI rejects malformed or empty summary artifacts with clear errors.
5. npm coverage ingestion can be run in CI using the documented command sequence.

## 14. Rollout Plan

1. Implement CLI `npm-upload` behind normal semantic version release.
2. Publish docs and sample workflow.
3. Pilot with one frontend repository.
4. Evaluate whether Phase 2 metadata fields are needed after pilot feedback.

## 15. Open Decisions

1. Should default metric remain `lines`, or be configurable per project in API config?
2. Should package grouping default be `dir` or `file` for large monorepos?
3. Should non-source directories (for example `e2e`, `scripts`) be excluded by default?
4. Do we need explicit source-language tagging in API responses for frontend filtering?