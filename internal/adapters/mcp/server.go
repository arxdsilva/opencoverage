package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"
	"github.com/arxdsilva/opencoverage/internal/platform/config"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	resourceProjects                    = "opencoverage://projects"
	resourceIntegrationHeatmap          = "opencoverage://integration/heatmap"
	resourceProjectTemplate             = "opencoverage://projects/{projectId}"
	resourceProjectCoverageTemplate     = "opencoverage://projects/{projectId}/coverage/latest"
	resourceProjectIntegrationTemplate  = "opencoverage://projects/{projectId}/integration/latest"
	resourceProjectContributorsTemplate = "opencoverage://projects/{projectId}/contributors"
	jsonMIMEType                        = "application/json"
	defaultListPageSize                 = 20
	defaultContributorLimit             = 10
)

type ListProjectsExecutor interface {
	Execute(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error)
}

type GetProjectExecutor interface {
	Execute(ctx context.Context, projectID string) (application.ProjectResponse, error)
}

type ListCoverageRunsExecutor interface {
	Execute(ctx context.Context, in application.ListCoverageRunsInput) (application.ListCoverageRunsOutput, error)
}

type GetLatestComparisonExecutor interface {
	Execute(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error)
}

type ListBranchesExecutor interface {
	Execute(ctx context.Context, projectID string) (application.ListBranchesOutput, error)
}

type ListContributorsExecutor interface {
	Execute(ctx context.Context, in application.ListContributorsInput) (application.ListContributorsOutput, error)
}

type ListIntegrationRunsExecutor interface {
	Execute(ctx context.Context, in application.ListIntegrationRunsInput) (application.ListIntegrationRunsOutput, error)
}

type GetLatestIntegrationComparisonExecutor interface {
	Execute(ctx context.Context, projectID string) (application.IngestIntegrationRunOutput, error)
}

type GetIntegrationRunExecutor interface {
	Execute(ctx context.Context, projectID string, runID string) (application.IngestIntegrationRunOutput, error)
}

type GetIntegrationHeatmapExecutor interface {
	Execute(ctx context.Context, in application.IntegrationHeatmapInput) (application.GetIntegrationHeatmapOutput, error)
}

type IngestCoverageRunExecutor interface {
	Execute(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error)
}

type IngestIntegrationRunExecutor interface {
	Execute(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error)
}

type Services struct {
	ListProjects                ListProjectsExecutor
	GetProject                  GetProjectExecutor
	ListCoverageRuns            ListCoverageRunsExecutor
	GetLatestComparison         GetLatestComparisonExecutor
	ListBranches                ListBranchesExecutor
	ListContributors            ListContributorsExecutor
	ListIntegrationRuns         ListIntegrationRunsExecutor
	GetLatestIntegrationCompare GetLatestIntegrationComparisonExecutor
	GetIntegrationRun           GetIntegrationRunExecutor
	GetIntegrationHeatmap       GetIntegrationHeatmapExecutor
	IngestCoverageRun           IngestCoverageRunExecutor
	IngestIntegrationRun        IngestIntegrationRunExecutor
}

type Adapter struct {
	cfg           config.Config
	services      Services
	authenticator application.APIKeyAuthenticator
}

type coverageComparisonEnvelope struct {
	Project    application.ProjectResponse             `json:"project"`
	Run        application.RunResponse                 `json:"run"`
	Comparison application.ComparisonResponse          `json:"comparison"`
	Packages   []application.PackageComparisonResponse `json:"packages"`
}

func NewServer(cfg config.Config, services Services, authenticator application.APIKeyAuthenticator) *server.MCPServer {
	opts := []server.ServerOption{
		server.WithRecovery(),
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithInstructions("OpenCoverage MCP server for coverage, contributor, and integration-test data."),
	}
	if cfg.MCPEnablePrompts {
		opts = append(opts, server.WithPromptCapabilities(false))
	}

	s := server.NewMCPServer(cfg.MCPServerName, cfg.MCPServerVersion, opts...)
	a := &Adapter{cfg: cfg, services: services, authenticator: authenticator}
	a.registerTools(s)
	a.registerResources(s)
	if cfg.MCPEnablePrompts {
		a.registerPrompts(s)
	}

	return s
}

