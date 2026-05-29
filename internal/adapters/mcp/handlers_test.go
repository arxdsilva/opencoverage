package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"
	"github.com/arxdsilva/opencoverage/internal/platform/config"

	"github.com/mark3labs/mcp-go/mcp"
)

type stubGetProject func(ctx context.Context, projectID string) (application.ProjectResponse, error)

func (s stubGetProject) Execute(ctx context.Context, projectID string) (application.ProjectResponse, error) {
	return s(ctx, projectID)
}

type stubListBranches func(ctx context.Context, projectID string) (application.ListBranchesOutput, error)

func (s stubListBranches) Execute(ctx context.Context, projectID string) (application.ListBranchesOutput, error) {
	return s(ctx, projectID)
}

type stubListCoverageRuns func(ctx context.Context, in application.ListCoverageRunsInput) (application.ListCoverageRunsOutput, error)

func (s stubListCoverageRuns) Execute(ctx context.Context, in application.ListCoverageRunsInput) (application.ListCoverageRunsOutput, error) {
	return s(ctx, in)
}

type stubGetLatestComparison func(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error)

func (s stubGetLatestComparison) Execute(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error) {
	return s(ctx, in)
}

type stubListContributors func(ctx context.Context, in application.ListContributorsInput) (application.ListContributorsOutput, error)

func (s stubListContributors) Execute(ctx context.Context, in application.ListContributorsInput) (application.ListContributorsOutput, error) {
	return s(ctx, in)
}

type stubListIntegrationRuns func(ctx context.Context, in application.ListIntegrationRunsInput) (application.ListIntegrationRunsOutput, error)

func (s stubListIntegrationRuns) Execute(ctx context.Context, in application.ListIntegrationRunsInput) (application.ListIntegrationRunsOutput, error) {
	return s(ctx, in)
}

type stubGetLatestIntegrationComparison func(ctx context.Context, projectID string) (application.IngestIntegrationRunOutput, error)

func (s stubGetLatestIntegrationComparison) Execute(ctx context.Context, projectID string) (application.IngestIntegrationRunOutput, error) {
	return s(ctx, projectID)
}

type stubGetIntegrationRun func(ctx context.Context, projectID string, runID string) (application.IngestIntegrationRunOutput, error)

func (s stubGetIntegrationRun) Execute(ctx context.Context, projectID string, runID string) (application.IngestIntegrationRunOutput, error) {
	return s(ctx, projectID, runID)
}

type stubGetIntegrationHeatmap func(ctx context.Context, in application.IntegrationHeatmapInput) (application.GetIntegrationHeatmapOutput, error)

func (s stubGetIntegrationHeatmap) Execute(ctx context.Context, in application.IntegrationHeatmapInput) (application.GetIntegrationHeatmapOutput, error) {
	return s(ctx, in)
}

type stubIngestIntegrationRun func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error)

func (s stubIngestIntegrationRun) Execute(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
	return s(ctx, in)
}

func newTestAdapter(cfg config.Config, services Services, authenticator application.APIKeyAuthenticator) *Adapter {
	if cfg.MCPMaxPageSize == 0 {
		cfg.MCPMaxPageSize = 100
	}
	if cfg.MCPDefaultRunsLimit == 0 {
		cfg.MCPDefaultRunsLimit = 20
	}
	if cfg.APIKeyHeader == "" {
		cfg.APIKeyHeader = "X-API-Key"
	}
	return &Adapter{cfg: cfg, services: services, authenticator: authenticator}
}

func writeToolRequest(apiKey string, arguments map[string]any) mcp.CallToolRequest {
	header := http.Header{}
	if apiKey != "" {
		header.Set("X-API-Key", apiKey)
	}
	return mcp.CallToolRequest{Header: header, Params: mcp.CallToolParams{Arguments: arguments}}
}

func TestHandleListProjects(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		called := false
		a := newTestAdapter(config.Config{}, Services{
			ListProjects: stubListProjects(func(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error) {
				called = true
				if in.Page != 2 || in.PageSize != 5 {
					t.Fatalf("unexpected pagination: %#v", in)
				}
				return application.ListProjectsOutput{}, nil
			}),
		}, nil)

		result, err := a.handleListProjects(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"page": 2, "pageSize": 5}}})
		if err != nil || result.IsError || !called {
			t.Fatalf("expected successful list projects call")
		}
	})

	t.Run("missing use case", func(t *testing.T) {
		a := newTestAdapter(config.Config{}, Services{}, nil)
		result, err := a.handleListProjects(context.Background(), mcp.CallToolRequest{})
		if err != nil || !result.IsError {
			t.Fatalf("expected tool error result")
		}
	})
}

