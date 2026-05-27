# OpenCoverage MCP Server Specification

## 1. Overview

### 1.1 Purpose
Define a Model Context Protocol (MCP) server for OpenCoverage so AI agents, IDE assistants, and other MCP clients can query coverage and integration-test data using a structured tool/resource interface instead of calling the REST API directly.

### 1.2 Primary Outcome
Provide a deterministic MCP surface that lets clients answer:
1. What projects exist and what is their current health?
2. Did coverage or integration health regress on a branch?
3. Which packages or specs changed relative to baseline?
4. Who contributed to recent default-branch changes?
5. Which projects or groups need attention across the fleet?

### 1.3 Design Principle
The MCP server is an additional adapter for the existing platform, not a parallel product. It should reuse the current application use cases and domain rules so that REST, frontend, and MCP all expose the same source of truth.

## 2. Scope

### 2.1 In Scope (v1)
1. MCP server that exposes project, coverage, contributor, and integration-test data.
2. Read-oriented tools for querying current and historical state.
3. MCP resources for stable, cacheable views such as project summaries and latest comparisons.
4. Prompt templates for common workflows such as regression investigation.
5. Authentication from MCP server to OpenCoverage using the existing API key secret or direct in-process wiring.
6. Stdio transport for local IDE/CLI clients.

### 2.2 Optional In Scope (v1.1)
1. Write tools for ingesting coverage runs.
2. Write tools for ingesting integration-test runs.
3. Streamable HTTP transport for remote MCP clients.

### 2.3 Out of Scope (v1)
1. User-specific auth and multi-tenant RBAC.
2. Arbitrary SQL querying.
3. Editing project configuration from MCP.
4. File-level or function-level coverage analysis.
5. Long-running background jobs or subscriptions.

## 3. Users and Use Cases

### 3.1 Primary Users
1. IDE agents that need project health context during code review or implementation.
2. CLI agents that investigate failing CI runs.
3. Internal automation that summarizes regressions across projects.

### 3.2 Representative Use Cases
1. "Show me the latest coverage comparison for project X on branch Y."
2. "Which packages regressed in the last run?"
3. "Why did integration health drop on the default branch?"
4. "List the projects in the backend group with failing integration runs."
5. "Summarize the top contributors for this project on main."

## 4. High-Level Architecture

### 4.1 Position in the System
The MCP server is a new adapter that sits alongside HTTP and PostgreSQL adapters:
1. `cmd/mcp` - MCP entrypoint and bootstrap wiring.
2. `internal/adapters/mcp` - MCP server, tool handlers, resource handlers, prompt definitions.
3. `internal/application` - existing use cases reused directly.
4. `internal/domain` - existing entities and deterministic comparison logic.
5. `internal/platform` - config, logging, auth secret loading, clock, IDs as needed.

### 4.2 Dependency Direction
1. MCP handlers depend on application use cases.
2. Application use cases depend on ports only.
3. The MCP adapter must not import HTTP handler DTOs or reuse HTTP-specific middleware.

### 4.3 Preferred Execution Model
1. Local mode: MCP server runs in-process and calls application use cases directly.
2. Compatibility mode: if needed later, the MCP adapter may call the REST API as a fallback, but this is not the preferred architecture because it duplicates boundary validation and error mapping.

## 5. Protocol and Transport

### 5.1 MCP Version
Target the current stable MCP specification supported by major IDE clients at implementation time.

### 5.2 Initial Transport
1. `stdio` transport is the required first transport.
2. Transport lifecycle must support standard MCP initialize, tool discovery, resource discovery, prompt discovery, and request handling.

### 5.3 Future Transport
1. Streamable HTTP may be added later for hosted MCP clients.
2. If added, auth requirements must be explicit between client and MCP server.

## 6. Configuration

### 6.1 Required Configuration
1. `MCP_SERVER_NAME` (default `opencoverage`)
2. `MCP_SERVER_VERSION` (derived from build or release metadata)
3. `MCP_TRANSPORT` (default `stdio`)
4. `DATABASE_URL` when running in-process against PostgreSQL-backed application wiring
5. `API_KEY_SECRET` when sharing the existing auth model or enabling ingest tools

### 6.2 Optional Configuration
1. `MCP_ENABLE_WRITE_TOOLS` (default `false`)
2. `MCP_MAX_PAGE_SIZE` (default `100`)
3. `MCP_DEFAULT_RUNS_LIMIT` (default `20`)
4. `MCP_LOG_LEVEL`
5. `MCP_ENABLE_PROMPTS` (default `true`)

## 7. Data Exposure Model