func (a *Adapter) registerTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_projects",
		mcp.WithDescription("List projects in OpenCoverage."),
		mcp.WithInteger("page", mcp.Description("Page number to return."), mcp.Min(1)),
		mcp.WithInteger("pageSize", mcp.Description("Number of projects per page."), mcp.Min(1), mcp.Max(a.pageSizeLimit())),
	), a.withToolLogging("list_projects", a.handleListProjects))

	s.AddTool(mcp.NewTool("get_project",
		mcp.WithDescription("Fetch project metadata by project ID."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
	), a.withToolLogging("get_project", a.handleGetProject))

	s.AddTool(mcp.NewTool("list_branches",
		mcp.WithDescription("List known branches for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
	), a.withToolLogging("list_branches", a.handleListBranches))

	s.AddTool(mcp.NewTool("list_coverage_runs",
		mcp.WithDescription("List paginated coverage run history for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("branch", mcp.Description("Optional branch filter.")),
		mcp.WithString("from", mcp.Description("Optional RFC3339 lower time bound.")),
		mcp.WithString("to", mcp.Description("Optional RFC3339 upper time bound.")),
		mcp.WithInteger("page", mcp.Description("Page number to return."), mcp.Min(1)),
		mcp.WithInteger("pageSize", mcp.Description("Number of runs per page."), mcp.Min(1), mcp.Max(a.pageSizeLimit())),
	), a.withToolLogging("list_coverage_runs", a.handleListCoverageRuns))

	s.AddTool(mcp.NewTool("get_latest_coverage_comparison",
		mcp.WithDescription("Get the latest coverage comparison for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("branch", mcp.Description("Optional branch filter for the current run.")),
	), a.withToolLogging("get_latest_coverage_comparison", a.handleGetLatestCoverageComparison))

	s.AddTool(mcp.NewTool("list_contributors",
		mcp.WithDescription("List top contributors for the project's default branch."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithInteger("limit", mcp.Description("Maximum contributors to return."), mcp.Min(1), mcp.Max(25)),
	), a.withToolLogging("list_contributors", a.handleListContributors))

	s.AddTool(mcp.NewTool("list_integration_runs",
		mcp.WithDescription("List paginated integration-test run history for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("branch", mcp.Description("Optional branch filter.")),
		mcp.WithString("status", mcp.Description("Optional status filter: passed or failed."), mcp.Enum("passed", "failed")),
		mcp.WithString("environment", mcp.Description("Optional environment filter."), mcp.Enum("test", "stage", "prod", "none")),
		mcp.WithString("from", mcp.Description("Optional RFC3339 lower time bound.")),
		mcp.WithString("to", mcp.Description("Optional RFC3339 upper time bound.")),
		mcp.WithInteger("page", mcp.Description("Page number to return."), mcp.Min(1)),
		mcp.WithInteger("pageSize", mcp.Description("Number of runs per page."), mcp.Min(1), mcp.Max(a.pageSizeLimit())),
	), a.withToolLogging("list_integration_runs", a.handleListIntegrationRuns))

	s.AddTool(mcp.NewTool("get_latest_integration_comparison",
		mcp.WithDescription("Get the latest integration-test comparison for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
	), a.withToolLogging("get_latest_integration_comparison", a.handleGetLatestIntegrationComparison))

	s.AddTool(mcp.NewTool("get_integration_run",
		mcp.WithDescription("Get a specific integration-test run and its failed specs."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("runId", mcp.Required(), mcp.Description("Integration run ID.")),
	), a.withToolLogging("get_integration_run", a.handleGetIntegrationRun))

	s.AddTool(mcp.NewTool("get_integration_heatmap",
		mcp.WithDescription("Get grouped integration heatmap data across projects."),
		mcp.WithString("branch", mcp.Description("Optional branch filter.")),
		mcp.WithString("status", mcp.Description("Optional status filter: passed or failed."), mcp.Enum("passed", "failed")),
		mcp.WithInteger("runsPerProject", mcp.Description("Maximum runs to include per project."), mcp.Min(1), mcp.Max(30)),
	), a.withToolLogging("get_integration_heatmap", a.handleGetIntegrationHeatmap))

	if a.cfg.MCPEnableWriteTools {
		s.AddTool(mcp.NewTool("ingest_coverage_run",
			mcp.WithDescription("Ingest a coverage run using the existing OpenCoverage payload contract."),
			mcp.WithObject("payload",
				mcp.Required(),
				mcp.Description("Coverage ingest payload."),
				func(schema map[string]any) {
					schema["required"] = coverageIngestPayloadRequired()
				},
				mcp.Properties(coverageIngestPayloadProperties()),
			),
		), a.withToolLogging("ingest_coverage_run", a.handleIngestCoverageRun))
		s.AddTool(mcp.NewTool("ingest_integration_run",
			mcp.WithDescription("Ingest an integration-test run using the existing OpenCoverage payload contract."),
			mcp.WithObject("payload",
				mcp.Required(),
				mcp.Description("Integration ingest payload."),
				func(schema map[string]any) {
					schema["required"] = integrationIngestPayloadRequired()
				},
				mcp.Properties(integrationIngestPayloadProperties()),
			),
		), a.withToolLogging("ingest_integration_run", a.handleIngestIntegrationRun))
	}
}

func (a *Adapter) registerResources(s *server.MCPServer) {
	s.AddResource(mcp.NewResource(resourceProjects, "Projects",
		mcp.WithResourceDescription("Default project catalog view."),
		mcp.WithMIMEType(jsonMIMEType),
	), a.withResourceLogging(resourceProjects, a.readProjectsResource))

	s.AddResource(mcp.NewResource(resourceIntegrationHeatmap, "Integration Heatmap",
		mcp.WithResourceDescription("Default grouped integration heatmap view."),
		mcp.WithMIMEType(jsonMIMEType),
	), a.withResourceLogging(resourceIntegrationHeatmap, a.readIntegrationHeatmapResource))

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectTemplate, "Project",
		mcp.WithTemplateDescription("Project metadata by project ID."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.withResourceLogging(resourceProjectTemplate, a.readProjectResource))

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectCoverageTemplate, "Latest Coverage Comparison",
		mcp.WithTemplateDescription("Latest coverage comparison for a project."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.withResourceLogging(resourceProjectCoverageTemplate, a.readProjectCoverageResource))

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectIntegrationTemplate, "Latest Integration Comparison",
		mcp.WithTemplateDescription("Latest integration comparison for a project."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.withResourceLogging(resourceProjectIntegrationTemplate, a.readProjectIntegrationResource))

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectContributorsTemplate, "Contributors",
		mcp.WithTemplateDescription("Contributor summary for a project's default branch."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.withResourceLogging(resourceProjectContributorsTemplate, a.readProjectContributorsResource))
}

func (a *Adapter) registerPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("summarize_project_health",
		mcp.WithPromptDescription("Guide the client to summarize a project's current health."),
		mcp.WithArgument("projectId", mcp.RequiredArgument(), mcp.ArgumentDescription("Project ID to inspect.")),
		mcp.WithArgument("branch", mcp.ArgumentDescription("Optional branch to emphasize for coverage.")),
	), a.withPromptLogging("summarize_project_health", a.getSummarizeProjectHealthPrompt))

	s.AddPrompt(mcp.NewPrompt("investigate_coverage_regression",
		mcp.WithPromptDescription("Guide the client through a coverage regression investigation."),
		mcp.WithArgument("projectId", mcp.RequiredArgument(), mcp.ArgumentDescription("Project ID to inspect.")),
		mcp.WithArgument("branch", mcp.ArgumentDescription("Optional branch to inspect.")),
	), a.withPromptLogging("investigate_coverage_regression", a.getInvestigateCoverageRegressionPrompt))

	s.AddPrompt(mcp.NewPrompt("investigate_integration_failures",
		mcp.WithPromptDescription("Guide the client through an integration failure investigation."),
		mcp.WithArgument("projectId", mcp.RequiredArgument(), mcp.ArgumentDescription("Project ID to inspect.")),
		mcp.WithArgument("environment", mcp.ArgumentDescription("Optional environment to focus on.")),
	), a.withPromptLogging("investigate_integration_failures", a.getInvestigateIntegrationFailuresPrompt))
}

func (a *Adapter) withToolLogging(name string, next func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		startedAt := time.Now()
		slog.Info("mcp_tool_handled", "tool", name, "phase", "start")
		result, err := next(ctx, request)
		if err != nil {
			slog.Error("mcp_tool_handled", "tool", name, "phase", "error", "error", err, "duration_ms", time.Since(startedAt).Milliseconds())
			return nil, err
		}
		if result != nil && result.IsError {
			slog.Error("mcp_tool_handled", "tool", name, "phase", "error", "error", "tool returned error result", "duration_ms", time.Since(startedAt).Milliseconds())
			return result, nil
		}
		slog.Info("mcp_tool_handled", "tool", name, "phase", "success", "duration_ms", time.Since(startedAt).Milliseconds())
		return result, nil
	}
}

func (a *Adapter) withResourceLogging(name string, next func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)) func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		startedAt := time.Now()
		slog.Info("mcp_resource_handled", "resource", name, "phase", "start")
		contents, err := next(ctx, request)
		if err != nil {
			slog.Error("mcp_resource_handled", "resource", name, "phase", "error", "error", err, "duration_ms", time.Since(startedAt).Milliseconds())
			return nil, err
		}
		slog.Info("mcp_resource_handled", "resource", name, "phase", "success", "duration_ms", time.Since(startedAt).Milliseconds())
		return contents, nil
	}
}

func (a *Adapter) withPromptLogging(name string, next func(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error)) func(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		startedAt := time.Now()
		slog.Info("mcp_prompt_handled", "prompt", name, "phase", "start")
		result, err := next(ctx, request)
		if err != nil {
			slog.Error("mcp_prompt_handled", "prompt", name, "phase", "error", "error", err, "duration_ms", time.Since(startedAt).Milliseconds())
			return nil, err
		}
		slog.Info("mcp_prompt_handled", "prompt", name, "phase", "success", "duration_ms", time.Since(startedAt).Milliseconds())
		return result, nil
	}
}