func TestHandleGetProjectAndBranches(t *testing.T) {
	a := newTestAdapter(config.Config{}, Services{
		GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
			if projectID != "p1" {
				t.Fatalf("unexpected project id %q", projectID)
			}
			return application.ProjectResponse{ID: projectID}, nil
		}),
		ListBranches: stubListBranches(func(ctx context.Context, projectID string) (application.ListBranchesOutput, error) {
			if projectID != "p1" {
				t.Fatalf("unexpected project id %q", projectID)
			}
			return application.ListBranchesOutput{}, nil
		}),
	}, nil)

	projectResult, err := a.handleGetProject(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || projectResult.IsError {
		t.Fatalf("expected get project success")
	}

	branchesResult, err := a.handleListBranches(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || branchesResult.IsError {
		t.Fatalf("expected list branches success")
	}

	invalidResult, err := a.handleGetProject(context.Background(), mcp.CallToolRequest{})
	if err != nil || !invalidResult.IsError {
		t.Fatalf("expected invalid argument error")
	}

	branchesInvalidResult, err := a.handleListBranches(context.Background(), mcp.CallToolRequest{})
	if err != nil || !branchesInvalidResult.IsError {
		t.Fatalf("expected list branches invalid argument error")
	}

	errorAdapter := newTestAdapter(config.Config{}, Services{
		ListBranches: stubListBranches(func(ctx context.Context, projectID string) (application.ListBranchesOutput, error) {
			return application.ListBranchesOutput{}, application.NewInternal("boom", nil)
		}),
	}, nil)
	errorResult, err := errorAdapter.handleListBranches(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || !errorResult.IsError {
		t.Fatalf("expected list branches downstream error")
	}
}

func TestHandleCoverageHandlers(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	calledRuns := false
	calledLatest := false
	a := newTestAdapter(config.Config{}, Services{
		ListCoverageRuns: stubListCoverageRuns(func(ctx context.Context, in application.ListCoverageRunsInput) (application.ListCoverageRunsOutput, error) {
			calledRuns = true
			if in.ProjectID != "p1" || in.Branch != "main" || in.From == nil || in.To == nil || in.From.Format(time.RFC3339) != now.Format(time.RFC3339) {
				t.Fatalf("unexpected coverage list input: %#v", in)
			}
			return application.ListCoverageRunsOutput{}, nil
		}),
		GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
			return application.ProjectResponse{ID: projectID}, nil
		}),
		GetLatestComparison: stubGetLatestComparison(func(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error) {
			calledLatest = true
			if in.ProjectID != "p1" || in.Branch != "main" {
				t.Fatalf("unexpected latest comparison input: %#v", in)
			}
			return application.LatestComparisonOutput{}, nil
		}),
	}, nil)

	runsResult, err := a.handleListCoverageRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"projectId": "p1",
		"branch":    "main",
		"from":      now.Format(time.RFC3339),
		"to":        now.Format(time.RFC3339),
		"page":      1,
		"pageSize":  10,
	}}})
	if err != nil || runsResult.IsError || !calledRuns {
		t.Fatalf("expected list coverage runs success")
	}

	latestResult, err := a.handleGetLatestCoverageComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "branch": "main"}}})
	if err != nil || latestResult.IsError || !calledLatest {
		t.Fatalf("expected get latest coverage comparison success")
	}

	invalidResult, err := a.handleListCoverageRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "from": "not-time"}}})
	if err != nil || !invalidResult.IsError {
		t.Fatalf("expected invalid argument error for from")
	}

	invalidToResult, err := a.handleListCoverageRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "to": "not-time"}}})
	if err != nil || !invalidToResult.IsError {
		t.Fatalf("expected invalid argument error for to")
	}

	missingProjectResult, err := a.handleListCoverageRuns(context.Background(), mcp.CallToolRequest{})
	if err != nil || !missingProjectResult.IsError {
		t.Fatalf("expected invalid argument error for missing project id")
	}

	errorAdapter := newTestAdapter(config.Config{}, Services{
		ListCoverageRuns: stubListCoverageRuns(func(ctx context.Context, in application.ListCoverageRunsInput) (application.ListCoverageRunsOutput, error) {
			return application.ListCoverageRunsOutput{}, application.NewInternal("boom", nil)
		}),
		GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
			return application.ProjectResponse{ID: projectID}, nil
		}),
		GetLatestComparison: stubGetLatestComparison(func(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error) {
			return application.LatestComparisonOutput{}, application.NewInternal("boom", nil)
		}),
	}, nil)

	errorRunsResult, err := errorAdapter.handleListCoverageRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || !errorRunsResult.IsError {
		t.Fatalf("expected list coverage runs downstream error")
	}

	errorLatestResult, err := errorAdapter.handleGetLatestCoverageComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || !errorLatestResult.IsError {
		t.Fatalf("expected latest coverage comparison downstream error")
	}

	getProjectErrorAdapter := newTestAdapter(config.Config{}, Services{
		GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
			return application.ProjectResponse{}, application.NewNotFound("missing", nil)
		}),
		GetLatestComparison: stubGetLatestComparison(func(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error) {
			return application.LatestComparisonOutput{}, nil
		}),
	}, nil)
	getProjectErrorResult, err := getProjectErrorAdapter.handleGetLatestCoverageComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || !getProjectErrorResult.IsError {
		t.Fatalf("expected latest coverage comparison get project error")
	}

	missingProjectLatestResult, err := errorAdapter.handleGetLatestCoverageComparison(context.Background(), mcp.CallToolRequest{})
	if err != nil || !missingProjectLatestResult.IsError {
		t.Fatalf("expected latest coverage comparison invalid argument error")
	}

	missingDepsAdapter := newTestAdapter(config.Config{}, Services{}, nil)
	missingDepsResult, err := missingDepsAdapter.handleGetLatestCoverageComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || !missingDepsResult.IsError {
		t.Fatalf("expected latest coverage comparison missing dependency error")
	}
}

