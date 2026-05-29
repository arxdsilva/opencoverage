package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"
	"github.com/arxdsilva/opencoverage/internal/platform/config"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	resourceProjects                  = "opencoverage://projects"
	resourceIntegrationHeatmap        = "opencoverage://integration/heatmap"
	resourceProjectTemplate           = "opencoverage://projects/{projectId}"
	resourceProjectCoverageTemplate   = "opencoverage://projects/{projectId}/coverage/latest"
	resourceProjectIntegrationTemplate = "opencoverage://projects/{projectId}/integration/latest"
	resourceProjectContributorsTemplate = "opencoverage://projects/{projectId}/contributors"
	jsonMIMEType                      = "application/json"
	defaultListPageSize               = 20
	defaultContributorLimit           = 10
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
	cfg      config.Config
	services Services
}

type coverageComparisonEnvelope struct {
	Project    application.ProjectResponse             `json:"project"`
	Run        application.RunResponse                 `json:"run"`
	Comparison application.ComparisonResponse          `json:"comparison"`
	Packages   []application.PackageComparisonResponse `json:"packages"`
}

func NewServer(cfg config.Config, services Services) *server.MCPServer {
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
	a := &Adapter{cfg: cfg, services: services}
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
		mcp.WithInteger("pageSize", mcp.Description("Number of projects per page."), mcp.Min(1), mcp.Max(a.cfg.MCPMaxPageSize)),
	), a.handleListProjects)

	s.AddTool(mcp.NewTool("get_project",
		mcp.WithDescription("Fetch project metadata by project ID."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
	), a.handleGetProject)

	s.AddTool(mcp.NewTool("list_branches",
		mcp.WithDescription("List known branches for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
	), a.handleListBranches)

	s.AddTool(mcp.NewTool("list_coverage_runs",
		mcp.WithDescription("List paginated coverage run history for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("branch", mcp.Description("Optional branch filter.")),
		mcp.WithString("from", mcp.Description("Optional RFC3339 lower time bound.")),
		mcp.WithString("to", mcp.Description("Optional RFC3339 upper time bound.")),
		mcp.WithInteger("page", mcp.Description("Page number to return."), mcp.Min(1)),
		mcp.WithInteger("pageSize", mcp.Description("Number of runs per page."), mcp.Min(1), mcp.Max(a.cfg.MCPMaxPageSize)),
	), a.handleListCoverageRuns)

	s.AddTool(mcp.NewTool("get_latest_coverage_comparison",
		mcp.WithDescription("Get the latest coverage comparison for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("branch", mcp.Description("Optional branch filter for the current run.")),
	), a.handleGetLatestCoverageComparison)

	s.AddTool(mcp.NewTool("list_contributors",
		mcp.WithDescription("List top contributors for the project's default branch."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithInteger("limit", mcp.Description("Maximum contributors to return."), mcp.Min(1), mcp.Max(25)),
	), a.handleListContributors)

	s.AddTool(mcp.NewTool("list_integration_runs",
		mcp.WithDescription("List paginated integration-test run history for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("branch", mcp.Description("Optional branch filter.")),
		mcp.WithString("status", mcp.Description("Optional status filter: passed or failed."), mcp.Enum("passed", "failed")),
		mcp.WithString("environment", mcp.Description("Optional environment filter."), mcp.Enum("test", "stage", "prod", "none")),
		mcp.WithString("from", mcp.Description("Optional RFC3339 lower time bound.")),
		mcp.WithString("to", mcp.Description("Optional RFC3339 upper time bound.")),
		mcp.WithInteger("page", mcp.Description("Page number to return."), mcp.Min(1)),
		mcp.WithInteger("pageSize", mcp.Description("Number of runs per page."), mcp.Min(1), mcp.Max(a.cfg.MCPMaxPageSize)),
	), a.handleListIntegrationRuns)

	s.AddTool(mcp.NewTool("get_latest_integration_comparison",
		mcp.WithDescription("Get the latest integration-test comparison for a project."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
	), a.handleGetLatestIntegrationComparison)

	s.AddTool(mcp.NewTool("get_integration_run",
		mcp.WithDescription("Get a specific integration-test run and its failed specs."),
		mcp.WithString("projectId", mcp.Required(), mcp.Description("Project ID.")),
		mcp.WithString("runId", mcp.Required(), mcp.Description("Integration run ID.")),
	), a.handleGetIntegrationRun)

	s.AddTool(mcp.NewTool("get_integration_heatmap",
		mcp.WithDescription("Get grouped integration heatmap data across projects."),
		mcp.WithString("branch", mcp.Description("Optional branch filter.")),
		mcp.WithString("status", mcp.Description("Optional status filter: passed or failed."), mcp.Enum("passed", "failed")),
		mcp.WithInteger("runsPerProject", mcp.Description("Maximum runs to include per project."), mcp.Min(1), mcp.Max(30)),
	), a.handleGetIntegrationHeatmap)

	if a.cfg.MCPEnableWriteTools {
		s.AddTool(mcp.NewTool("ingest_coverage_run",
			mcp.WithDescription("Ingest a coverage run using the existing OpenCoverage payload contract."),
		), a.handleIngestCoverageRun)
		s.AddTool(mcp.NewTool("ingest_integration_run",
			mcp.WithDescription("Ingest an integration-test run using the existing OpenCoverage payload contract."),
		), a.handleIngestIntegrationRun)
	}
}

func (a *Adapter) registerResources(s *server.MCPServer) {
	s.AddResource(mcp.NewResource(resourceProjects, "Projects",
		mcp.WithResourceDescription("Default project catalog view."),
		mcp.WithMIMEType(jsonMIMEType),
	), a.readProjectsResource)

	s.AddResource(mcp.NewResource(resourceIntegrationHeatmap, "Integration Heatmap",
		mcp.WithResourceDescription("Default grouped integration heatmap view."),
		mcp.WithMIMEType(jsonMIMEType),
	), a.readIntegrationHeatmapResource)

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectTemplate, "Project",
		mcp.WithTemplateDescription("Project metadata by project ID."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.readProjectResource)

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectCoverageTemplate, "Latest Coverage Comparison",
		mcp.WithTemplateDescription("Latest coverage comparison for a project."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.readProjectCoverageResource)

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectIntegrationTemplate, "Latest Integration Comparison",
		mcp.WithTemplateDescription("Latest integration comparison for a project."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.readProjectIntegrationResource)

	s.AddResourceTemplate(mcp.NewResourceTemplate(resourceProjectContributorsTemplate, "Contributors",
		mcp.WithTemplateDescription("Contributor summary for a project's default branch."),
		mcp.WithTemplateMIMEType(jsonMIMEType),
	), a.readProjectContributorsResource)
}

func (a *Adapter) registerPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("summarize_project_health",
		mcp.WithPromptDescription("Guide the client to summarize a project's current health."),
		mcp.WithArgument("projectId", mcp.RequiredArgument(), mcp.ArgumentDescription("Project ID to inspect.")),
		mcp.WithArgument("branch", mcp.ArgumentDescription("Optional branch to emphasize for coverage.")),
	), a.getSummarizeProjectHealthPrompt)

	s.AddPrompt(mcp.NewPrompt("investigate_coverage_regression",
		mcp.WithPromptDescription("Guide the client through a coverage regression investigation."),
		mcp.WithArgument("projectId", mcp.RequiredArgument(), mcp.ArgumentDescription("Project ID to inspect.")),
		mcp.WithArgument("branch", mcp.ArgumentDescription("Optional branch to inspect.")),
	), a.getInvestigateCoverageRegressionPrompt)

	s.AddPrompt(mcp.NewPrompt("investigate_integration_failures",
		mcp.WithPromptDescription("Guide the client through an integration failure investigation."),
		mcp.WithArgument("projectId", mcp.RequiredArgument(), mcp.ArgumentDescription("Project ID to inspect.")),
		mcp.WithArgument("environment", mcp.ArgumentDescription("Optional environment to focus on.")),
	), a.getInvestigateIntegrationFailuresPrompt)
}

func (a *Adapter) handleListProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.ListProjects == nil {
		return toolErrorResult(application.NewInternal("list projects use case is not configured", nil)), nil
	}
	out, err := a.services.ListProjects.Execute(ctx, application.ListProjectsInput{
		Page:     request.GetInt("page", 1),
		PageSize: a.normalizePageSize(request.GetInt("pageSize", defaultListPageSize)),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

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
		Page:      request.GetInt("page", 1),
		PageSize:  a.normalizePageSize(request.GetInt("pageSize", defaultListPageSize)),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

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
		Limit:     request.GetInt("limit", defaultContributorLimit),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

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
		Page:        request.GetInt("page", 1),
		PageSize:    a.normalizePageSize(request.GetInt("pageSize", defaultListPageSize)),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

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

func (a *Adapter) handleGetIntegrationHeatmap(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if a.services.GetIntegrationHeatmap == nil {
		return toolErrorResult(application.NewInternal("integration heatmap use case is not configured", nil)), nil
	}
	out, err := a.services.GetIntegrationHeatmap.Execute(ctx, application.IntegrationHeatmapInput{
		Branch:         request.GetString("branch", ""),
		Status:         request.GetString("status", ""),
		RunsPerProject: request.GetInt("runsPerProject", a.defaultRunsLimit()),
	})
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

func (a *Adapter) handleIngestCoverageRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !a.cfg.MCPEnableWriteTools || a.services.IngestCoverageRun == nil {
		return toolErrorResult(application.NewUnauthenticated("coverage ingest tool is disabled")), nil
	}
	var in application.IngestCoverageRunInput
	if err := request.BindArguments(&in); err != nil {
		return toolErrorResult(application.NewInvalidArgument("invalid coverage ingest payload", map[string]any{"error": err.Error()})), nil
	}
	out, err := a.services.IngestCoverageRun.Execute(ctx, in)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

func (a *Adapter) handleIngestIntegrationRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !a.cfg.MCPEnableWriteTools || a.services.IngestIntegrationRun == nil {
		return toolErrorResult(application.NewUnauthenticated("integration ingest tool is disabled")), nil
	}
	var in application.IngestIntegrationRunInput
	if err := request.BindArguments(&in); err != nil {
		return toolErrorResult(application.NewInvalidArgument("invalid integration ingest payload", map[string]any{"error": err.Error()})), nil
	}
	out, err := a.services.IngestIntegrationRun.Execute(ctx, in)
	if err != nil {
		return toolErrorResult(err), nil
	}
	return toolJSONResult(out)
}

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

func (a *Adapter) getSummarizeProjectHealthPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectID := strings.TrimSpace(request.Params.Arguments["projectId"])
	branch := strings.TrimSpace(request.Params.Arguments["branch"])
	text := fmt.Sprintf("Summarize project health for project %q. First call get_project with projectId=%q. Then call get_latest_coverage_comparison with projectId=%q", projectID, projectID, projectID)
	if branch != "" {
		text += fmt.Sprintf(" and branch=%q", branch)
	}
	text += fmt.Sprintf(". Then call get_latest_integration_comparison with projectId=%q and list_contributors with projectId=%q. Produce a concise health summary covering project metadata, current coverage direction, threshold status, integration pass-rate direction, failed specs if any, and notable contributors.", projectID, projectID)
	return mcp.NewGetPromptResult("Summarize project health", []mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))}), nil
}

func (a *Adapter) getInvestigateCoverageRegressionPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectID := strings.TrimSpace(request.Params.Arguments["projectId"])
	branch := strings.TrimSpace(request.Params.Arguments["branch"])
	text := fmt.Sprintf("Investigate coverage regression for project %q. Start with get_latest_coverage_comparison using projectId=%q", projectID, projectID)
	if branch != "" {
		text += fmt.Sprintf(" and branch=%q", branch)
	}
	text += fmt.Sprintf(". Then call list_coverage_runs for recent history and list_branches if branch context is unclear. Focus on package deltas with direction down, compare against default-branch baseline behavior, and summarize likely regression hotspots.")
	return mcp.NewGetPromptResult("Investigate coverage regression", []mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))}), nil
}

func (a *Adapter) getInvestigateIntegrationFailuresPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectID := strings.TrimSpace(request.Params.Arguments["projectId"])
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
	if pageSize > a.cfg.MCPMaxPageSize {
		return a.cfg.MCPMaxPageSize
	}
	return pageSize
}

func (a *Adapter) defaultRunsLimit() int {
	if a.cfg.MCPDefaultRunsLimit <= 0 {
		return 10
	}
	return a.cfg.MCPDefaultRunsLimit
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
	payload := map[string]any{"error": errorPayload(err)}
	result := mcp.NewToolResultStructured(payload, fmt.Sprintf("%s: %s", payload["error"].(map[string]any)["code"], payload["error"].(map[string]any)["message"]))
	result.IsError = true
	return result
}

func toolProtocolError(err error) error {
	payload := errorPayload(err)
	return fmt.Errorf("%s: %s", payload["code"], payload["message"])
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
		payload["message"] = err.Error()
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