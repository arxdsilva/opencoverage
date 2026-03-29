package http

import (
	"net/http"
	"time"

	"github.com/arxdsilva/coverage-api/internal/application"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(handler *Handler, auth application.APIKeyAuthenticator, apiKeyHeader string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(RequestLoggingMiddleware())
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(APIKeyMiddleware(auth, apiKeyHeader))

		v1.Get("/projects", handler.ListProjects)
		v1.Post("/coverage-runs", handler.IngestCoverageRun)
		v1.Get("/projects/{projectId}", handler.GetProject)
		v1.Get("/projects/{projectId}/coverage-runs", handler.ListCoverageRuns)
		v1.Get("/projects/{projectId}/coverage-runs/latest-comparison", handler.GetLatestComparison)
		v1.Get("/projects/{projectId}/branches", handler.ListBranches)
	})

	return r
}