func TestHandleContributorsAndIntegrationReads(t *testing.T) {
	calledContrib := false
	calledRuns := false
	calledLatest := false
	calledRun := false
	calledHeatmap := false
	a := newTestAdapter(config.Config{}, Services{
		ListContributors: stubListContributors(func(ctx context.Context, in application.ListContributorsInput) (application.ListContributorsOutput, error) {
			calledContrib = true
			if in.ProjectID != "p1" || in.Limit != 7 {
				t.Fatalf("unexpected contributors input: %#v", in)
			}
			return application.ListContributorsOutput{}, nil
		}),
		ListIntegrationRuns: stubListIntegrationRuns(func(ctx context.Context, in application.ListIntegrationRunsInput) (application.ListIntegrationRunsOutput, error) {
			calledRuns = true
			if in.ProjectID != "p1" || in.Status != "failed" || in.Environment != "stage" {
				t.Fatalf("unexpected integration list input: %#v", in)
			}
			return application.ListIntegrationRunsOutput{}, nil
		}),
		GetLatestIntegrationCompare: stubGetLatestIntegrationComparison(func(ctx context.Context, projectID string) (application.IngestIntegrationRunOutput, error) {
			calledLatest = true
			if projectID != "p1" {
				t.Fatalf("unexpected project id %q", projectID)
			}
			return application.IngestIntegrationRunOutput{}, nil
		}),
		GetIntegrationRun: stubGetIntegrationRun(func(ctx context.Context, projectID string, runID string) (application.IngestIntegrationRunOutput, error) {
			calledRun = true
			if projectID != "p1" || runID != "r1" {
				t.Fatalf("unexpected get run input: %q %q", projectID, runID)
			}
			return application.IngestIntegrationRunOutput{}, nil
		}),
		GetIntegrationHeatmap: stubGetIntegrationHeatmap(func(ctx context.Context, in application.IntegrationHeatmapInput) (application.GetIntegrationHeatmapOutput, error) {
			calledHeatmap = true
			if in.Branch != "main" || in.Status != "failed" || in.RunsPerProject != 3 {
				t.Fatalf("unexpected heatmap input: %#v", in)
			}
			return application.GetIntegrationHeatmapOutput{}, nil
		}),
	}, nil)

	result, err := a.handleListContributors(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "limit": 7}}})
	if err != nil || result.IsError || !calledContrib {
		t.Fatalf("expected list contributors success")
	}

	result, err = a.handleListIntegrationRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "status": "failed", "environment": "stage"}}})
	if err != nil || result.IsError || !calledRuns {
		t.Fatalf("expected list integration runs success")
	}

	result, err = a.handleGetLatestIntegrationComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || result.IsError || !calledLatest {
		t.Fatalf("expected latest integration comparison success")
	}

	result, err = a.handleGetIntegrationRun(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "runId": "r1"}}})
	if err != nil || result.IsError || !calledRun {
		t.Fatalf("expected get integration run success")
	}

	result, err = a.handleGetIntegrationHeatmap(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"branch": "main", "status": "failed", "runsPerProject": 3}}})
	if err != nil || result.IsError || !calledHeatmap {
		t.Fatalf("expected integration heatmap success")
	}

	result, err = a.handleGetIntegrationRun(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || !result.IsError {
		t.Fatalf("expected get integration run invalid argument result")
	}

	invalidLatestResult, err := a.handleGetLatestIntegrationComparison(context.Background(), mcp.CallToolRequest{})
	if err != nil || !invalidLatestResult.IsError {
		t.Fatalf("expected latest integration comparison invalid argument result")
	}

	invalidContribResult, err := a.handleListContributors(context.Background(), mcp.CallToolRequest{})
	if err != nil || !invalidContribResult.IsError {
		t.Fatalf("expected contributors invalid argument result")
	}

	invalidIntegrationRunsResult, err := a.handleListIntegrationRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "to": "not-time"}}})
	if err != nil || !invalidIntegrationRunsResult.IsError {
		t.Fatalf("expected integration runs invalid time result")
	}

	invalidFromIntegrationRunsResult, err := a.handleListIntegrationRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "from": "not-time"}}})
	if err != nil || !invalidFromIntegrationRunsResult.IsError {
		t.Fatalf("expected integration runs invalid from result")
	}

	missingProjectIntegrationRunsResult, err := a.handleListIntegrationRuns(context.Background(), mcp.CallToolRequest{})
	if err != nil || !missingProjectIntegrationRunsResult.IsError {
		t.Fatalf("expected integration runs invalid project id result")
	}

	errorAdapter := newTestAdapter(config.Config{}, Services{
		GetIntegrationRun: stubGetIntegrationRun(func(ctx context.Context, projectID string, runID string) (application.IngestIntegrationRunOutput, error) {
			return application.IngestIntegrationRunOutput{}, application.NewInternal("boom", nil)
		}),
		ListIntegrationRuns: stubListIntegrationRuns(func(ctx context.Context, in application.ListIntegrationRunsInput) (application.ListIntegrationRunsOutput, error) {
			return application.ListIntegrationRunsOutput{}, application.NewInternal("boom", nil)
		}),
	}, nil)

	errorRunResult, err := errorAdapter.handleGetIntegrationRun(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "runId": "r1"}}})
	if err != nil || !errorRunResult.IsError {
		t.Fatalf("expected get integration run downstream error")
	}

	errorRunsResult, err := errorAdapter.handleListIntegrationRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
	if err != nil || !errorRunsResult.IsError {
		t.Fatalf("expected list integration runs downstream error")
	}

	missingProjectRunResult, err := errorAdapter.handleGetIntegrationRun(context.Background(), mcp.CallToolRequest{})
	if err != nil || !missingProjectRunResult.IsError {
		t.Fatalf("expected get integration run invalid project id result")
	}
}

