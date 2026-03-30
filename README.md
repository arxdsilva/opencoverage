# coverage-api

[![CI](https://github.com/arxdsilva/coverage-api/actions/workflows/ci.yml/badge.svg)](https://github.com/arxdsilva/coverage-api/actions/workflows/ci.yml)

Go REST API for ingesting coverage runs and computing coverage deltas.

## Architecture

This project follows Hexagonal Architecture (ports and adapters):

- `cmd/api` - application bootstrap
- `internal/domain` - entities and deterministic domain logic
- `internal/application` - use cases and ports
- `internal/adapters/http` - HTTP transport and middleware
- `internal/adapters/postgres` - repository implementations
- `internal/adapters/auth` - API key authentication adapter
- `internal/platform` - config and infrastructure utilities

## Requirements

- Go 1.23+
- PostgreSQL 14+

## Configuration

Environment variables:

- `DATABASE_URL` (required)
- `SERVER_ADDR` (default `:8080`)
- `API_KEY_HEADER` (default `X-API-Key`)
- `API_KEY_SECRET` (required; value expected in the API key header)
- `SHUTDOWN_TIMEOUT_SECONDS` (default `10`)

## Run

```bash
export DATABASE_URL="postgres://coverage:coverage@localhost:5432/coverage?sslmode=disable"
export API_KEY_SECRET="dev-local-key"
go run ./cmd/api
```

Start full local stack with Docker Compose (db + migrate + api):

```bash
make compose-up
```

If port `5432` is already in use on your machine, override it:

```bash
DB_PORT=5433 make compose-up
```

## Migrations

Initial schema is in `migrations/001_init.sql`.

Common migration commands:

```bash
make migrate-status
make migrate-up
make migrate-down
make migrate-create name=add_new_table
```

## API

Main endpoints:

- `GET /v1/projects`
- `POST /v1/coverage-runs`
- `GET /v1/projects/{projectId}`
- `GET /v1/projects/{projectId}/coverage-runs`
- `GET /v1/projects/{projectId}/coverage-runs/latest-comparison`

For full contract details, see `SPEC.md`.

## Usage (curl)

Set variables first:

```bash
export BASE_URL="http://localhost:8080"
export API_KEY="dev-local-key"
export PROJECT_ID="replace-with-project-id"
```

Health check (no auth):

```bash
curl -i "$BASE_URL/healthz"
```

Ingest a coverage run:

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

Get project metadata:

```bash
curl -i "$BASE_URL/v1/projects/$PROJECT_ID" \
	-H "X-API-Key: $API_KEY"
```

List coverage runs (paginated):

```bash
curl -i "$BASE_URL/v1/projects/$PROJECT_ID/coverage-runs?page=1&pageSize=20&branch=main" \
	-H "X-API-Key: $API_KEY"
```

Get latest comparison:

```bash
curl -i "$BASE_URL/v1/projects/$PROJECT_ID/coverage-runs/latest-comparison" \
	-H "X-API-Key: $API_KEY"
```

## Coverage CLI Workflow

Use the CLI tool to generate ingest payloads from Go coverage profiles:

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

### Ingest Response Example (`POST /v1/coverage-runs`)

When a record is created, the API returns project/run metadata plus computed comparison and package deltas.

Example response:

```json
{
	"project": {
		"id": "1b22413a-f9d2-4675-8ab9-f0ee309ef871",
		"projectKey": "github.com/arxdsilva/coverage-api",
		"name": "coverage-api",
		"defaultBranch": "main",
		"globalThresholdPercent": 80,
		"created": true
	},
	"run": {
		"id": "0f171469-1cf6-4172-9334-b29381dd23de",
		"branch": "main",
		"commitSha": "local",
		"author": "local",
		"triggerType": "manual",
		"runTimestamp": "2026-03-28T22:18:18Z",
		"totalCoveragePercent": 1.9
	},
	"comparison": {
		"baselineSource": "latest_default_branch",
		"previousTotalCoveragePercent": null,
		"currentTotalCoveragePercent": 1.9,
		"deltaPercent": null,
		"direction": "new",
		"thresholdPercent": 80,
		"thresholdStatus": "failed"
	},
	"packages": [
		{
			"importPath": "github.com/arxdsilva/coverage-api/internal/domain",
			"previousCoveragePercent": null,
			"currentCoveragePercent": 50,
			"deltaPercent": null,
			"direction": "new"
		}
	]
}
```

Fields typically used in CI policy:

- `comparison.thresholdStatus` (`passed` or `failed`)
- `comparison.deltaPercent` (negative means coverage dropped)
- `comparison.currentTotalCoveragePercent`
- `comparison.previousTotalCoveragePercent`

## GitHub Actions

Workflow file: `.github/workflows/ci.yml`

It runs tests on pushes and pull requests, generates a coverage profile artifact, and optionally uploads coverage to this API on push events.

To enable coverage upload from CI, add these repository secrets:

- `COVERAGE_API_URL` (example: `https://your-api.example.com/v1/coverage-runs`)
- `COVERAGE_API_KEY`

Install the CLI from GitHub:

```bash
go install github.com/arxdsilva/coverage-api/cmd/coveragecli@latest
```

CI-friendly (pin to a specific ref/tag/commit):

```bash
go install github.com/arxdsilva/coverage-api/cmd/coveragecli@v1.0.0
# or
go install github.com/arxdsilva/coverage-api/cmd/coveragecli@<git-sha>
```

GitHub Actions step example:

```yaml
- name: Install coverage CLI
	run: go install github.com/arxdsilva/coverage-api/cmd/coveragecli@latest

- name: Upload coverage to API
	env:
		API_KEY: ${{ secrets.COVERAGE_API_KEY }}
	run: |
		go test ./... -coverprofile=coverage.out
		coveragecli \
			-coverprofile coverage.out \
			-out coverage-upload.json \
			-api-url https://your-api.example.com/v1/coverage-runs \
			-api-key "$API_KEY" \
			-project-key github.com/your-org/your-repo \
			-project-name your-repo \
			-branch "$GITHUB_REF_NAME" \
			-commit-sha "$GITHUB_SHA" \
			-author "github-actions" \
			-trigger-type push \
			-upload
```

### PR Coverage Interaction (Warning or Fail)

For pull requests, you can upload coverage and then decide policy from API response:

1. **Warning mode**: annotate PR with a warning, but do not fail job.
2. **Fail mode**: fail job when `comparison.thresholdStatus == "failed"`.

Example PR step (requires `jq` on runner):

```yaml
- name: Upload PR coverage and capture response
	id: pr_coverage
	if: ${{ github.event_name == 'pull_request' && secrets.COVERAGE_API_URL != '' && secrets.COVERAGE_API_KEY != '' }}
	env:
		API_URL: ${{ secrets.COVERAGE_API_URL }}
		API_KEY: ${{ secrets.COVERAGE_API_KEY }}
	run: |
		go test ./... -coverprofile=coverage.out
		go run ./cmd/coveragecli \
			-coverprofile coverage.out \
			-out coverage-upload.json \
			-project-key "${{ github.repository }}" \
			-project-name "${{ github.event.repository.name }}" \
			-branch "${{ github.head_ref }}" \
			-commit-sha "${{ github.sha }}" \
			-author "github-actions" \
			-trigger-type pr

		RESPONSE=$(curl -sS -X POST "$API_URL" \
			-H "Content-Type: application/json" \
			-H "X-API-Key: $API_KEY" \
			--data-binary @coverage-upload.json)

		echo "$RESPONSE" > coverage-api-response.json

		STATUS=$(jq -r '.comparison.thresholdStatus' coverage-api-response.json)
		CURRENT=$(jq -r '.comparison.currentTotalCoveragePercent' coverage-api-response.json)
		PREV=$(jq -r '.comparison.previousTotalCoveragePercent' coverage-api-response.json)
		DELTA=$(jq -r '.comparison.deltaPercent' coverage-api-response.json)

		echo "thresholdStatus=$STATUS" >> "$GITHUB_OUTPUT"
		echo "currentCoverage=$CURRENT" >> "$GITHUB_OUTPUT"
		echo "previousCoverage=$PREV" >> "$GITHUB_OUTPUT"
		echo "deltaCoverage=$DELTA" >> "$GITHUB_OUTPUT"
```

Warning-only policy:

```yaml
- name: Warn when threshold fails
	if: ${{ steps.pr_coverage.outputs.thresholdStatus == 'failed' }}
	run: |
		echo "::warning title=Coverage Threshold Failed::Current=${{ steps.pr_coverage.outputs.currentCoverage }} Previous=${{ steps.pr_coverage.outputs.previousCoverage }} Delta=${{ steps.pr_coverage.outputs.deltaCoverage }}"
```

Failing policy:

```yaml
- name: Fail when threshold fails
	if: ${{ steps.pr_coverage.outputs.thresholdStatus == 'failed' }}
	run: |
		echo "Coverage threshold failed"
		exit 1
```

Generate Go coverage profile and API payload file:

```bash
make coverage-file
```

This creates:

- `coverage.out` (Go cover profile)
- `coverage-upload.json` (API-ready JSON payload)

Generate and upload in one step:

```bash
make coverage-upload API_URL="http://localhost:8080/v1/coverage-runs" API_KEY="dev-local-key"
```

Sample command to send an existing payload file from Go CLI to the API:

```bash
go run ./cmd/coveragecli \
	-coverprofile coverage.out \
	-out coverage-upload.json \
	-api-url http://localhost:8080/v1/coverage-runs \
	-api-key dev-local-key \
	-upload
```
