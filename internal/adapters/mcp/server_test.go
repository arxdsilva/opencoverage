package mcpadapter

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"
	"github.com/arxdsilva/opencoverage/internal/platform/config"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServerRegistersReadOnlySurface(t *testing.T) {
	cfg := config.Config{
		MCPServerName:       "opencoverage",
		MCPServerVersion:    "test",
		MCPTransport:        "stdio",
		MCPMaxPageSize:      100,
		MCPDefaultRunsLimit: 20,
		MCPEnablePrompts:    true,
		MCPEnableWriteTools: false,
	}

	s := NewServer(cfg, Services{}, nil)

	if got := len(s.ListTools()); got != 10 {
		t.Fatalf("expected 10 read tools, got %d", got)
	}
	if got := len(s.ListResources()); got != 2 {
		t.Fatalf("expected 2 static resources, got %d", got)
	}
	if got := len(s.ListPrompts()); got != 3 {
		t.Fatalf("expected 3 prompts, got %d", got)
	}
	if s.GetTool("ingest_coverage_run") != nil {
		t.Fatalf("expected write tool to be disabled")
	}
}

func TestListProjectsToolHandlerReturnsStructuredJSON(t *testing.T) {
	cfg := config.Config{
		MCPServerName:       "opencoverage",
		MCPServerVersion:    "test",
		MCPTransport:        "stdio",
		MCPMaxPageSize:      100,
		MCPDefaultRunsLimit: 20,
	}
	s := NewServer(cfg, Services{
		ListProjects: stubListProjects(func(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error) {
			if in.Page != 2 {
				t.Fatalf("expected page 2, got %d", in.Page)
			}
			if in.PageSize != 5 {
				t.Fatalf("expected page size 5, got %d", in.PageSize)
			}
			return application.ListProjectsOutput{
				Items:      []application.ProjectResponse{{ID: "project-1", ProjectKey: "org/repo", Name: "repo", DefaultBranch: "main", GlobalThresholdPercent: 80}},
				Pagination: application.PaginationResponse{Page: 2, PageSize: 5, TotalItems: 1, TotalPages: 1},
			}, nil
		}),
	}, nil)

	tool := s.GetTool("list_projects")
	if tool == nil {
		t.Fatalf("expected list_projects tool to be registered")
	}

	result, err := tool.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{
		Name:      "list_projects",
		Arguments: map[string]any{"page": 2, "pageSize": 5},
	}})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful result, got error: %#v", result.StructuredContent)
	}
	if result.StructuredContent == nil {
		t.Fatalf("expected structured content in result")
	}
	content, ok := result.StructuredContent.(application.ListProjectsOutput)
	if !ok {
		t.Fatalf("expected structured content to be ListProjectsOutput, got %T", result.StructuredContent)
	}
	if len(content.Items) != 1 {
		t.Fatalf("expected one project item, got %#v", content.Items)
	}
}

func TestListProjectsToolHandlerMapsAppError(t *testing.T) {
	cfg := config.Config{
		MCPServerName:       "opencoverage",
		MCPServerVersion:    "test",
		MCPTransport:        "stdio",
		MCPMaxPageSize:      100,
		MCPDefaultRunsLimit: 20,
	}
	s := NewServer(cfg, Services{
		ListProjects: stubListProjects(func(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error) {
			return application.ListProjectsOutput{}, application.NewNotFound("project catalog unavailable", map[string]any{"scope": "test"})
		}),
	}, nil)

	tool := s.GetTool("list_projects")
	result, err := tool.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "list_projects", Arguments: map[string]any{}}})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result")
	}
	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured error content, got %T", result.StructuredContent)
	}
	errorBody, ok := content["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error body, got %#v", content)
	}
	if errorBody["code"] != "not_found" {
		t.Fatalf("expected not_found code, got %#v", errorBody["code"])
	}
}

type stubListProjects func(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error)

func (s stubListProjects) Execute(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error) {
	return s(ctx, in)
}

func TestIngestCoverageRunRequiresAuthentication(t *testing.T) {
	cfg := config.Config{
		MCPServerName:       "opencoverage",
		MCPServerVersion:    "test",
		MCPTransport:        "stdio",
		MCPMaxPageSize:      100,
		MCPDefaultRunsLimit: 20,
		MCPEnableWriteTools: true,
		APIKeyHeader:        "X-API-Key",
	}

	s := NewServer(cfg, Services{
		IngestCoverageRun: stubIngestCoverageRun(func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
			t.Fatalf("ingest use case should not be called when auth fails")
			return application.IngestCoverageRunOutput{}, nil
		}),
	}, stubAuthenticator{expected: "secret-key"})

	tool := s.GetTool("ingest_coverage_run")
	if tool == nil {
		t.Fatalf("expected ingest_coverage_run tool to be registered")
	}

	result, err := tool.Handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "ingest_coverage_run", Arguments: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected authentication error result")
	}

	content, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured error content, got %T", result.StructuredContent)
	}
	errorBody, ok := content["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error body, got %#v", content)
	}
	if errorBody["code"] != "unauthorized" {
		t.Fatalf("expected unauthorized code, got %#v", errorBody["code"])
	}
}

