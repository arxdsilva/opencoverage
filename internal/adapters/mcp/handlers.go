package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/arxdsilva/opencoverage/internal/application"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	applicationMaxPageSize = 100
	maxContributorLimit    = 25
	maxRunsPerProject      = 30
)

// handleListProjects returns a paginated list of projects.
func (a *Adapter) handleListProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.ListProjects == nil {
		return toolErrorResult(application.NewInternal("list projects use case is not configured", nil)), nil
	}
	out, err := a.services.ListProjects.Execute(ctx, application.ListProjectsInput{
		Page:     a.normalizePage(request.GetInt("page", 1)),
		PageSize: a.normalizePageSize(request.GetInt("pageSize", defaultListPageSize)),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleGetProject returns project metadata for a single project ID.
func (a *Adapter) handleGetProject(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.GetProject == nil {
		return toolErrorResult(application.NewInternal("get project use case is not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	out, err := a.services.GetProject.Execute(ctx, projectID)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleListBranches returns known branches for a project.
func (a *Adapter) handleListBranches(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.ListBranches == nil {
		return toolErrorResult(application.NewInternal("list branches use case is not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	out, err := a.services.ListBranches.Execute(ctx, projectID)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleListCoverageRuns returns paginated coverage runs with optional filters.
func (a *Adapter) handleListCoverageRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.ListCoverageRuns == nil {
		return toolErrorResult(application.NewInternal("list coverage runs use case is not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	from, err := parseOptionalTime(request.GetString("from", ""), "from")
	if err != nil {
		return toolErrorResult(err), nil
	}
	to, err := parseOptionalTime(request.GetString("to", ""), "to")
	if err != nil {
		return toolErrorResult(err), nil
	}
	out, err := a.services.ListCoverageRuns.Execute(ctx, application.ListCoverageRunsInput{
		ProjectID: projectID,
		Branch:    request.GetString("branch", ""),
		From:      from,
		To:        to,
		Page:      a.normalizePage(request.GetInt("page", 1)),
		PageSize:  a.normalizePageSize(request.GetInt("pageSize", defaultListPageSize)),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleGetLatestCoverageComparison returns the latest coverage comparison envelope.
func (a *Adapter) handleGetLatestCoverageComparison(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.GetLatestComparison == nil || a.services.GetProject == nil {
		return toolErrorResult(application.NewInternal("latest coverage comparison dependencies are not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	project, err := a.services.GetProject.Execute(ctx, projectID)
	if err != nil {
		return toolErrorResult(err), nil
	}
	out, err := a.services.GetLatestComparison.Execute(ctx, application.GetLatestComparisonInput{
		ProjectID: projectID,
		Branch:    request.GetString("branch", ""),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(coverageComparisonEnvelope{
		Project:    project,
		Run:        out.Run,
		Comparison: out.Comparison,
		Packages:   out.Packages,
	})
}

// handleListContributors returns top contributors for a project.
func (a *Adapter) handleListContributors(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.ListContributors == nil {
		return toolErrorResult(application.NewInternal("list contributors use case is not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	out, err := a.services.ListContributors.Execute(ctx, application.ListContributorsInput{
		ProjectID: projectID,
		Limit:     a.normalizeContributorLimit(request.GetInt("limit", defaultContributorLimit)),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleListIntegrationRuns returns paginated integration runs with optional filters.
func (a *Adapter) handleListIntegrationRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.ListIntegrationRuns == nil {
		return toolErrorResult(application.NewInternal("list integration runs use case is not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	from, err := parseOptionalTime(request.GetString("from", ""), "from")
	if err != nil {
		return toolErrorResult(err), nil
	}
	to, err := parseOptionalTime(request.GetString("to", ""), "to")
	if err != nil {
		return toolErrorResult(err), nil
	}
	out, err := a.services.ListIntegrationRuns.Execute(ctx, application.ListIntegrationRunsInput{
		ProjectID:   projectID,
		Branch:      request.GetString("branch", ""),
		Status:      request.GetString("status", ""),
		Environment: request.GetString("environment", ""),
		From:        from,
		To:          to,
		Page:        a.normalizePage(request.GetInt("page", 1)),
		PageSize:    a.normalizePageSize(request.GetInt("pageSize", defaultListPageSize)),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleGetLatestIntegrationComparison returns the latest integration comparison.
func (a *Adapter) handleGetLatestIntegrationComparison(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.GetLatestIntegrationCompare == nil {
		return toolErrorResult(application.NewInternal("latest integration comparison use case is not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	out, err := a.services.GetLatestIntegrationCompare.Execute(ctx, projectID)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleGetIntegrationRun returns a specific integration run.
func (a *Adapter) handleGetIntegrationRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.GetIntegrationRun == nil {
		return toolErrorResult(application.NewInternal("get integration run use case is not configured", nil)), nil
	}
	projectID, err := request.RequireString("projectId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "projectId"})), nil
	}
	runID, err := request.RequireString("runId")
	if err != nil {
		return toolErrorResult(application.NewInvalidArgument(err.Error(), map[string]any{"field": "runId"})), nil
	}
	out, err := a.services.GetIntegrationRun.Execute(ctx, projectID, runID)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleGetIntegrationHeatmap returns grouped integration heatmap data.
func (a *Adapter) handleGetIntegrationHeatmap(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.GetIntegrationHeatmap == nil {
		return toolErrorResult(application.NewInternal("integration heatmap use case is not configured", nil)), nil
	}
	out, err := a.services.GetIntegrationHeatmap.Execute(ctx, application.IntegrationHeatmapInput{
		Branch:         request.GetString("branch", ""),
		Status:         request.GetString("status", ""),
		RunsPerProject: a.normalizeRunsPerProject(request.GetInt("runsPerProject", a.defaultRunsLimit())),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleIngestCoverageRun ingests a coverage run payload after authenticating write access.
func (a *Adapter) handleIngestCoverageRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !a.cfg.MCPEnableWriteTools || a.services.IngestCoverageRun == nil {
		return toolErrorResult(application.NewUnauthenticated("coverage ingest tool is disabled")), nil
	}
	if err := a.authenticateWriteRequest(ctx, request); err != nil {
		return toolErrorResult(err), nil
	}
	var in application.IngestCoverageRunInput
	if err := bindPayloadOrArguments(request, &in); err != nil {
		return toolErrorResult(application.NewInvalidArgument("invalid coverage ingest payload", map[string]any{"error": err.Error()})), nil
	}
	if err := application.ValidateCoverageIngestInput(in); err != nil {
		return toolErrorResult(err), nil
	}
	out, err := a.services.IngestCoverageRun.Execute(ctx, in)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// handleIngestIntegrationRun ingests an integration run payload after authenticating write access.
func (a *Adapter) handleIngestIntegrationRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !a.cfg.MCPEnableWriteTools || a.services.IngestIntegrationRun == nil {
		return toolErrorResult(application.NewUnauthenticated("integration ingest tool is disabled")), nil
	}
	if err := a.authenticateWriteRequest(ctx, request); err != nil {
		return toolErrorResult(err), nil
	}
	var in application.IngestIntegrationRunInput
	if err := bindPayloadOrArguments(request, &in); err != nil {
		return toolErrorResult(application.NewInvalidArgument("invalid integration ingest payload", map[string]any{"error": err.Error()})), nil
	}
	if err := application.ValidateIntegrationIngestInput(in); err != nil {
		return toolErrorResult(err), nil
	}
	out, err := a.services.IngestIntegrationRun.Execute(ctx, in)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

// readProjectsResource returns the default projects resource payload.
func (a *Adapter) readProjectsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	if a.services.ListProjects == nil {
		return nil, toolProtocolError(application.NewInternal("list projects use case is not configured", nil))
	}
	out, err := a.services.ListProjects.Execute(ctx, application.ListProjectsInput{Page: 1, PageSize: a.normalizePageSize(defaultListPageSize)})
	if err != nil {
		return nil, toolProtocolError(err)
	}
	return jsonResourceContents(resourceProjects, out)
}

// readIntegrationHeatmapResource returns the default integration heatmap resource payload.
func (a *Adapter) readIntegrationHeatmapResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	if a.services.GetIntegrationHeatmap == nil {
		return nil, toolProtocolError(application.NewInternal("integration heatmap use case is not configured", nil))
	}
	out, err := a.services.GetIntegrationHeatmap.Execute(ctx, application.IntegrationHeatmapInput{RunsPerProject: a.defaultRunsLimit()})
	if err != nil {
		return nil, toolProtocolError(err)
	}
	return jsonResourceContents(request.Params.URI, out)
}

// readProjectResource returns project metadata from a project resource URI.
func (a *Adapter) readProjectResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	projectID, suffix, err := parseProjectURI(request.Params.URI)
	if err != nil {
		return nil, err
	}
	if len(suffix) != 0 {
		return nil, fmt.Errorf("resource not found")
	}
	if a.services.GetProject == nil {
		return nil, toolProtocolError(application.NewInternal("get project use case is not configured", nil))
	}
	out, err := a.services.GetProject.Execute(ctx, projectID)
	if err != nil {
		return nil, toolProtocolError(err)
	}
	return jsonResourceContents(request.Params.URI, out)
}

// readProjectCoverageResource returns the latest coverage comparison from a project resource URI.
func (a *Adapter) readProjectCoverageResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	projectID, suffix, err := parseProjectURI(request.Params.URI)
	if err != nil {
		return nil, err
	}
	if len(suffix) != 2 || suffix[0] != "coverage" || suffix[1] != "latest" {
		return nil, fmt.Errorf("resource not found")
	}
	if a.services.GetProject == nil || a.services.GetLatestComparison == nil {
		return nil, toolProtocolError(application.NewInternal("coverage comparison dependencies are not configured", nil))
	}
	project, err := a.services.GetProject.Execute(ctx, projectID)
	if err != nil {
		return nil, toolProtocolError(err)
	}
	out, err := a.services.GetLatestComparison.Execute(ctx, application.GetLatestComparisonInput{ProjectID: projectID})
	if err != nil {
		return nil, toolProtocolError(err)
	}
	return jsonResourceContents(request.Params.URI, coverageComparisonEnvelope{Project: project, Run: out.Run, Comparison: out.Comparison, Packages: out.Packages})
}

// readProjectIntegrationResource returns the latest integration comparison from a project resource URI.
func (a *Adapter) readProjectIntegrationResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	projectID, suffix, err := parseProjectURI(request.Params.URI)
	if err != nil {
		return nil, err
	}
	if len(suffix) != 2 || suffix[0] != "integration" || suffix[1] != "latest" {
		return nil, fmt.Errorf("resource not found")
	}
	if a.services.GetLatestIntegrationCompare == nil {
		return nil, toolProtocolError(application.NewInternal("latest integration comparison use case is not configured", nil))
	}
	out, err := a.services.GetLatestIntegrationCompare.Execute(ctx, projectID)
	if err != nil {
		return nil, toolProtocolError(err)
	}
	return jsonResourceContents(request.Params.URI, out)
}

// readProjectContributorsResource returns contributor data from a project resource URI.
func (a *Adapter) readProjectContributorsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	projectID, suffix, err := parseProjectURI(request.Params.URI)
	if err != nil {
		return nil, err
	}
	if len(suffix) != 1 || suffix[0] != "contributors" {
		return nil, fmt.Errorf("resource not found")
	}
	if a.services.ListContributors == nil {
		return nil, toolProtocolError(application.NewInternal("list contributors use case is not configured", nil))
	}
	out, err := a.services.ListContributors.Execute(ctx, application.ListContributorsInput{ProjectID: projectID, Limit: defaultContributorLimit})
	if err != nil {
		return nil, toolProtocolError(err)
	}
	return jsonResourceContents(request.Params.URI, out)
}

// getSummarizeProjectHealthPrompt builds a prompt for a project health summary workflow.
func (a *Adapter) getSummarizeProjectHealthPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectID := strings.TrimSpace(request.Params.Arguments["projectId"])
	if projectID == "" {
		return nil, application.NewInvalidArgument("projectId is required", map[string]any{"field": "projectId"})
	}
	branch := strings.TrimSpace(request.Params.Arguments["branch"])
	text := fmt.Sprintf("Summarize project health for project %q. First call get_project with projectId=%q. Then call get_latest_coverage_comparison with projectId=%q", projectID, projectID, projectID)
	if branch != "" {
		text += fmt.Sprintf(" and branch=%q", branch)
	}
	text += fmt.Sprintf(". Then call get_latest_integration_comparison with projectId=%q and list_contributors with projectId=%q. Produce a concise health summary covering project metadata, current coverage direction, threshold status, integration pass-rate direction, failed specs if any, and notable contributors.", projectID, projectID)
	return mcp.NewGetPromptResult("Summarize project health", []mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))}), nil
}

// getInvestigateCoverageRegressionPrompt builds a prompt for investigating coverage regressions.
func (a *Adapter) getInvestigateCoverageRegressionPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectID := strings.TrimSpace(request.Params.Arguments["projectId"])
	if projectID == "" {
		return nil, application.NewInvalidArgument("projectId is required", map[string]any{"field": "projectId"})
	}
	branch := strings.TrimSpace(request.Params.Arguments["branch"])
	text := fmt.Sprintf("Investigate coverage regression for project %q. Start with get_latest_coverage_comparison using projectId=%q", projectID, projectID)
	if branch != "" {
		text += fmt.Sprintf(" and branch=%q", branch)
	}
	text += fmt.Sprintf(". Then call list_coverage_runs for recent history and list_branches if branch context is unclear. Focus on package deltas with direction down, compare against default-branch baseline behavior, and summarize likely regression hotspots.")
	return mcp.NewGetPromptResult("Investigate coverage regression", []mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))}), nil
}

// getInvestigateIntegrationFailuresPrompt builds a prompt for investigating integration failures.
func (a *Adapter) getInvestigateIntegrationFailuresPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectID := strings.TrimSpace(request.Params.Arguments["projectId"])
	if projectID == "" {
		return nil, application.NewInvalidArgument("projectId is required", map[string]any{"field": "projectId"})
	}
	environment := strings.TrimSpace(request.Params.Arguments["environment"])
	text := fmt.Sprintf("Investigate integration failures for project %q. Start with get_latest_integration_comparison using projectId=%q. Then call list_integration_runs with projectId=%q", projectID, projectID, projectID)
	if environment != "" {
		text += fmt.Sprintf(" and environment=%q", environment)
	}
	text += ". If the latest comparison shows failures, inspect get_integration_run for the relevant run to review failed specs and summarize new versus resolved failures, likely failure areas, and whether the issue appears branch-specific or systemic."
	return mcp.NewGetPromptResult("Investigate integration failures", []mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))}), nil
}

func (a *Adapter) normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return defaultListPageSize
	}
	limit := a.pageSizeLimit()
	if pageSize > limit {
		return limit
	}
	return pageSize
}

func (a *Adapter) pageSizeLimit() int {
	if a.cfg.MCPMaxPageSize <= 0 || a.cfg.MCPMaxPageSize > applicationMaxPageSize {
		return applicationMaxPageSize
	}
	return a.cfg.MCPMaxPageSize
}

func (a *Adapter) normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func (a *Adapter) normalizeContributorLimit(limit int) int {
	if limit <= 0 {
		return defaultContributorLimit
	}
	if limit > maxContributorLimit {
		return maxContributorLimit
	}
	return limit
}

func (a *Adapter) normalizeRunsPerProject(limit int) int {
	defaultLimit := a.defaultRunsLimit()
	if defaultLimit > maxRunsPerProject {
		defaultLimit = maxRunsPerProject
	}
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxRunsPerProject {
		return maxRunsPerProject
	}
	return limit
}

func (a *Adapter) defaultRunsLimit() int {
	if a.cfg.MCPDefaultRunsLimit <= 0 {
		return 10
	}
	return a.cfg.MCPDefaultRunsLimit
}

func (a *Adapter) authenticateWriteRequest(ctx context.Context, request mcp.CallToolRequest) error {
	if a.authenticator == nil {
		return application.NewUnauthenticated("write tool authentication is not configured")
	}

	apiKey := request.Header.Get(a.cfg.APIKeyHeader)
	if apiKey == "" {
		return application.NewUnauthenticated("missing API key header")
	}

	if err := a.authenticator.Authenticate(ctx, apiKey); err != nil {
		return application.NewUnauthenticated("invalid API key")
	}

	return nil
}

func bindPayloadOrArguments[T any](request mcp.CallToolRequest, target *T) error {
	args := request.GetArguments()
	if payload, ok := args["payload"]; ok {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return json.Unmarshal(encoded, target)
	}
	return request.BindArguments(target)
}
