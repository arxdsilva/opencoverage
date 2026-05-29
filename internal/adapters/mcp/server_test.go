package mcpadapter

import (
	"context"
	"testing"

	"github.com/arxdsilva/opencoverage/internal/application"
	"github.com/arxdsilva/opencoverage/internal/platform/config"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServerRegistersReadOnlySurface(t *testing.T) {
	cfg := config.Config{
		MCPServerName:        "opencoverage",
		MCPServerVersion:     "test",
		MCPTransport:         "stdio",
		MCPMaxPageSize:       100,
		MCPDefaultRunsLimit:  20,
		MCPEnablePrompts:     true,
		MCPEnableWriteTools:  false,
	}

	s := NewServer(cfg, Services{})

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
				Items: []application.ProjectResponse{{ID: "project-1", ProjectKey: "org/repo", Name: "repo", DefaultBranch: "main", GlobalThresholdPercent: 80}},
				Pagination: application.PaginationResponse{Page: 2, PageSize: 5, TotalItems: 1, TotalPages: 1},
			}, nil
		}),
	})

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
	})

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