func parseOptionalTime(raw string, field string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, application.NewInvalidArgument(field+" must be RFC3339", map[string]any{"field": field})
	}
	return &parsed, nil
}

func parseProjectURI(raw string) (string, []string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("invalid resource uri: %w", err)
	}
	if u.Scheme != "opencoverage" || u.Host != "projects" {
		return "", nil, fmt.Errorf("resource not found")
	}
	segments := splitPath(u.Path)
	if len(segments) == 0 {
		return "", nil, fmt.Errorf("resource not found")
	}
	return segments[0], segments[1:], nil
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func jsonResourceContents(uri string, payload any) ([]mcp.ResourceContents, error) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{mcp.TextResourceContents{
		URI:      uri,
		MIMEType: jsonMIMEType,
		Text:     string(encoded),
	}}, nil
}

func toolJSONResult(payload any) (*mcp.CallToolResult, error) {
	result, err := mcp.NewToolResultJSON(payload)
	if err != nil {
		return toolErrorResult(application.NewInternal("failed to encode tool result", err)), nil
	}
	return result, nil
}

func toolErrorResult(err error) *mcp.CallToolResult {
	errorBody := errorPayload(err)
	payload := map[string]any{"error": errorBody}
	result := mcp.NewToolResultStructured(
		payload,
		formatErrorSummary(errorBody),
	)
	result.IsError = true
	return result
}