func TestWriteToolsAdvertisePayloadSchema(t *testing.T) {
	cfg := config.Config{
		MCPServerName:       "opencoverage",
		MCPServerVersion:    "test",
		MCPTransport:        "stdio",
		MCPMaxPageSize:      500,
		MCPDefaultRunsLimit: 20,
		MCPEnableWriteTools: true,
	}

	s := NewServer(cfg, Services{}, stubAuthenticator{expected: "secret-key"})
	coverageTool := s.GetTool("ingest_coverage_run")
	if coverageTool == nil {
		t.Fatalf("expected ingest_coverage_run tool to be registered")
	}
	if _, ok := coverageTool.Tool.InputSchema.Properties["payload"]; !ok {
		t.Fatalf("expected ingest_coverage_run schema to advertise payload")
	}
	if _, ok := coverageTool.Tool.InputSchema.Properties["apiKey"]; ok {
		t.Fatalf("expected ingest_coverage_run schema to omit apiKey")
	}
	coveragePayload, ok := coverageTool.Tool.InputSchema.Properties["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected coverage payload schema to be object, got %T", coverageTool.Tool.InputSchema.Properties["payload"])
	}
	coverageRequired, ok := coveragePayload["required"].([]string)
	if !ok {
		t.Fatalf("expected coverage payload required list, got %#v", coveragePayload["required"])
	}
	if !slices.Contains(coverageRequired, "projectKey") || !slices.Contains(coverageRequired, "packages") {
		t.Fatalf("expected coverage payload required fields to include projectKey and packages, got %#v", coverageRequired)
	}
	coveragePageSize, ok := s.GetTool("list_projects").Tool.InputSchema.Properties["pageSize"].(map[string]any)
	if !ok {
		t.Fatalf("expected list_projects pageSize schema to be object, got %T", s.GetTool("list_projects").Tool.InputSchema.Properties["pageSize"])
	}
	if coveragePageSize["maximum"] != float64(applicationMaxPageSize) && coveragePageSize["maximum"] != applicationMaxPageSize {
		t.Fatalf("expected pageSize maximum to be %d, got %#v", applicationMaxPageSize, coveragePageSize["maximum"])
	}
	coverageRunsPageSize, ok := s.GetTool("list_coverage_runs").Tool.InputSchema.Properties["pageSize"].(map[string]any)
	if !ok {
		t.Fatalf("expected list_coverage_runs pageSize schema to be object, got %T", s.GetTool("list_coverage_runs").Tool.InputSchema.Properties["pageSize"])
	}
	if coverageRunsPageSize["maximum"] != float64(applicationMaxPageSize) && coverageRunsPageSize["maximum"] != applicationMaxPageSize {
		t.Fatalf("expected coverage runs pageSize maximum to be %d, got %#v", applicationMaxPageSize, coverageRunsPageSize["maximum"])
	}
	integrationRunsPageSize, ok := s.GetTool("list_integration_runs").Tool.InputSchema.Properties["pageSize"].(map[string]any)
	if !ok {
		t.Fatalf("expected list_integration_runs pageSize schema to be object, got %T", s.GetTool("list_integration_runs").Tool.InputSchema.Properties["pageSize"])
	}
	if integrationRunsPageSize["maximum"] != float64(applicationMaxPageSize) && integrationRunsPageSize["maximum"] != applicationMaxPageSize {
		t.Fatalf("expected integration runs pageSize maximum to be %d, got %#v", applicationMaxPageSize, integrationRunsPageSize["maximum"])
	}

	integrationTool := s.GetTool("ingest_integration_run")
	if integrationTool == nil {
		t.Fatalf("expected ingest_integration_run tool to be registered")
	}
	if _, ok := integrationTool.Tool.InputSchema.Properties["payload"]; !ok {
		t.Fatalf("expected ingest_integration_run schema to advertise payload")
	}
	if _, ok := integrationTool.Tool.InputSchema.Properties["apiKey"]; ok {
		t.Fatalf("expected ingest_integration_run schema to omit apiKey")
	}
	integrationPayload, ok := integrationTool.Tool.InputSchema.Properties["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected integration payload schema to be object, got %T", integrationTool.Tool.InputSchema.Properties["payload"])
	}
	integrationRequired, ok := integrationPayload["required"].([]string)
	if !ok {
		t.Fatalf("expected integration payload required list, got %#v", integrationPayload["required"])
	}
	if !slices.Contains(integrationRequired, "projectKey") || !slices.Contains(integrationRequired, "ginkgoReport") {
		t.Fatalf("expected integration payload required fields to include projectKey and ginkgoReport, got %#v", integrationRequired)
	}

	integrationProperties, ok := integrationPayload["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected integration payload properties map, got %T", integrationPayload["properties"])
	}
	ginkgoReport, ok := integrationProperties["ginkgoReport"].(map[string]any)
	if !ok {
		t.Fatalf("expected ginkgoReport schema object, got %T", integrationProperties["ginkgoReport"])
	}
	ginkgoProperties, ok := ginkgoReport["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected ginkgoReport properties map, got %T", ginkgoReport["properties"])
	}
	if _, ok := ginkgoProperties["suiteSucceeded"]; !ok {
		t.Fatalf("expected ginkgoReport to advertise suiteSucceeded")
	}
	if _, ok := ginkgoProperties["specialSuiteFailureReasons"]; !ok {
		t.Fatalf("expected ginkgoReport to advertise specialSuiteFailureReasons")
	}

	specReports, ok := ginkgoProperties["specReports"].(map[string]any)
	if !ok {
		t.Fatalf("expected specReports schema object, got %T", ginkgoProperties["specReports"])
	}
	specItems, ok := specReports["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected specReports items schema object, got %T", specReports["items"])
	}
	specProperties, ok := specItems["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected specReports item properties map, got %T", specItems["properties"])
	}
	failureSchema, ok := specProperties["failure"].(map[string]any)
	if !ok {
		t.Fatalf("expected failure schema object, got %T", specProperties["failure"])
	}
	failureProperties, ok := failureSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected failure properties map, got %T", failureSchema["properties"])
	}
	if _, ok := failureProperties["message"]; !ok {
		t.Fatalf("expected failure schema to advertise message")
	}
	locationSchema, ok := failureProperties["location"].(map[string]any)
	if !ok {
		t.Fatalf("expected failure location schema object, got %T", failureProperties["location"])
	}
	locationProperties, ok := locationSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected failure location properties map, got %T", locationSchema["properties"])
	}
	if _, ok := locationProperties["fileName"]; !ok {
		t.Fatalf("expected failure location schema to advertise fileName")
	}
	if _, ok := locationProperties["lineNumber"]; !ok {
		t.Fatalf("expected failure location schema to advertise lineNumber")
	}
}

