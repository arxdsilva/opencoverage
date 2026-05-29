package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	mcpadapter "github.com/arxdsilva/opencoverage/internal/adapters/mcp"
	"github.com/arxdsilva/opencoverage/internal/platform/bootstrap"
	"github.com/arxdsilva/opencoverage/internal/platform/config"

	"github.com/mark3labs/mcp-go/server"
)

func parseMCPLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		fallthrough
	default:
		return slog.LevelInfo
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		slog.SetDefault(logger)
		slog.Error("startup_failed", "stage", "load_config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: parseMCPLogLevel(cfg.MCPLogLevel)}))
	slog.SetDefault(logger)

	if err := cfg.ValidateMCP(); err != nil {
		slog.Error("startup_failed", "stage", "validate_config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	app, err := bootstrap.New(ctx, cfg)
	if err != nil {
		slog.Error("startup_failed", "stage", "bootstrap", "error", err)
		os.Exit(1)
	}
	defer app.Close()

	mcpServer := mcpadapter.NewServer(cfg, mcpadapter.Services{
		ListProjects:                app.ListProjects,
		GetProject:                  app.GetProject,
		ListCoverageRuns:            app.ListCoverageRuns,
		GetLatestComparison:         app.GetLatestComparison,
		ListBranches:                app.ListBranches,
		ListContributors:            app.ListContributors,
		ListIntegrationRuns:         app.ListIntegrationRuns,
		GetLatestIntegrationCompare: app.GetLatestIntegrationCompare,
		GetIntegrationRun:           app.GetIntegrationRun,
		GetIntegrationHeatmap:       app.GetIntegrationHeatmap,
		IngestCoverageRun:           app.IngestCoverageRun,
		IngestIntegrationRun:        app.IngestIntegrationRun,
	}, app.Authenticator)

	slog.Info("mcp_server_starting", "name", cfg.MCPServerName, "version", cfg.MCPServerVersion, "transport", cfg.MCPTransport)
	if err := server.ServeStdio(mcpServer); err != nil {
		slog.Error("mcp_server_failed", "error", err)
		os.Exit(1)
	}
}
