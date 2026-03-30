# coverage-api Specification

## 1. Overview

### 1.1 Purpose
coverage-api is a Go REST API that ingests Go test coverage results and returns coverage comparison insights.

For each ingest request:
1. If the project already exists, the API returns current coverage, previous coverage baseline, delta, threshold status, and package-level deltas.
2. If the project does not exist, the API creates it, stores the first run, and returns the same comparison object (with no previous baseline values).

### 1.2 Primary Outcome
Provide a reliable coverage history system that can answer:
1. Did coverage go up or down?
2. By how much?
3. Which packages changed?
4. Is the project under threshold?

### 1.3 API Style
REST, versioned under `/v1`.

## 2. Scope

### 2.1 In Scope (v1)
1. API key authentication on every request.
2. JSON ingestion endpoint for coverage runs.
3. Auto-create project during ingest if it does not exist.
4. Coverage comparison against latest run on default branch.
5. Package-level coverage tracking keyed by Go import path.
6. Global threshold evaluation (default 80%).
7. Structured error responses.
8. Run history retrieval endpoints with pagination.
9. Persistent storage in PostgreSQL.

### 2.2 Out of Scope (v1)
1. Per-package thresholds.
2. Multiple baseline strategies (custom commit baseline).
3. File-level or function-level coverage diffing.
4. Deletion/pruning of historical runs (history is retained forever).

## 3. Key Decisions and Assumptions

1. Input is JSON payload (already parsed, not raw file upload).
2. Project ID is server-generated.
3. Branch is included in run metadata, but coverage comparison baseline is always latest run on default branch.
4. If coverage is below threshold, endpoint still returns HTTP 200 with comparison status `failed`.
5. Duplicate submissions are allowed (same project, branch, commit can be stored multiple times).

## 4. High-Level Architecture

### 4.1 Components
1. HTTP API layer (Go).
2. Auth middleware (API key validation).
3. Domain service:
   - project resolution and auto-creation
   - baseline lookup
   - coverage diff computation
   - threshold evaluation
4. PostgreSQL persistence.

### 4.2 Request Flow (Ingest)
1. Validate API key.
2. Validate JSON payload.
3. Resolve project by identifier (input key), create if missing.
4. Persist run + package coverages.
5. Fetch latest default-branch baseline (if any previous run exists).
6. Compute deltas at overall and package levels.
7. Evaluate threshold and set status.
8. Return normalized comparison response.

## 5. Data Model (Draft)

### 5.1 Entity Summary
1. Project
2. CoverageRun
3. PackageCoverage

### 5.2 Logical Fields

#### Project
1. id (UUID, server-generated)
2. project_key (string, client-provided identifier used to find project)
3. name (string, optional display name)
4. group (string, optional group name for organizing projects in UI)
5. default_branch (string, default `main`)
6. global_threshold_percent (numeric(5,2), default 80.00)
7. created_at (timestamp)
8. updated_at (timestamp)

#### CoverageRun
1. id (UUID)
2. project_id (UUID, FK -> projects.id)
3. branch (string)
4. commit_sha (string)
5. author (string, optional)
6. trigger_type (enum/string: push, pr, manual)
7. run_timestamp (timestamp from client or server)
8. total_coverage_percent (numeric(5,2))
9. created_at (timestamp)

#### PackageCoverage
1. id (UUID)
2. run_id (UUID, FK -> coverage_runs.id)
3. package_import_path (string)
4. coverage_percent (numeric(5,2))

### 5.3 Suggested PostgreSQL Schema (Starter)

```sql
CREATE TABLE projects (
  id UUID PRIMARY KEY,
  project_key TEXT NOT NULL UNIQUE,
  name TEXT,
  "group" TEXT,
  default_branch TEXT NOT NULL DEFAULT 'main',
  global_threshold_percent NUMERIC(5,2) NOT NULL DEFAULT 80.00,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE coverage_runs (
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

CREATE INDEX coverage_runs_project_branch_ts_idx
  ON coverage_runs(project_id, branch, run_timestamp DESC);

CREATE INDEX coverage_runs_project_default_lookup_idx
  ON coverage_runs(project_id, run_timestamp DESC);

CREATE TABLE package_coverages (
  id UUID PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES coverage_runs(id) ON DELETE CASCADE,
  package_import_path TEXT NOT NULL,
  coverage_percent NUMERIC(5,2) NOT NULL
);

CREATE INDEX package_coverages_run_id_idx ON package_coverages(run_id);
CREATE INDEX package_coverages_import_path_idx ON package_coverages(package_import_path);
```