func TestIngestCoverageRunAuthenticatesWithHeader(t *testing.T) {
	cfg := config.Config{
		MCPServerName:       "opencoverage",
		MCPServerVersion:    "test",
		MCPTransport:        "stdio",
		MCPMaxPageSize:      100,
		MCPDefaultRunsLimit: 20,
		MCPEnableWriteTools: true,
		APIKeyHeader:        "X-API-Key",
	}

	called := false
	s := NewServer(cfg, Services{
		IngestCoverageRun: stubIngestCoverageRun(func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
			called = true
			return application.IngestCoverageRunOutput{}, nil
		}),
	}, stubAuthenticator{expected: "secret-key"})

	tool := s.GetTool("ingest_coverage_run")
	header := http.Header{}
	header.Set("X-API-Key", "secret-key")
	result, err := tool.Handler(context.Background(), mcp.CallToolRequest{
		Header: header,
		Params: mcp.CallToolParams{Name: "ingest_coverage_run", Arguments: map[string]any{
			"payload": map[string]any{
				"projectKey":           "org/repo",
				"branch":               "main",
				"commitSha":            "abc123",
				"triggerType":          "push",
				"runTimestamp":         time.Now().UTC().Format(time.RFC3339),
				"totalCoveragePercent": 80.0,
				"packages": []any{map[string]any{
					"importPath":      "github.com/org/repo/pkg",
					"coveragePercent": 80.0,
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected successful result, got %#v", result.StructuredContent)
	}
	if !called {
		t.Fatalf("expected ingest use case to be called on successful authentication")
	}
}

type stubIngestCoverageRun func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error)

func (s stubIngestCoverageRun) Execute(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
	return s(ctx, in)
}

type stubAuthenticator struct {
	expected string
}

func (s stubAuthenticator) Authenticate(ctx context.Context, apiKey string) error {
	if apiKey != s.expected {
		return fmt.Errorf("invalid key")
	}
	return nil
}

func (s stubAuthenticator) WantedAPIKey() string {
	return s.expected
}