func TestHandlerMissingServiceAndErrorBranches(t *testing.T) {
	t.Run("tool handlers missing services", func(t *testing.T) {
		cases := []struct {
			name   string
			call   func(*Adapter) (*mcp.CallToolResult, error)
			wantOK bool
		}{
			{name: "get project", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleGetProject(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "list branches", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleListBranches(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "list coverage runs", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleListCoverageRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "latest coverage comparison", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleGetLatestCoverageComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "list contributors", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleListContributors(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "list integration runs", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleListIntegrationRuns(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "latest integration comparison", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleGetLatestIntegrationComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "integration run", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleGetIntegrationRun(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1", "runId": "r1"}}})
			}},
			{name: "integration heatmap", call: func(a *Adapter) (*mcp.CallToolResult, error) {
				return a.handleGetIntegrationHeatmap(context.Background(), mcp.CallToolRequest{})
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				a := newTestAdapter(config.Config{}, Services{}, nil)
				result, err := tc.call(a)
				if err != nil || !result.IsError {
					t.Fatalf("expected missing service tool error, got result=%#v err=%v", result, err)
				}
			})
		}
	})

	t.Run("downstream tool errors", func(t *testing.T) {
		a := newTestAdapter(config.Config{}, Services{
			GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
				return application.ProjectResponse{}, application.NewNotFound("missing", nil)
			}),
			ListContributors: stubListContributors(func(ctx context.Context, in application.ListContributorsInput) (application.ListContributorsOutput, error) {
				return application.ListContributorsOutput{}, application.NewInternal("boom", nil)
			}),
			GetLatestIntegrationCompare: stubGetLatestIntegrationComparison(func(ctx context.Context, projectID string) (application.IngestIntegrationRunOutput, error) {
				return application.IngestIntegrationRunOutput{}, application.NewInternal("boom", nil)
			}),
			GetIntegrationHeatmap: stubGetIntegrationHeatmap(func(ctx context.Context, in application.IntegrationHeatmapInput) (application.GetIntegrationHeatmapOutput, error) {
				return application.GetIntegrationHeatmapOutput{}, application.NewInternal("boom", nil)
			}),
		}, nil)

		cases := []struct {
			name string
			call func() (*mcp.CallToolResult, error)
		}{
			{name: "get project error", call: func() (*mcp.CallToolResult, error) {
				return a.handleGetProject(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "latest coverage comparison error", call: func() (*mcp.CallToolResult, error) {
				return a.handleGetLatestCoverageComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "list contributors error", call: func() (*mcp.CallToolResult, error) {
				return a.handleListContributors(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "latest integration comparison error", call: func() (*mcp.CallToolResult, error) {
				return a.handleGetLatestIntegrationComparison(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectId": "p1"}}})
			}},
			{name: "integration heatmap error", call: func() (*mcp.CallToolResult, error) {
				return a.handleGetIntegrationHeatmap(context.Background(), mcp.CallToolRequest{})
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := tc.call()
				if err != nil || !result.IsError {
					t.Fatalf("expected downstream tool error, got result=%#v err=%v", result, err)
				}
			})
		}
	})

	t.Run("resource handlers missing services", func(t *testing.T) {
		a := newTestAdapter(config.Config{}, Services{}, nil)
		cases := []struct {
			name string
			call func() error
		}{
			{name: "projects", call: func() error {
				_, err := a.readProjectsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: resourceProjects}})
				return err
			}},
			{name: "integration heatmap", call: func() error {
				_, err := a.readIntegrationHeatmapResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: resourceIntegrationHeatmap}})
				return err
			}},
			{name: "project", call: func() error {
				_, err := a.readProjectResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1"}})
				return err
			}},
			{name: "project coverage", call: func() error {
				_, err := a.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/coverage/latest"}})
				return err
			}},
			{name: "project integration", call: func() error {
				_, err := a.readProjectIntegrationResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/integration/latest"}})
				return err
			}},
			{name: "project contributors", call: func() error {
				_, err := a.readProjectContributorsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/contributors"}})
				return err
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if err := tc.call(); err == nil {
					t.Fatalf("expected missing service resource error")
				}
			})
		}
	})

	t.Run("resource downstream errors", func(t *testing.T) {
		a := newTestAdapter(config.Config{}, Services{
			GetIntegrationHeatmap: stubGetIntegrationHeatmap(func(ctx context.Context, in application.IntegrationHeatmapInput) (application.GetIntegrationHeatmapOutput, error) {
				return application.GetIntegrationHeatmapOutput{}, application.NewInternal("boom", nil)
			}),
			GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
				return application.ProjectResponse{}, application.NewNotFound("missing", nil)
			}),
			ListContributors: stubListContributors(func(ctx context.Context, in application.ListContributorsInput) (application.ListContributorsOutput, error) {
				return application.ListContributorsOutput{}, application.NewInternal("boom", nil)
			}),
			GetLatestIntegrationCompare: stubGetLatestIntegrationComparison(func(ctx context.Context, projectID string) (application.IngestIntegrationRunOutput, error) {
				return application.IngestIntegrationRunOutput{}, application.NewInternal("boom", nil)
			}),
		}, nil)

		cases := []struct {
			name string
			call func() error
		}{
			{name: "integration heatmap error", call: func() error {
				_, err := a.readIntegrationHeatmapResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: resourceIntegrationHeatmap}})
				return err
			}},
			{name: "project error", call: func() error {
				_, err := a.readProjectResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1"}})
				return err
			}},
			{name: "project coverage error", call: func() error {
				_, err := a.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/coverage/latest"}})
				return err
			}},
			{name: "project integration error", call: func() error {
				_, err := a.readProjectIntegrationResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/integration/latest"}})
				return err
			}},
			{name: "project contributors error", call: func() error {
				_, err := a.readProjectContributorsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/contributors"}})
				return err
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if err := tc.call(); err == nil {
					t.Fatalf("expected downstream resource error")
				}
			})
		}
	})
}