### 7.1 Core Domain Coverage
The MCP server exposes these existing OpenCoverage concepts:
1. Project
2. CoverageRun
3. PackageCoverage delta/comparison
4. IntegrationTestRun
5. IntegrationSpecResult
6. Contributor summary

### 7.2 Baseline Semantics
The MCP server must preserve existing application behavior:
1. Coverage baseline = latest run on the project default branch.
2. Integration baseline = latest integration run on the project default branch.
3. Missing baseline returns a valid result with direction `new`.

## 8. Tool Surface (v1)

All tool outputs should be structured JSON objects. Tool names use snake_case to stay readable across MCP clients.

### 8.1 `list_projects`
Purpose: return paginated project catalog.

Input schema:
```json
{
  "page": 1,
  "pageSize": 20
}
```

Behavior:
1. Maps to the existing list-projects use case.
2. Returns projects plus pagination metadata.

### 8.2 `get_project`
Purpose: fetch project metadata by project ID.

Input schema:
```json
{
  "projectId": "uuid"
}
```

### 8.3 `list_branches`
Purpose: list known branches for a project.

Input schema:
```json
{
  "projectId": "uuid"
}
```

### 8.4 `list_coverage_runs`
Purpose: fetch paginated coverage history with optional branch/time filtering.

Input schema:
```json
{
  "projectId": "uuid",
  "branch": "main",
  "from": "2026-01-01T00:00:00Z",
  "to": "2026-01-31T23:59:59Z",
  "page": 1,
  "pageSize": 20
}
```

### 8.5 `get_latest_coverage_comparison`
Purpose: return the latest coverage run and comparison snapshot for a project, optionally scoped to a branch.

Input schema:
```json
{
  "projectId": "uuid",
  "branch": "feature/my-branch"
}
```

Output requirements:
1. Include project summary, run summary, comparison summary, and package deltas.
2. Preserve `direction`, threshold status, and `baselineSource` fields.

### 8.6 `list_contributors`
Purpose: return top contributors for a project's default branch.

Input schema:
```json
{
  "projectId": "uuid",
  "limit": 10
}
```

Behavior:
1. Maps to the existing contributor use case, which currently resolves the project's default branch internally.
2. Branch override is a future enhancement, not part of v1.

### 8.7 `list_integration_runs`
Purpose: fetch paginated integration history.

Input schema:
```json
{
  "projectId": "uuid",
  "branch": "main",
  "status": "failed",
  "environment": "prod",
  "from": "2026-01-01T00:00:00Z",
  "to": "2026-01-31T23:59:59Z",
  "page": 1,
  "pageSize": 20
}
```

### 8.8 `get_latest_integration_comparison`
Purpose: return the latest integration run and comparison snapshot for a project.

Input schema:
```json
{
  "projectId": "uuid"
}
```

Output requirements:
1. Include pass-rate delta, new failures, resolved failures, and failed spec summaries.
2. Preserve existing baseline semantics.

### 8.9 `get_integration_run`
Purpose: return full run details for a specific integration run.

Input schema:
```json
{
  "projectId": "uuid",
  "runId": "uuid"
}
```

### 8.10 `get_integration_heatmap`
Purpose: return all-project grouped integration heatmap data.

Input schema:
```json
{
  "branch": "main",
  "status": "failed",
  "runsPerProject": 5
}
```

### 8.11 Optional Write Tool: `ingest_coverage_run`
Purpose: accept the same normalized coverage payload already defined by the REST API.

Default state:
1. Disabled in v1 unless `MCP_ENABLE_WRITE_TOOLS=true`.
2. If enabled, input schema must match the current ingest contract to avoid dual formats.

### 8.12 Optional Write Tool: `ingest_integration_run`
Purpose: accept the normalized Ginkgo payload already defined by the REST API.

Default state:
1. Disabled in v1 unless `MCP_ENABLE_WRITE_TOOLS=true`.
2. Input schema must match the current integration ingest contract.

## 9. Resource Surface (v1)

Resources are intended for stable, discoverable views that agents may reference repeatedly.

### 9.1 `opencoverage://projects`
Returns the first page of project catalog or a configurable default view.

### 9.2 `opencoverage://projects/{projectId}`
Returns project metadata.

### 9.3 `opencoverage://projects/{projectId}/coverage/latest`
Returns latest coverage comparison snapshot.

### 9.4 `opencoverage://projects/{projectId}/integration/latest`
Returns latest integration comparison snapshot.

### 9.5 `opencoverage://projects/{projectId}/contributors`
Returns contributor summary for the default branch unless branch override is part of resource params supported by the chosen MCP library.

### 9.6 `opencoverage://integration/heatmap`
Returns grouped integration heatmap data using default server filters.

## 10. Prompt Surface (v1)

