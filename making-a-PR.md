# Making a PR with Coverage API

This guide explains how a Pull Request (PR) should send coverage to this API and how CI can decide whether to warn or fail.

## What Happens on PR Upload

When CI sends a coverage payload to:

- `POST /v1/coverage-runs`

the API will:

1. Record a new coverage run.
2. Compute comparison values against baseline.
3. Return a response with threshold and delta information.

Important: runs are persisted even when threshold fails.

## Required Request Fields for PR Context

Use these fields in your payload:

- `projectKey`: stable repository identifier (for example: `owner/repo`)
- `branch`: PR branch name
- `commitSha`: PR commit SHA
- `triggerType`: `pr`
- `runTimestamp`: current UTC timestamp
- `totalCoveragePercent`: overall coverage
- `packages`: package-level coverage array

Authentication header:

- `X-API-Key: <your-api-secret>`

## Recommended CI Flow

1. Run tests with coverage profile:

```bash
go test ./... -coverprofile=coverage.out
```

2. Build API payload (using project CLI):

```bash
go run ./cmd/coveragecli \
  -coverprofile coverage.out \
  -out coverage-upload.json \
  -project-key "${GITHUB_REPOSITORY}" \
  -project-name "${GITHUB_REPOSITORY##*/}" \
  -project-group "team-name" \
  -branch "${GITHUB_HEAD_REF}" \
  -commit-sha "${GITHUB_SHA}" \
  -author "github-actions" \
  -trigger-type pr
```

3. Upload payload to API:

```bash
curl -sS -X POST "$COVERAGE_API_URL" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $COVERAGE_API_KEY" \
  --data-binary @coverage-upload.json > coverage-api-response.json
```

## Response Fields to Use in PR Policy

Use these response fields:

- `comparison.thresholdStatus`
- `comparison.deltaPercent`
- `comparison.currentTotalCoveragePercent`
- `comparison.previousTotalCoveragePercent`

Example checks:

- Warning-only mode:
  - emit warning when `thresholdStatus == "failed"`
- Fail mode:
  - fail the job when `thresholdStatus == "failed"`
- Optional stricter mode:
  - fail when `deltaPercent < 0`

## GitHub Actions Example (PR)

```yaml
- name: Upload PR coverage and capture response
  id: pr_coverage
  if: ${{ github.event_name == 'pull_request' && secrets.COVERAGE_API_URL != '' && secrets.COVERAGE_API_KEY != '' }}
  env:
    COVERAGE_API_URL: ${{ secrets.COVERAGE_API_URL }}
    COVERAGE_API_KEY: ${{ secrets.COVERAGE_API_KEY }}
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

    RESPONSE=$(curl -sS -X POST "$COVERAGE_API_URL" \
      -H "Content-Type: application/json" \
      -H "X-API-Key: $COVERAGE_API_KEY" \
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

Warning step:

```yaml
- name: Warn when threshold fails
  if: ${{ steps.pr_coverage.outputs.thresholdStatus == 'failed' }}
  run: |
    echo "::warning title=Coverage Threshold Failed::Current=${{ steps.pr_coverage.outputs.currentCoverage }} Previous=${{ steps.pr_coverage.outputs.previousCoverage }} Delta=${{ steps.pr_coverage.outputs.deltaCoverage }}"
```

Fail step:

```yaml
- name: Fail when threshold fails
  if: ${{ steps.pr_coverage.outputs.thresholdStatus == 'failed' }}
  run: |
    echo "Coverage threshold failed"
    exit 1
```

## Quick Answers

- Will PR uploads be stored? Yes.
- Will failed threshold uploads be stored? Yes.
- Can CI warn instead of fail? Yes.
- Can CI fail the PR check? Yes.
