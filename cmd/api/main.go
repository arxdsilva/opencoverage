package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpadapter "github.com/arxdsilva/opencoverage/internal/adapters/http"
	"github.com/arxdsilva/opencoverage/internal/platform/bootstrap"
	"github.com/arxdsilva/opencoverage/internal/platform/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("startup_failed", "stage", "load_config", "error", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("startup_failed", "stage", "validate_config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	app, err := bootstrap.New(ctx, cfg, true)
	if err != nil {
		slog.Error("startup_failed", "stage", "bootstrap", "error", err)
		os.Exit(1)
	}
	defer app.Close()

	handler := httpadapter.NewHandler(
		app.IngestCoverageRun,
		app.IngestIntegrationRun,
		app.IngestE2ERun,
		app.ListProjects,
		app.GetProject,
		app.ListCoverageRuns,
		app.ListIntegrationRuns,
		app.ListE2ERuns,
		app.GetLatestComparison,
		app.GetLatestE2ECompare,
		app.GetLatestIntegrationCompare,
		app.GetIntegrationRun,
		app.GetE2ERun,
		app.GetIntegrationHeatmap,
		app.GetE2EHeatmap,
		app.ListBranches,
		app.ListContributors,
	)
	router := httpadapter.NewRouter(handler, app.Authenticator, cfg.APIKeyHeader)

	server := &http.Server{
		Addr:         cfg.ServerAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server_starting", "addr", cfg.ServerAddr)
		errCh <- server.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("shutdown_signal_received", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server_failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown_failed", "error", err)
	}
	slog.Info("server_stopped")
}