func TestHandleIngestHandlers(t *testing.T) {
	t.Run("coverage ingest disabled", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: false}, Services{}, nil)
		result, err := a.handleIngestCoverageRun(context.Background(), mcp.CallToolRequest{})
		if err != nil || !result.IsError {
			t.Fatalf("expected unauthorized error when tool disabled")
		}
	})

	t.Run("coverage ingest success from payload", func(t *testing.T) {
		called := false
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true, APIKeyHeader: "X-API-Key"}, Services{
			IngestCoverageRun: stubIngestCoverageRun(func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
				called = true
				if in.ProjectKey != "org/repo" {
					t.Fatalf("unexpected project key: %q", in.ProjectKey)
				}
				return application.IngestCoverageRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestCoverageRun(context.Background(), writeToolRequest("secret", map[string]any{
			"payload": map[string]any{
				"projectKey":           "org/repo",
				"branch":               "main",
				"commitSha":            "abc123",
				"triggerType":          "push",
				"runTimestamp":         time.Now().UTC().Format(time.RFC3339),
				"totalCoveragePercent": 88.5,
				"packages": []any{map[string]any{
					"importPath":      "github.com/org/repo/pkg",
					"coveragePercent": 88.5,
				}},
			},
		}))
		if err != nil || result.IsError || !called {
			t.Fatalf("expected successful ingest coverage run")
		}
	})

	t.Run("coverage ingest invalid payload", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true, APIKeyHeader: "X-API-Key"}, Services{
			IngestCoverageRun: stubIngestCoverageRun(func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
				return application.IngestCoverageRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestCoverageRun(context.Background(), writeToolRequest("secret", map[string]any{
			"payload": map[string]any{"projectKey": func() {}},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected invalid coverage ingest payload")
		}
	})

	t.Run("coverage ingest downstream error", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true, APIKeyHeader: "X-API-Key"}, Services{
			IngestCoverageRun: stubIngestCoverageRun(func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
				return application.IngestCoverageRunOutput{}, application.NewInternal("boom", nil)
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestCoverageRun(context.Background(), writeToolRequest("secret", map[string]any{
			"payload": map[string]any{
				"projectKey":           "org/repo",
				"branch":               "main",
				"commitSha":            "abc123",
				"triggerType":          "push",
				"runTimestamp":         time.Now().UTC().Format(time.RFC3339),
				"totalCoveragePercent": 88.5,
				"packages": []any{map[string]any{
					"importPath":      "github.com/org/repo/pkg",
					"coveragePercent": 88.5,
				}},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected coverage ingest downstream error")
		}
	})

	t.Run("coverage ingest validation failure", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true, APIKeyHeader: "X-API-Key"}, Services{
			IngestCoverageRun: stubIngestCoverageRun(func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
				t.Fatalf("ingest use case should not run on validation failure")
				return application.IngestCoverageRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestCoverageRun(context.Background(), writeToolRequest("secret", map[string]any{
			"payload": map[string]any{
				"projectKey":           "org/repo",
				"branch":               "main",
				"commitSha":            "abc123",
				"triggerType":          "unknown",
				"runTimestamp":         time.Now().UTC().Format(time.RFC3339),
				"totalCoveragePercent": 88.5,
				"packages": []any{map[string]any{
					"importPath":      "github.com/org/repo/pkg",
					"coveragePercent": 88.5,
				}},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected coverage ingest validation error")
		}
	})

	t.Run("coverage ingest rejects duplicate packages", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true, APIKeyHeader: "X-API-Key"}, Services{
			IngestCoverageRun: stubIngestCoverageRun(func(ctx context.Context, in application.IngestCoverageRunInput) (application.IngestCoverageRunOutput, error) {
				t.Fatalf("ingest use case should not run on validation failure")
				return application.IngestCoverageRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestCoverageRun(context.Background(), writeToolRequest("secret", map[string]any{
			"payload": map[string]any{
				"projectKey":           "org/repo",
				"branch":               "main",
				"commitSha":            "abc123",
				"triggerType":          "push",
				"runTimestamp":         time.Now().UTC().Format(time.RFC3339),
				"totalCoveragePercent": 88.5,
				"packages": []any{
					map[string]any{"importPath": "github.com/org/repo/pkg", "coveragePercent": 88.5},
					map[string]any{"importPath": "github.com/org/repo/pkg", "coveragePercent": 80.0},
				},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected duplicate package validation error")
		}
	})

	t.Run("integration ingest success from arguments", func(t *testing.T) {
		called := false
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true, APIKeyHeader: "X-API-Key"}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				called = true
				if in.ProjectKey != "org/repo" {
					t.Fatalf("unexpected project key: %q", in.ProjectKey)
				}
				return application.IngestIntegrationRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{
			"projectKey":   "org/repo",
			"branch":       "main",
			"commitSha":    "abc123",
			"triggerType":  "push",
			"runTimestamp": time.Now().UTC().Format(time.RFC3339),
			"ginkgoReport": map[string]any{
				"suiteDescription": "suite",
				"suitePath":        "integration/suite",
				"specReports": []any{map[string]any{
					"leafNodeText": "spec",
					"state":        "passed",
					"runTime":      0.1,
				}},
			},
		}))
		if err != nil || result.IsError || !called {
			t.Fatalf("expected successful ingest integration run")
		}
	})

	t.Run("authentication failure", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				return application.IngestIntegrationRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("wrong", map[string]any{}))
		if err != nil || !result.IsError {
			t.Fatalf("expected unauthorized result")
		}
	})

	t.Run("integration ingest invalid payload", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				return application.IngestIntegrationRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{"payload": map[string]any{"projectKey": func() {}}}))
		if err != nil || !result.IsError {
			t.Fatalf("expected invalid integration ingest payload")
		}
	})

	t.Run("integration ingest missing service", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{}, stubAuthenticator{expected: "secret"})
		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{}))
		if err != nil || !result.IsError {
			t.Fatalf("expected missing integration ingest service error")
		}
	})

	t.Run("integration ingest downstream error", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				return application.IngestIntegrationRunOutput{}, application.NewInternal("boom", nil)
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{
			"projectKey":   "org/repo",
			"branch":       "main",
			"commitSha":    "abc123",
			"triggerType":  "push",
			"runTimestamp": time.Now().UTC().Format(time.RFC3339),
			"ginkgoReport": map[string]any{
				"suiteDescription": "suite",
				"suitePath":        "integration/suite",
				"specReports": []any{map[string]any{
					"leafNodeText": "spec",
					"state":        "passed",
					"runTime":      0.1,
				}},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected integration ingest downstream error")
		}
	})

	t.Run("integration ingest validation failure", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				t.Fatalf("ingest integration use case should not run on validation failure")
				return application.IngestIntegrationRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{
			"projectKey":   "org/repo",
			"branch":       "main",
			"commitSha":    "abc123",
			"triggerType":  "push",
			"runTimestamp": time.Now().UTC().Format(time.RFC3339),
			"ginkgoReport": map[string]any{
				"suiteDescription": "suite",
				"suitePath":        "integration/suite",
				"specReports": []any{map[string]any{
					"leafNodeText": "spec",
					"state":        "failed",
					"runTime":      0.1,
				}},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected integration ingest validation error")
		}
	})

	t.Run("integration ingest failed spec requires failure location", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				t.Fatalf("ingest integration use case should not run on validation failure")
				return application.IngestIntegrationRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{
			"projectKey":   "org/repo",
			"branch":       "main",
			"commitSha":    "abc123",
			"triggerType":  "push",
			"runTimestamp": time.Now().UTC().Format(time.RFC3339),
			"ginkgoReport": map[string]any{
				"suiteDescription": "suite",
				"suitePath":        "integration/suite",
				"specReports": []any{map[string]any{
					"leafNodeText": "spec",
					"state":        "failed",
					"runTime":      0.1,
					"failure": map[string]any{
						"message": "boom",
					},
				}},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected failure location validation error")
		}
	})

	t.Run("integration ingest failed spec rejects bad failure location fields", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				t.Fatalf("ingest integration use case should not run on validation failure")
				return application.IngestIntegrationRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{
			"projectKey":   "org/repo",
			"branch":       "main",
			"commitSha":    "abc123",
			"triggerType":  "push",
			"runTimestamp": time.Now().UTC().Format(time.RFC3339),
			"ginkgoReport": map[string]any{
				"suiteDescription": "suite",
				"suitePath":        "integration/suite",
				"specReports": []any{map[string]any{
					"leafNodeText": "spec",
					"state":        "failed",
					"runTime":      0.1,
					"failure": map[string]any{
						"message": "boom",
						"location": map[string]any{
							"fileName":   "   ",
							"lineNumber": -1,
						},
					},
				}},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected bad failure location validation error")
		}
	})

	t.Run("integration ingest failed spec rejects negative failure location line number", func(t *testing.T) {
		a := newTestAdapter(config.Config{MCPEnableWriteTools: true}, Services{
			IngestIntegrationRun: stubIngestIntegrationRun(func(ctx context.Context, in application.IngestIntegrationRunInput) (application.IngestIntegrationRunOutput, error) {
				t.Fatalf("ingest integration use case should not run on validation failure")
				return application.IngestIntegrationRunOutput{}, nil
			}),
		}, stubAuthenticator{expected: "secret"})

		result, err := a.handleIngestIntegrationRun(context.Background(), writeToolRequest("secret", map[string]any{
			"projectKey":   "org/repo",
			"branch":       "main",
			"commitSha":    "abc123",
			"triggerType":  "push",
			"runTimestamp": time.Now().UTC().Format(time.RFC3339),
			"ginkgoReport": map[string]any{
				"suiteDescription": "suite",
				"suitePath":        "integration/suite",
				"specReports": []any{map[string]any{
					"leafNodeText": "spec",
					"state":        "failed",
					"runTime":      0.1,
					"failure": map[string]any{
						"message": "boom",
						"location": map[string]any{
							"fileName":   "spec_test.go",
							"lineNumber": -1,
						},
					},
				}},
			},
		}))
		if err != nil || !result.IsError {
			t.Fatalf("expected negative line number validation error")
		}
	})
}