## 6. API Contract (v1)

### 6.1 Authentication
1. Header: `X-API-Key: <key>`
2. Required for all endpoints.
3. API validates key against environment secret `API_KEY_SECRET`.
4. Missing/invalid key -> `401 Unauthorized`.

### 6.2 Content Type
1. Request: `application/json`
2. Response: `application/json`

### 6.3 Endpoint Summary
1. `POST /v1/coverage-runs` - ingest run, auto-create project, return comparison.
2. `GET /v1/projects/{projectId}` - fetch project metadata.
3. `GET /v1/projects/{projectId}/coverage-runs` - paginated run history.
4. `GET /v1/projects/{projectId}/coverage-runs/latest-comparison` - latest run comparison snapshot.

## 7. Detailed Endpoint Specs

### 7.1 POST /v1/coverage-runs

#### Request Body
```json
{
  "projectKey": "org/repo-service",
  "projectName": "repo-service",
  "projectGroup": "platform-team",
  "defaultBranch": "main",
  "branch": "main",
  "commitSha": "a1b2c3d4",
  "author": "alice",
  "triggerType": "push",
  "runTimestamp": "2026-03-28T12:00:00Z",
  "totalCoveragePercent": 83.42,
  "packages": [
    { "importPath": "github.com/acme/repo-service/internal/api", "coveragePercent": 85.10 },
    { "importPath": "github.com/acme/repo-service/internal/service", "coveragePercent": 80.70 }
  ]
}
```

#### Behavior
1. If `projectKey` does not exist, create project (server generates `projectId`).
2. Store run and package coverage rows.
3. Lookup previous baseline from latest run on project default branch.
4. Compute overall and package-level deltas against baseline.
5. Evaluate threshold (project global threshold, default 80).
6. Return HTTP 200 with status details.

#### Response Body (Example)
```json
{
  "project": {
    "id": "5d6e8f6d-f1c8-4f3f-8c93-caf78e7a6a34",
    "projectKey": "org/repo-service",
    "name": "repo-service",
    "defaultBranch": "main",
    "globalThresholdPercent": 80.0,
    "created": false
  },
  "run": {
    "id": "9ee1df19-494c-4d72-b4f8-5f5f2df45d2b",
    "branch": "main",
    "commitSha": "a1b2c3d4",
    "author": "alice",
    "triggerType": "push",
    "runTimestamp": "2026-03-28T12:00:00Z",
    "totalCoveragePercent": 83.42
  },
  "comparison": {
    "baselineSource": "latest_default_branch",
    "previousTotalCoveragePercent": 84.10,
    "currentTotalCoveragePercent": 83.42,
    "deltaPercent": -0.68,
    "direction": "down",
    "thresholdPercent": 80.0,
    "thresholdStatus": "passed"
  },
  "packages": [
    {
      "importPath": "github.com/acme/repo-service/internal/api",
      "previousCoveragePercent": 86.0,
      "currentCoveragePercent": 85.1,
      "deltaPercent": -0.9,
      "direction": "down"
    },
    {
      "importPath": "github.com/acme/repo-service/internal/service",
      "previousCoveragePercent": 79.9,
      "currentCoveragePercent": 80.7,
      "deltaPercent": 0.8,
      "direction": "up"
    }
  ]
}
```

#### First Run Case
If no baseline exists:
1. `previousTotalCoveragePercent = null`
2. package previous values are `null`
3. `deltaPercent = null`
4. `direction = "new"`

### 7.2 GET /v1/projects/{projectId}
Returns project metadata and threshold/default branch config.

### 7.3 GET /v1/projects/{projectId}/coverage-runs

#### Query Params
1. `page` (default 1)
2. `pageSize` (default 20, max 100)
3. `branch` (optional filter)
4. `from` (optional ISO timestamp)
5. `to` (optional ISO timestamp)

