# coverage-api

[![CI](https://github.com/arxdsilva/opencoverage/actions/workflows/ci.yml/badge.svg)](https://github.com/arxdsilva/opencoverage/actions/workflows/ci.yml)

## Dashboard Preview

![OpenCoverage dashboard preview 1](imgs/opencov-1.png)
![OpenCoverage dashboard preview 2](imgs/opencov-2.png)

Self-hosted Go code coverage API and dashboard for ingesting test coverage, comparing deltas, and tracking trends across projects, branches, and teams.

`coverage-api` is part of the `opencoverage` project and is designed for developer teams that want coverage visibility in their own infrastructure.

## Why coverage-api

- Ingest Go coverage results from local runs or CI pipelines
- Compute deterministic baseline comparisons and deltas
- Track project and package-level coverage history
- Group projects (for team-level reporting)
- Integrate with GitHub Actions and other CI systems
- Self-host API + frontend with PostgreSQL

## Key Features

- REST API with `/v1` endpoints for ingest, history, and latest comparison
- Coverage CLI to convert `coverage.out` into API-ready JSON payloads
- Dashboard frontend for project, multi-branch trend, comparison, and heatmap visualization
- Integration test result ingestion and heatmap visualization
- API key authentication for protected endpoints
- Hexagonal Architecture (ports and adapters)

## Quick Start (Docker Compose)

Run the full local stack (PostgreSQL + migrations + API + frontend):

```bash
make compose-up
```

If local port `5432` is already in use:

```bash
DB_PORT=5433 make compose-up
```

Access services:

- API: `http://localhost:8080`
- Frontend dashboard: `http://localhost:8090`
- Health check: `http://localhost:8080/healthz`

Stop the stack:

```bash
make compose-down
```

## Local Development

Requirements:

- Go 1.23+
- PostgreSQL 14+

Core environment variables:

- `DATABASE_URL` (required)
- `MIGRATIONS_DIR` (default `./migrations`)
- `API_KEY_SECRET` (required)
- `SERVER_ADDR` (default `:8080`)
- `API_KEY_HEADER` (default `X-API-Key`)
- `SHUTDOWN_TIMEOUT_SECONDS` (default `10`)

Run API locally:

```bash
export DATABASE_URL="postgres://coverage:coverage@localhost:5432/coverage?sslmode=disable"
export API_KEY_SECRET="dev-local-key"
go run ./cmd/api
```

Run frontend locally:

```bash
make frontend-run
```

Useful developer commands:

```bash
make deps
make fmt
make test
make migrate-status
make migrate-up
```

## API Overview

Main endpoints:

- `GET /v1/projects`
- `POST /v1/coverage-runs`
- `GET /v1/projects/{projectId}`
- `GET /v1/projects/{projectId}/coverage-runs`
- `GET /v1/projects/{projectId}/coverage-runs/latest-comparison`
- `GET /v1/projects/{projectId}/branches`
- `GET /v1/projects/{projectId}/contributors`
- `POST /v1/integration-test-runs`
- `GET /v1/integration-test-runs/heatmap`
- `GET /v1/projects/{projectId}/integration-test-runs`
- `GET /v1/projects/{projectId}/integration-test-runs/latest-comparison`
- `GET /v1/projects/{projectId}/integration-test-runs/{runId}`

For full API contract details, see [SPEC.md](SPEC.md).

## Ingest Coverage With cURL

Set variables:

```bash
export BASE_URL="http://localhost:8080"
export API_KEY="dev-local-key"
```

Send a coverage run:

```bash
curl -i -X POST "$BASE_URL/v1/coverage-runs" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
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
      {"importPath": "github.com/acme/repo-service/internal/api", "coveragePercent": 85.10},
      {"importPath": "github.com/acme/repo-service/internal/service", "coveragePercent": 80.70}
    ]
  }'
```

## Coverage CLI Workflow

Generate payload from Go coverage profile:

```bash
go run ./cmd/coveragecli \
  -coverprofile coverage.out \
  -out coverage-upload.json \
  -project-key "github.com/example/repo" \
  -project-name "repo" \
  -project-group "platform" \
  -default-branch "main" \
  -branch "main" \
  -commit-sha "abc123" \
  -author "alice"
```

Generate and upload in one command:

```bash
make coverage-upload API_URL="http://localhost:8080/v1/coverage-runs" API_KEY="dev-local-key"
```

Install CLI from GitHub:

```bash
go install github.com/arxdsilva/opencoverage/cmd/coveragecli@latest
```

### Upload Vitest Coverage Summary (npm)

Generate Vitest coverage summary:

```bash
npx vitest run --coverage --coverage.reporter=lcov --coverage.reporter=json-summary
```

Upload `coverage/coverage-summary.json` using the CLI:

```bash
go run ./cmd/coveragecli npm-upload \
  -vitest-summary coverage/coverage-summary.json \
  -api-url http://localhost:8080/v1/coverage-runs \
  -api-key dev-local-key \
  -project-key github.com/example/frontend-repo \
  -project-name frontend-repo \
  -project-group frontend \
  -default-branch main \
  -branch main \
  -commit-sha abc123 \
  -author alice \
  -trigger-type push
```

Useful options:

- `-metric lines|statements|functions|branches` (default `lines`)
- `-group-by dir|file` (default `dir`)
- `-include-glob` and `-exclude-glob` (repeatable)
- `-path-strip-prefix` for deterministic path normalization
- `-dry-run` and `-out <payload.json>` to inspect payload without upload

## Architecture

This project follows Hexagonal Architecture (ports and adapters):

- `cmd/api` - bootstrap and dependency wiring
- `internal/domain` - entities, invariants, deterministic domain logic
- `internal/application` - use cases and ports
- `internal/adapters/http` - handlers, DTOs, middleware
- `internal/adapters/postgres` - repository implementations
- `internal/adapters/auth` - API key authentication adapter
- `internal/platform` - config and infrastructure utilities

## Database and Migrations

Migration files are in `migrations/`.

Common commands:

```bash
make migrate-status
make migrate-up
make migrate-down
make migrate-create name=add_new_table
```

Seed local database:

```bash
make seed
```

## CI/CD and GitHub Actions

For complete CI examples (unit tests, `coverage.out`, CLI payload generation, upload to self-hosted API), see [GITHUB_ACTIONS_INTEGRATION.md](GITHUB_ACTIONS_INTEGRATION.md).

The examples include:

- Configurable project metadata (`project key`, `name`, `group`)
- Push and pull request workflows
- Multi-project matrix workflows for monorepos
- PR comments and threshold-based quality gates

## Frontend and Product Docs

- Frontend behavior and UI notes: [frontend.md](frontend.md)
- API contract and response model: [SPEC.md](SPEC.md)
- Contribution/PR workflow: [making-a-PR.md](making-a-PR.md)

Frontend highlights:

- Project overview includes a multi-branch coverage trend chart.
- The trend view overlays the default branch with all discovered branches for the selected project.
- The branch selector is used for latest-comparison details, not for filtering the trend chart.
- Heatmap overlay shows all projects grouped by team, with tiles color-coded on a -3% to +3% delta scale (green = improved, red = regressed). A scale legend is displayed in the overlay header.
- Top Contributors overlay shows the leading commit contributors per project across all teams, grouped the same way as the heatmap.
- Integration Tests screen provides per-project integration test run history and failed spec details.
- Integration Run Chain is rendered oldest to newest (newest on the right) and shows up to 5 runs.
- Integration Pass Rate card shows run success ratio percentage computed as `passed runs / failed runs * 100` over the returned run-list window (up to last 20 runs).
- Integration Heatmap shows all projects grouped by team but displays default-branch runs only, with `✅`/`❌` run markers and newest-run status tint per project row.
- Integration Tests sidebar now supports group-first project navigation: filter by project group, then select a project from the filtered list (combined with project search).

## Typical Integration Flow

1. Run tests and produce `coverage.out`.
2. Convert coverage profile to JSON payload with `coveragecli`.
3. Upload payload to `POST /v1/coverage-runs`.
4. Read comparison metadata (`thresholdStatus`, `deltaPercent`) in CI.
5. Visualize trends and project groups in the dashboard.