func TestReadResourceHandlers(t *testing.T) {
	a := newTestAdapter(config.Config{}, Services{
		ListProjects: stubListProjects(func(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error) {
			return application.ListProjectsOutput{}, nil
		}),
		GetIntegrationHeatmap: stubGetIntegrationHeatmap(func(ctx context.Context, in application.IntegrationHeatmapInput) (application.GetIntegrationHeatmapOutput, error) {
			return application.GetIntegrationHeatmapOutput{}, nil
		}),
		GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
			return application.ProjectResponse{ID: projectID}, nil
		}),
		GetLatestComparison: stubGetLatestComparison(func(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error) {
			return application.LatestComparisonOutput{}, nil
		}),
		GetLatestIntegrationCompare: stubGetLatestIntegrationComparison(func(ctx context.Context, projectID string) (application.IngestIntegrationRunOutput, error) {
			return application.IngestIntegrationRunOutput{}, nil
		}),
		ListContributors: stubListContributors(func(ctx context.Context, in application.ListContributorsInput) (application.ListContributorsOutput, error) {
			return application.ListContributorsOutput{}, nil
		}),
	}, nil)

	if _, err := a.readProjectsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: resourceProjects}}); err != nil {
		t.Fatalf("expected projects resource success: %v", err)
	}
	if _, err := a.readIntegrationHeatmapResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: resourceIntegrationHeatmap}}); err != nil {
		t.Fatalf("expected integration heatmap resource success: %v", err)
	}
	if _, err := a.readProjectResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1"}}); err != nil {
		t.Fatalf("expected project resource success: %v", err)
	}
	if _, err := a.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/coverage/latest"}}); err != nil {
		t.Fatalf("expected project coverage resource success: %v", err)
	}
	if _, err := a.readProjectIntegrationResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/integration/latest"}}); err != nil {
		t.Fatalf("expected project integration resource success: %v", err)
	}
	if _, err := a.readProjectContributorsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/contributors"}}); err != nil {
		t.Fatalf("expected project contributors resource success: %v", err)
	}
	if _, err := a.readProjectResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "://bad uri"}}); err == nil {
		t.Fatalf("expected invalid project resource uri error")
	}
	if _, err := a.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "://bad uri"}}); err == nil {
		t.Fatalf("expected invalid project coverage resource uri error")
	}
	if _, err := a.readProjectIntegrationResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "://bad uri"}}); err == nil {
		t.Fatalf("expected invalid project integration resource uri error")
	}
	if _, err := a.readProjectContributorsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "://bad uri"}}); err == nil {
		t.Fatalf("expected invalid project contributors resource uri error")
	}

	errorCoverageAdapter := newTestAdapter(config.Config{}, Services{
		GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
			return application.ProjectResponse{ID: projectID}, nil
		}),
		GetLatestComparison: stubGetLatestComparison(func(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error) {
			return application.LatestComparisonOutput{}, application.NewInternal("boom", nil)
		}),
	}, nil)
	if _, err := errorCoverageAdapter.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/coverage/latest"}}); err == nil {
		t.Fatalf("expected project coverage downstream comparison error")
	}

	missingCoverageDepsAdapter := newTestAdapter(config.Config{}, Services{}, nil)
	if _, err := missingCoverageDepsAdapter.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/coverage/latest"}}); err == nil {
		t.Fatalf("expected project coverage missing dependency error")
	}

	getProjectCoverageErrorAdapter := newTestAdapter(config.Config{}, Services{
		GetProject: stubGetProject(func(ctx context.Context, projectID string) (application.ProjectResponse, error) {
			return application.ProjectResponse{}, application.NewNotFound("missing", nil)
		}),
		GetLatestComparison: stubGetLatestComparison(func(ctx context.Context, in application.GetLatestComparisonInput) (application.LatestComparisonOutput, error) {
			return application.LatestComparisonOutput{}, nil
		}),
	}, nil)
	if _, err := getProjectCoverageErrorAdapter.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/coverage/latest"}}); err == nil {
		t.Fatalf("expected project coverage get project error")
	}

	if _, err := a.readProjectResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/extra"}}); err == nil {
		t.Fatalf("expected not found error for invalid project resource path")
	}
	if _, err := a.readProjectCoverageResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/coverage/current"}}); err == nil {
		t.Fatalf("expected not found error for invalid coverage resource path")
	}
	if _, err := a.readProjectIntegrationResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/integration/current"}}); err == nil {
		t.Fatalf("expected not found error for invalid integration resource path")
	}
	if _, err := a.readProjectContributorsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "opencoverage://projects/p1/author"}}); err == nil {
		t.Fatalf("expected not found error for invalid contributors resource path")
	}
}