Prompts should be optional and server-side discoverable.

### 10.1 `summarize_project_health`
Inputs:
1. `projectId`
2. `branch` (optional)

Behavior:
1. Guide the client to call latest comparison and integration tools.
2. Produce a concise health summary covering coverage, integration status, and likely attention areas.

### 10.2 `investigate_coverage_regression`
Inputs:
1. `projectId`
2. `branch` (optional)

Behavior:
1. Guide the client to inspect latest comparison, package deltas, and recent run history.

### 10.3 `investigate_integration_failures`
Inputs:
1. `projectId`
2. `environment` (optional)

Behavior:
1. Guide the client to inspect latest integration comparison, failed specs, and recent run history.

## 11. Error Handling

### 11.1 Principles
1. Preserve typed application errors where possible.
2. Return MCP tool errors with machine-readable codes and concise human messages.
3. Do not leak secrets, connection strings, or internal SQL details.

### 11.2 Standard Error Mapping
1. Invalid input -> `invalid_argument`
2. Missing project or run -> `not_found`
3. Unauthorized write operation or bad API secret -> `unauthorized`
4. Internal repository/use-case failure -> `internal`

### 11.3 Validation Rules
1. Time filters must be RFC3339.
2. `pageSize` must not exceed `MCP_MAX_PAGE_SIZE`.
3. Optional enum-like filters such as `status` and `environment` should be normalized or rejected consistently with existing application behavior.

## 12. Security

### 12.1 Trust Model
1. In local stdio mode, the MCP client is trusted as much as the local developer session.
2. The server still must protect secrets and redact them from logs.

### 12.2 Secret Handling
1. Never expose `API_KEY_SECRET` in any tool response, resource, prompt, or log.
2. If write tools are enabled, require explicit configuration rather than enabling by default.

### 12.3 Authorization Strategy
For v1, authorization is coarse-grained:
1. Read tools are enabled for any configured MCP client.
2. Write tools are disabled by default.
3. If streamable HTTP is added later, client auth must be added before exposing write tools remotely.

## 13. Observability

### 13.1 Logging
1. Structured logs with request or invocation IDs.
2. Log tool name, success or failure, duration, and key identifiers such as project ID.
3. Never log raw ingest payloads in full when they may contain noisy or sensitive metadata.

### 13.2 Metrics
Recommended metrics:
1. Tool invocation count by tool name and status.
2. Tool latency by tool name.
3. Resource read count.
4. Prompt invocation count.
5. DB query error count.

## 14. Implementation Notes

### 14.1 Package Layout
Suggested structure:
1. `cmd/mcp/main.go`
2. `internal/adapters/mcp/server.go`
3. `internal/adapters/mcp/tools.go`
4. `internal/adapters/mcp/resources.go`
5. `internal/adapters/mcp/prompts.go`
6. `internal/adapters/mcp/dto.go`

### 14.2 Reuse Strategy
1. Reuse application-layer input/output types where they already represent domain-safe contracts.
2. Introduce MCP-specific DTOs only when MCP schemas need a different shape from HTTP responses.
3. Do not move business logic into the MCP adapter.

### 14.3 Library Choice
Implementation should use a stable Go MCP SDK that supports:
1. stdio transport
2. tool discovery and invocation
3. resources
4. prompts
5. structured JSON schemas

The exact library choice is intentionally left open in this spec.

## 15. Testing Strategy

### 15.1 Unit Tests
1. Tool handler input validation.
2. Error mapping from application errors to MCP errors.
3. Prompt assembly logic.

### 15.2 Integration Tests
1. End-to-end stdio handshake and tool invocation.
2. Querying latest coverage comparison from seeded data.
3. Querying latest integration comparison and failed specs from seeded data.
4. Verifying disabled write tools are not advertised when feature flag is off.

### 15.3 Contract Tests
1. Ensure MCP tool outputs stay aligned with current application semantics.
2. Add golden responses for key tools such as latest comparison and integration heatmap.

## 16. Rollout Plan

### 16.1 Phase 1
1. Implement read-only stdio MCP server.
2. Ship project, coverage, contributor, and integration tools.
3. Ship latest-comparison resources and prompts.

### 16.2 Phase 2
1. Add ingest tools behind a feature flag.
2. Add remote transport if there is a real client need.
3. Add richer fleet-wide summary prompts.

## 17. Definition of Done

The MCP server is complete for v1 when:
1. It runs via `go run ./cmd/mcp`.
2. It exposes the read-only tools defined in this spec.
3. It reuses current application use cases rather than duplicating comparison logic.
4. It includes tests for tool invocation, validation, and error mapping.
5. It documents local client configuration examples in the README or docs.