#### Response
Paginated run list with metadata:
```json
{
  "items": [
    {
      "id": "9ee1df19-494c-4d72-b4f8-5f5f2df45d2b",
      "branch": "main",
      "commitSha": "a1b2c3d4",
      "runTimestamp": "2026-03-28T12:00:00Z",
      "totalCoveragePercent": 83.42
    }
  ],
  "pagination": {
    "page": 1,
    "pageSize": 20,
    "totalItems": 124,
    "totalPages": 7
  }
}
```

### 7.4 GET /v1/projects/{projectId}/coverage-runs/latest-comparison
Returns latest run plus computed comparison against latest default-branch baseline.

## 8. Error Model

### 8.1 Structured Error Format
```json
{
  "error": {
    "code": "INVALID_ARGUMENT",
    "message": "branch is required",
    "details": {
      "field": "branch"
    }
  }
}
```

### 8.2 Standard Error Codes
1. `UNAUTHENTICATED` (401)
2. `INVALID_ARGUMENT` (400)
3. `NOT_FOUND` (404)
4. `CONFLICT` (409, reserved for future use)
5. `INTERNAL` (500)

## 9. Validation Rules

1. `projectKey` required, non-empty.
2. `branch` required, non-empty.
3. `commitSha` required, non-empty.
4. `triggerType` in `push|pr|manual`.
5. `totalCoveragePercent` range 0 to 100.
6. Each package must have unique import path within request.
7. Each package coverage range 0 to 100.
8. `runTimestamp` must be valid ISO-8601 timestamp.

## 10. Comparison Rules

1. Baseline is latest run on project default branch.
2. Overall delta formula:
   - `deltaPercent = currentTotalCoveragePercent - previousTotalCoveragePercent`
3. Package delta formula:
   - `deltaPercent = currentPackageCoveragePercent - previousPackageCoveragePercent`
4. Direction:
   - `up` if delta > 0
   - `down` if delta < 0
   - `equal` if delta = 0
   - `new` if no previous value
5. Threshold status:
   - `passed` if current total >= threshold
   - `failed` if current total < threshold

## 11. Non-Functional Requirements

1. Every endpoint requires API key auth.
2. Persist all runs forever.
3. Basic observability:
   - request ID propagation
   - structured logs
   - endpoint latency metrics
4. P95 ingest latency target: <= 300ms (excluding DB/network variability).
5. Safe DB writes with transactions for run + package inserts.

## 12. OpenAPI Starter Snippet

```yaml
openapi: 3.0.3
info:
  title: coverage-api
  version: 1.0.0
servers:
  - url: /v1
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-API-Key
  schemas:
    IngestCoverageRunRequest:
      type: object
      required:
        - projectKey
        - branch
        - commitSha
        - triggerType
        - runTimestamp
        - totalCoveragePercent
        - packages
      properties:
        projectKey:
          type: string
        projectName:
          type: string
        defaultBranch:
          type: string
          default: main
        branch:
          type: string
        commitSha:
          type: string
        author:
          type: string
        triggerType:
          type: string
          enum: [push, pr, manual]
        runTimestamp:
          type: string
          format: date-time
        totalCoveragePercent:
          type: number
          format: float
          minimum: 0
          maximum: 100
        packages:
          type: array
          items:
            type: object
            required: [importPath, coveragePercent]
            properties:
              importPath:
                type: string
              coveragePercent:
                type: number
                minimum: 0
                maximum: 100
paths:
  /coverage-runs:
    post:
      security:
        - ApiKeyAuth: []
      summary: Ingest coverage run and return comparison
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/IngestCoverageRunRequest'
      responses:
        '200':
          description: Ingested and compared
        '400':
          description: Validation error
        '401':
          description: Invalid API key
        '500':
          description: Internal error
```

## 13. Acceptance Criteria

1. API accepts JSON coverage payloads and authenticates each request via API key.
2. New projects are auto-created at ingest time with server-generated ID.
3. Each ingest persists a new run record (no deduping).
4. Response includes current, previous, and delta overall coverage values.
5. Response includes package-level current, previous, and delta values.
6. Threshold pass/fail status is returned with HTTP 200.
7. History endpoints return paginated results.
8. Data persists in PostgreSQL and remains queryable over time.

## 14. Future Enhancements (Post-v1)

1. Per-package thresholds.
2. Commit-to-commit diff baseline selection.
3. Branch-specific baseline mode.
4. Webhook callbacks for failed threshold events.
5. Retention policies and archive tiers.