func TestPromptHandlers(t *testing.T) {
	a := newTestAdapter(config.Config{}, Services{}, nil)

	result, err := a.getSummarizeProjectHealthPrompt(context.Background(), mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{"projectId": "p1", "branch": "main"}}})
	if err != nil {
		t.Fatalf("expected summarize prompt success: %v", err)
	}
	encoded, _ := json.Marshal(result)
	if !strings.Contains(string(encoded), "p1") {
		t.Fatalf("expected prompt payload to include project id")
	}

	result, err = a.getInvestigateCoverageRegressionPrompt(context.Background(), mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{"projectId": "p1"}}})
	if err != nil {
		t.Fatalf("expected coverage prompt success: %v", err)
	}
	encoded, _ = json.Marshal(result)
	if !strings.Contains(string(encoded), "regression") {
		t.Fatalf("expected coverage prompt payload")
	}

	result, err = a.getInvestigateCoverageRegressionPrompt(context.Background(), mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{"projectId": "p1", "branch": "main"}}})
	if err != nil {
		t.Fatalf("expected coverage prompt success with branch: %v", err)
	}
	encoded, _ = json.Marshal(result)
	if !strings.Contains(string(encoded), "main") {
		t.Fatalf("expected coverage prompt payload to include branch")
	}

	result, err = a.getInvestigateIntegrationFailuresPrompt(context.Background(), mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{"projectId": "p1", "environment": "stage"}}})
	if err != nil {
		t.Fatalf("expected integration failures prompt success: %v", err)
	}
	encoded, _ = json.Marshal(result)
	if !strings.Contains(string(encoded), "stage") {
		t.Fatalf("expected integration prompt payload to include environment")
	}

	if result, err := a.getSummarizeProjectHealthPrompt(context.Background(), mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{"projectId": ""}}}); err == nil || result != nil {
		t.Fatalf("expected summarize project health prompt to reject empty projectId")
	}
	if result, err := a.getInvestigateCoverageRegressionPrompt(context.Background(), mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{"projectId": ""}}}); err == nil || result != nil {
		t.Fatalf("expected investigate coverage regression prompt to reject empty projectId")
	}
	if result, err := a.getInvestigateIntegrationFailuresPrompt(context.Background(), mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{"projectId": ""}}}); err == nil || result != nil {
		t.Fatalf("expected investigate integration failures prompt to reject empty projectId")
	}
}

func TestHandlerHelpers(t *testing.T) {
	a := newTestAdapter(config.Config{MCPMaxPageSize: 50, MCPDefaultRunsLimit: 15}, Services{}, nil)

	if got := a.normalizePage(0); got != 1 {
		t.Fatalf("expected normalized page to be 1, got %d", got)
	}
	if got := a.normalizePage(3); got != 3 {
		t.Fatalf("expected unchanged page, got %d", got)
	}

	if got := a.normalizePageSize(0); got != defaultListPageSize {
		t.Fatalf("expected default page size, got %d", got)
	}
	if got := a.normalizePageSize(200); got != 50 {
		t.Fatalf("expected capped page size, got %d", got)
	}
	if got := a.normalizePageSize(10); got != 10 {
		t.Fatalf("expected unchanged page size, got %d", got)
	}
	if got := a.pageSizeLimit(); got != 50 {
		t.Fatalf("expected configured page size limit, got %d", got)
	}
	if got := a.defaultRunsLimit(); got != 15 {
		t.Fatalf("expected configured runs limit, got %d", got)
	}
	if got := a.normalizeContributorLimit(0); got != defaultContributorLimit {
		t.Fatalf("expected default contributor limit, got %d", got)
	}
	if got := a.normalizeContributorLimit(200); got != maxContributorLimit {
		t.Fatalf("expected capped contributor limit, got %d", got)
	}
	if got := a.normalizeContributorLimit(7); got != 7 {
		t.Fatalf("expected unchanged contributor limit, got %d", got)
	}
	if got := a.normalizeRunsPerProject(0); got != 15 {
		t.Fatalf("expected default runs per project, got %d", got)
	}
	if got := a.normalizeRunsPerProject(200); got != maxRunsPerProject {
		t.Fatalf("expected capped runs per project, got %d", got)
	}
	if got := a.normalizeRunsPerProject(8); got != 8 {
		t.Fatalf("expected unchanged runs per project, got %d", got)
	}

	fallbackAdapter := &Adapter{cfg: config.Config{MCPDefaultRunsLimit: 0}}
	if got := fallbackAdapter.defaultRunsLimit(); got != 10 {
		t.Fatalf("expected fallback runs limit, got %d", got)
	}
	if got := fallbackAdapter.normalizeRunsPerProject(0); got != 10 {
		t.Fatalf("expected fallback normalized runs per project, got %d", got)
	}

	cappedDefaultAdapter := &Adapter{cfg: config.Config{MCPDefaultRunsLimit: 200}}
	if got := cappedDefaultAdapter.normalizeRunsPerProject(0); got != maxRunsPerProject {
		t.Fatalf("expected capped default runs per project, got %d", got)
	}

	overLimitAdapter := newTestAdapter(config.Config{MCPMaxPageSize: 500, MCPDefaultRunsLimit: 15}, Services{}, nil)
	if got := overLimitAdapter.pageSizeLimit(); got != applicationMaxPageSize {
		t.Fatalf("expected app page size limit, got %d", got)
	}
	if got := overLimitAdapter.normalizePageSize(500); got != applicationMaxPageSize {
		t.Fatalf("expected capped page size, got %d", got)
	}

	t.Run("authenticate missing authenticator", func(t *testing.T) {
		authAdapter := newTestAdapter(config.Config{APIKeyHeader: "X-API-Key"}, Services{}, nil)
		err := authAdapter.authenticateWriteRequest(context.Background(), mcp.CallToolRequest{})
		if err == nil {
			t.Fatalf("expected auth configuration error")
		}
	})

	t.Run("authenticate with header", func(t *testing.T) {
		authAdapter := newTestAdapter(config.Config{APIKeyHeader: "X-API-Key"}, Services{}, stubAuthenticator{expected: "secret"})
		headers := http.Header{}
		headers.Set("X-API-Key", "secret")
		err := authAdapter.authenticateWriteRequest(context.Background(), mcp.CallToolRequest{Header: headers})
		if err != nil {
			t.Fatalf("expected successful auth, got %v", err)
		}
	})

	t.Run("authenticate missing header", func(t *testing.T) {
		authAdapter := newTestAdapter(config.Config{APIKeyHeader: "X-API-Key"}, Services{}, stubAuthenticator{expected: "secret"})
		err := authAdapter.authenticateWriteRequest(context.Background(), mcp.CallToolRequest{})
		if err == nil {
			t.Fatalf("expected missing-header auth failure")
		}
	})

	t.Run("bind payload and args", func(t *testing.T) {
		type bindTarget struct {
			ProjectKey string `json:"projectKey"`
		}
		var target bindTarget
		err := bindPayloadOrArguments(mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"payload": map[string]any{"projectKey": "org/repo"}}}}, &target)
		if err != nil || target.ProjectKey != "org/repo" {
			t.Fatalf("expected payload bind success, got target=%#v err=%v", target, err)
		}

		target = bindTarget{}
		err = bindPayloadOrArguments(mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"projectKey": "org/repo"}}}, &target)
		if err != nil || target.ProjectKey != "org/repo" {
			t.Fatalf("expected argument bind success, got target=%#v err=%v", target, err)
		}
	})

	t.Run("bind payload encode failure", func(t *testing.T) {
		type bindTarget struct {
			ProjectKey string `json:"projectKey"`
		}
		var target bindTarget
		err := bindPayloadOrArguments(mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"payload": map[string]any{"bad": func() {}}}}}, &target)
		if err == nil {
			t.Fatalf("expected marshal failure")
		}
	})

	t.Run("resource protocol mapping", func(t *testing.T) {
		badAdapter := newTestAdapter(config.Config{}, Services{
			ListProjects: stubListProjects(func(ctx context.Context, in application.ListProjectsInput) (application.ListProjectsOutput, error) {
				return application.ListProjectsOutput{}, errors.New("boom")
			}),
		}, nil)
		_, err := badAdapter.readProjectsResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: resourceProjects}})
		if err == nil {
			t.Fatalf("expected protocol error")
		}
	})
}

type logCaptureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *logCaptureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *logCaptureHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record.Clone())
	return nil
}

func (h *logCaptureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *logCaptureHandler) WithGroup(_ string) slog.Handler { return h }

func (h *logCaptureHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, 0, len(h.records))
	for _, record := range h.records {
		out = append(out, record.Message)
	}
	return out
}

func (h *logCaptureHandler) hasDurationFor(message string, phase string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, record := range h.records {
		if record.Message != message {
			continue
		}
		hasPhase := false
		hasDuration := false
		record.Attrs(func(attr slog.Attr) bool {
			if attr.Key == "phase" && attr.Value.String() == phase {
				hasPhase = true
			}
			if attr.Key == "duration_ms" {
				hasDuration = true
			}
			return true
		})
		if hasPhase && hasDuration {
			return true
		}
	}
	return false
}

func TestOperationLoggingWrappers(t *testing.T) {
	capture := &logCaptureHandler{}
	logger := slog.New(capture)
	previous := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(previous)

	a := newTestAdapter(config.Config{}, Services{}, nil)

	toolHandler := a.withToolLogging("test_tool", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return toolJSONResult(map[string]string{"ok": "true"})
	})
	if _, err := toolHandler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("expected tool wrapper success: %v", err)
	}

	resourceHandler := a.withResourceLogging("test_resource", func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{}, nil
	})
	if _, err := resourceHandler(context.Background(), mcp.ReadResourceRequest{}); err != nil {
		t.Fatalf("expected resource wrapper success: %v", err)
	}

	promptHandler := a.withPromptLogging("test_prompt", func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult("test", nil), nil
	})
	if _, err := promptHandler(context.Background(), mcp.GetPromptRequest{}); err != nil {
		t.Fatalf("expected prompt wrapper success: %v", err)
	}

	errorToolHandler := a.withToolLogging("test_tool_error", func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return toolErrorResult(application.NewInternal("boom", nil)), nil
	})
	if _, err := errorToolHandler(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("expected wrapped tool error result without protocol error: %v", err)
	}

	messages := strings.Join(capture.messages(), "\n")
	if !strings.Contains(messages, "mcp_tool_handled") || !strings.Contains(messages, "mcp_resource_handled") || !strings.Contains(messages, "mcp_prompt_handled") {
		t.Fatalf("expected wrapper log messages, got %q", messages)
	}
	if !capture.hasDurationFor("mcp_tool_handled", "success") || !capture.hasDurationFor("mcp_tool_handled", "error") {
		t.Fatalf("expected duration_ms on tool success/error logs")
	}
	if !capture.hasDurationFor("mcp_resource_handled", "success") {
		t.Fatalf("expected duration_ms on resource success log")
	}
	if !capture.hasDurationFor("mcp_prompt_handled", "success") {
		t.Fatalf("expected duration_ms on prompt success log")
	}
}

func TestFormatErrorSummary(t *testing.T) {
	t.Run("uses payload values", func(t *testing.T) {
		summary := formatErrorSummary(map[string]any{"code": "not_found", "message": "missing"})
		if summary != "not_found: missing" {
			t.Fatalf("unexpected summary %q", summary)
		}
	})

	t.Run("falls back for unexpected types", func(t *testing.T) {
		summary := formatErrorSummary(map[string]any{"code": 123, "message": true})
		if summary != "internal: internal server error" {
			t.Fatalf("unexpected fallback summary %q", summary)
		}
	})
}

func TestErrorPayloadForNonAppErrorIsGeneric(t *testing.T) {
	payload := errorPayload(errors.New("sql: password authentication failed for user app"))

	if payload["code"] != "internal" {
		t.Fatalf("expected internal code, got %#v", payload["code"])
	}
	if payload["message"] != "internal server error" {
		t.Fatalf("expected generic internal message, got %#v", payload["message"])
	}
}