func toolProtocolError(err error) error {
	payload := errorPayload(err)
	return fmt.Errorf("%s", formatErrorSummary(payload))
}

func formatErrorSummary(payload map[string]any) string {
	code, _ := payload["code"].(string)
	if code == "" {
		code = "internal"
	}

	message, _ := payload["message"].(string)
	if message == "" {
		message = "internal server error"
	}

	return fmt.Sprintf("%s: %s", code, message)
}

func errorPayload(err error) map[string]any {
	payload := map[string]any{
		"code":    "internal",
		"message": "internal server error",
	}
	var appErr *application.AppError
	if ok := asAppError(err, &appErr); ok && appErr != nil {
		payload["code"] = mapErrorCode(appErr.Code)
		payload["message"] = appErr.Message
		if len(appErr.Details) > 0 {
			payload["details"] = appErr.Details
		}
		return payload
	}
	if err != nil {
		slog.Error("mcp_error_fallback", "error", err)
	}
	return payload
}

func asAppError(err error, target **application.AppError) bool {
	var appErr *application.AppError
	if errors.As(err, &appErr) {
		*target = appErr
		return true
	}
	return false
}

func mapErrorCode(code application.ErrorCode) string {
	switch code {
	case application.CodeInvalidArgument:
		return "invalid_argument"
	case application.CodeNotFound:
		return "not_found"
	case application.CodeUnauthenticated:
		return "unauthorized"
	default:
		return "internal"
	}
}
