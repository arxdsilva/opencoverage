package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/arxdsilva/opencoverage/internal/application"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	ingest                      *application.IngestCoverageRunUseCase
	ingestIntegration           *application.IngestIntegrationRunUseCase
	ingestE2E                   *application.IngestE2ERunUseCase
	listProjects                *application.ListProjectsUseCase
	getProject                  *application.GetProjectUseCase
	listRuns                    *application.ListCoverageRunsUseCase
	listIntegrationRuns         *application.ListIntegrationRunsUseCase
	listE2ERuns                 *application.ListE2ERunsUseCase
	latestComparison            *application.GetLatestComparisonUseCase
	latestIntegrationComparison *application.GetLatestIntegrationComparisonUseCase
	latestE2EComparison         *application.GetLatestE2EComparisonUseCase
	getIntegrationRun           *application.GetIntegrationRunUseCase
	getE2ERun                   *application.GetE2ERunUseCase
	getIntegrationHeatmap       *application.GetIntegrationHeatmapUseCase
	getE2EHeatmap               *application.GetE2EHeatmapUseCase
	listBranches                *application.ListBranchesUseCase
	listContributors            *application.ListContributorsUseCase
}

func NewHandler(
	ingest *application.IngestCoverageRunUseCase,
	ingestIntegration *application.IngestIntegrationRunUseCase,
	ingestE2E *application.IngestE2ERunUseCase,
	listProjects *application.ListProjectsUseCase,
	getProject *application.GetProjectUseCase,
	listRuns *application.ListCoverageRunsUseCase,
	listIntegrationRuns *application.ListIntegrationRunsUseCase,
	listE2ERuns *application.ListE2ERunsUseCase,
	latestComparison *application.GetLatestComparisonUseCase,
	latestE2EComparison *application.GetLatestE2EComparisonUseCase,
	latestIntegrationComparison *application.GetLatestIntegrationComparisonUseCase,
	getIntegrationRun *application.GetIntegrationRunUseCase,
	getE2ERun *application.GetE2ERunUseCase,
	getIntegrationHeatmap *application.GetIntegrationHeatmapUseCase,
	getE2EHeatmap *application.GetE2EHeatmapUseCase,
	listBranches *application.ListBranchesUseCase,
	listContributors *application.ListContributorsUseCase,
) *Handler {
	return &Handler{
		ingest:                      ingest,
		ingestIntegration:           ingestIntegration,
		ingestE2E:                   ingestE2E,
		listProjects:                listProjects,
		getProject:                  getProject,
		listRuns:                    listRuns,
		listIntegrationRuns:         listIntegrationRuns,
		listE2ERuns:                 listE2ERuns,
		latestComparison:            latestComparison,
		latestIntegrationComparison: latestIntegrationComparison,
		latestE2EComparison:         latestE2EComparison,
		getIntegrationRun:           getIntegrationRun,
		getE2ERun:                   getE2ERun,
		getIntegrationHeatmap:       getIntegrationHeatmap,
		getE2EHeatmap:               getE2EHeatmap,
		listBranches:                listBranches,
		listContributors:            listContributors,
	}
}

func (h *Handler) IngestCoverageRun(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	slog.Info("operation",
		"name", "ingest_coverage_run",
		"stage", "start",
		"request_id", requestID,
	)

	var in application.IngestCoverageRunInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		slog.Warn("operation",
			"name", "ingest_coverage_run",
			"stage", "decode_failed",
			"request_id", requestID,
			"error", err,
		)
		writeError(w, http.StatusBadRequest, application.NewInvalidArgument("invalid JSON request body", nil))
		return
	}

	out, err := h.ingest.Execute(r.Context(), in)
	if err != nil {
		slog.Error("operation",
			"name", "ingest_coverage_run",
			"stage", "execute_failed",
			"request_id", requestID,
			"project_key", in.ProjectKey,
			"error", err,
		)
		writeAppError(w, err)
		return
	}

	slog.Info("operation",
		"name", "ingest_coverage_run",
		"stage", "success",
		"request_id", requestID,
		"project_id", out.Project.ID,
		"run_id", out.Run.ID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) IngestIntegrationRun(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	slog.Info("operation",
		"name", "ingest_integration_run",
		"stage", "start",
		"request_id", requestID,
	)

	var in application.IngestIntegrationRunInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		slog.Warn("operation",
			"name", "ingest_integration_run",
			"stage", "decode_failed",
			"request_id", requestID,
			"error", err,
		)
		writeError(w, http.StatusBadRequest, application.NewInvalidArgument("invalid JSON request body", nil))
		return
	}

	out, err := h.ingestIntegration.Execute(r.Context(), in)
	if err != nil {
		slog.Error("operation",
			"name", "ingest_integration_run",
			"stage", "execute_failed",
			"request_id", requestID,
			"project_key", in.ProjectKey,
			"error", err,
		)
		writeAppError(w, err)
		return
	}

	slog.Info("operation",
		"name", "ingest_integration_run",
		"stage", "success",
		"request_id", requestID,
		"project_id", out.Project.ID,
		"run_id", out.Run.ID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) IngestE2ERun(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	slog.Info("operation",
		"name", "ingest_e2e_run",
		"stage", "start",
		"request_id", requestID,
	)

	var in application.IngestE2ERunInput
	slog.Info("operation",
		"name", "ingest_e2e_run",
		"stage", "decoding_input",
		"e2e_ingest_input", in,
	)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		slog.Warn("operation",
			"name", "ingest_e2e_run",
			"stage", "decode_failed",
			"request_id", requestID,
			"error", err,
		)
		writeError(w, http.StatusBadRequest, application.NewInvalidArgument("invalid JSON request body", nil))
		return
	}

	out, err := h.ingestE2E.Execute(r.Context(), in)
	if err != nil {
		slog.Error("operation",
			"name", "ingest_e2e_run",
			"stage", "execute_failed",
			"request_id", requestID,
			"project_key", in.ProjectKey,
			"error", err,
		)
		writeAppError(w, err)
		return
	}

	slog.Info("operation",
		"name", "ingest_e2e_run",
		"stage", "success",
		"request_id", requestID,
		"project_id", out.Project.ID,
		"run_id", out.Run.ID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	slog.Info("operation", "name", "get_project", "stage", "start", "request_id", requestID, "project_id", projectID)
	out, err := h.getProject.Execute(r.Context(), projectID)
	if err != nil {
		slog.Error("operation", "name", "get_project", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}
	slog.Info("operation", "name", "get_project", "stage", "success", "request_id", requestID, "project_id", projectID, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	slog.Info("operation", "name", "list_projects", "stage", "start", "request_id", requestID, "page", page, "page_size", pageSize)
	out, err := h.listProjects.Execute(r.Context(), application.ListProjectsInput{Page: page, PageSize: pageSize})
	if err != nil {
		slog.Error("operation", "name", "list_projects", "stage", "execute_failed", "request_id", requestID, "error", err)
		writeAppError(w, err)
		return
	}
	slog.Info("operation", "name", "list_projects", "stage", "success", "request_id", requestID, "items", len(out.Items), "page", out.Pagination.Page, "page_size", out.Pagination.PageSize, "total_items", out.Pagination.TotalItems, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ListCoverageRuns(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	q := r.URL.Query()
	slog.Info("operation", "name", "list_coverage_runs", "stage", "start", "request_id", requestID, "project_id", projectID)

	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	var from *time.Time
	if fromRaw := q.Get("from"); fromRaw != "" {
		parsed, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			slog.Warn("operation", "name", "list_coverage_runs", "stage", "validation_failed", "request_id", requestID, "field", "from", "error", err)
			writeError(w, http.StatusBadRequest, application.NewInvalidArgument("from must be RFC3339", map[string]any{"field": "from"}))
			return
		}
		from = &parsed
	}

	var to *time.Time
	if toRaw := q.Get("to"); toRaw != "" {
		parsed, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			slog.Warn("operation", "name", "list_coverage_runs", "stage", "validation_failed", "request_id", requestID, "field", "to", "error", err)
			writeError(w, http.StatusBadRequest, application.NewInvalidArgument("to must be RFC3339", map[string]any{"field": "to"}))
			return
		}
		to = &parsed
	}

	out, err := h.listRuns.Execute(r.Context(), application.ListCoverageRunsInput{
		ProjectID: projectID,
		Branch:    q.Get("branch"),
		From:      from,
		To:        to,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		slog.Error("operation", "name", "list_coverage_runs", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "list_coverage_runs", "stage", "success", "request_id", requestID, "project_id", projectID, "items", len(out.Items), "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ListIntegrationRuns(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	q := r.URL.Query()
	slog.Info("operation", "name", "list_integration_runs", "stage", "start", "request_id", requestID, "project_id", projectID)

	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	var from *time.Time
	if fromRaw := q.Get("from"); fromRaw != "" {
		parsed, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			slog.Warn("operation", "name", "list_integration_runs", "stage", "validation_failed", "request_id", requestID, "field", "from", "error", err)
			writeError(w, http.StatusBadRequest, application.NewInvalidArgument("from must be RFC3339", map[string]any{"field": "from"}))
			return
		}
		from = &parsed
	}

	var to *time.Time
	if toRaw := q.Get("to"); toRaw != "" {
		parsed, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			slog.Warn("operation", "name", "list_integration_runs", "stage", "validation_failed", "request_id", requestID, "field", "to", "error", err)
			writeError(w, http.StatusBadRequest, application.NewInvalidArgument("to must be RFC3339", map[string]any{"field": "to"}))
			return
		}
		to = &parsed
	}

	out, err := h.listIntegrationRuns.Execute(r.Context(), application.ListIntegrationRunsInput{
		ProjectID:   projectID,
		Branch:      q.Get("branch"),
		Status:      q.Get("status"),
		Environment: q.Get("environment"),
		From:        from,
		To:          to,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		slog.Error("operation", "name", "list_integration_runs", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "list_integration_runs", "stage", "success", "request_id", requestID, "project_id", projectID, "items", len(out.Items), "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ListE2ERuns(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	q := r.URL.Query()
	slog.Info("operation", "name", "list_e2e_runs", "stage", "start", "request_id", requestID, "project_id", projectID)

	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	var from *time.Time
	if fromRaw := q.Get("from"); fromRaw != "" {
		parsed, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			slog.Warn("operation", "name", "list_e2e_runs", "stage", "validation_failed", "request_id", requestID, "field", "from", "error", err)
			writeError(w, http.StatusBadRequest, application.NewInvalidArgument("from must be RFC3339", map[string]any{"field": "from"}))
			return
		}
		from = &parsed
	}

	var to *time.Time
	if toRaw := q.Get("to"); toRaw != "" {
		parsed, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			slog.Warn("operation", "name", "list_e2e_runs", "stage", "validation_failed", "request_id", requestID, "field", "to", "error", err)
			writeError(w, http.StatusBadRequest, application.NewInvalidArgument("to must be RFC3339", map[string]any{"field": "to"}))
			return
		}
		to = &parsed
	}

	out, err := h.listE2ERuns.Execute(r.Context(), application.ListE2ERunsInput{
		ProjectID:   projectID,
		Branch:      q.Get("branch"),
		Status:      q.Get("status"),
		Environment: q.Get("environment"),
		From:        from,
		To:          to,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		slog.Error("operation", "name", "list_e2e_runs", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "list_e2e_runs", "stage", "success", "request_id", requestID, "project_id", projectID, "items", len(out.Items), "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetLatestComparison(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	branch := r.URL.Query().Get("branch")
	slog.Info("operation", "name", "get_latest_comparison", "stage", "start", "request_id", requestID, "project_id", projectID, "branch", branch)
	out, err := h.latestComparison.Execute(r.Context(), application.GetLatestComparisonInput{
		ProjectID: projectID,
		Branch:    branch,
	})
	if err != nil {
		slog.Error("operation", "name", "get_latest_comparison", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "branch", branch, "error", err)
		writeAppError(w, err)
		return
	}
	slog.Info("operation", "name", "get_latest_comparison", "stage", "success", "request_id", requestID, "project_id", projectID, "branch", branch, "run_id", out.Run.ID, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetLatestIntegrationComparison(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	slog.Info("operation", "name", "get_latest_integration_comparison", "stage", "start", "request_id", requestID, "project_id", projectID)

	out, err := h.latestIntegrationComparison.Execute(r.Context(), projectID)
	if err != nil {
		slog.Error("operation", "name", "get_latest_integration_comparison", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "get_latest_integration_comparison", "stage", "success", "request_id", requestID, "project_id", projectID, "run_id", out.Run.ID, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetLatestE2EComparison(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	slog.Info("operation", "name", "get_latest_e2e_comparison", "stage", "start", "request_id", requestID, "project_id", projectID)
	out, err := h.latestE2EComparison.Execute(r.Context(), projectID)
	if err != nil {
		slog.Error("operation", "name", "get_latest_e2e_comparison", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "get_latest_e2e_comparison", "stage", "success", "request_id", requestID, "project_id", projectID, "run_id", out.Run.ID, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetIntegrationRun(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	runID := chi.URLParam(r, "runId")
	slog.Info("operation", "name", "get_integration_run", "stage", "start", "request_id", requestID, "project_id", projectID, "run_id", runID)

	out, err := h.getIntegrationRun.Execute(r.Context(), projectID, runID)
	if err != nil {
		slog.Error("operation", "name", "get_integration_run", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "run_id", runID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "get_integration_run", "stage", "success", "request_id", requestID, "project_id", projectID, "run_id", runID, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetE2ERun(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	runID := chi.URLParam(r, "runId")
	slog.Info("operation", "name", "get_e2e_run", "stage", "start", "request_id", requestID, "project_id", projectID, "run_id", runID)

	out, err := h.getE2ERun.Execute(r.Context(), projectID, runID)
	if err != nil {
		slog.Error("operation", "name", "get_e2e_run", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "run_id", runID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "get_e2e_run", "stage", "success", "request_id", requestID, "project_id", projectID, "run_id", runID, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ListBranches(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	slog.Info("operation", "name", "list_branches", "stage", "start", "request_id", requestID, "project_id", projectID)
	out, err := h.listBranches.Execute(r.Context(), projectID)
	if err != nil {
		slog.Error("operation", "name", "list_branches", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}
	slog.Info("operation", "name", "list_branches", "stage", "success", "request_id", requestID, "project_id", projectID, "count", len(out.Branches), "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) ListContributors(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	projectID := chi.URLParam(r, "projectId")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	slog.Info("operation", "name", "list_contributors", "stage", "start", "request_id", requestID, "project_id", projectID, "limit", limit)

	out, err := h.listContributors.Execute(r.Context(), application.ListContributorsInput{
		ProjectID: projectID,
		Limit:     limit,
	})
	if err != nil {
		slog.Error("operation", "name", "list_contributors", "stage", "execute_failed", "request_id", requestID, "project_id", projectID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "list_contributors", "stage", "success", "request_id", requestID, "project_id", projectID, "items", len(out.Contributors), "default_branch", out.DefaultBranch, "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetIntegrationHeatmap(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	q := r.URL.Query()
	runsPerProject, _ := strconv.Atoi(q.Get("runsPerProject"))

	slog.Info("operation", "name", "get_integration_heatmap", "stage", "start", "request_id", requestID)

	out, err := h.getIntegrationHeatmap.Execute(r.Context(), application.IntegrationHeatmapInput{
		Branch:         q.Get("branch"),
		Status:         q.Get("status"),
		RunsPerProject: runsPerProject,
	})
	if err != nil {
		slog.Error("operation", "name", "get_integration_heatmap", "stage", "execute_failed", "request_id", requestID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "get_integration_heatmap", "stage", "success", "request_id", requestID, "groups", len(out.Groups), "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) GetE2EHeatmap(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := chiMiddleware.GetReqID(r.Context())
	q := r.URL.Query()
	runsPerProject, _ := strconv.Atoi(q.Get("runsPerProject"))

	slog.Info("operation", "name", "get_e2e_heatmap", "stage", "start", "request_id", requestID)

	out, err := h.getE2EHeatmap.Execute(r.Context(), application.E2EHeatmapInput{
		Branch:         q.Get("branch"),
		Status:         q.Get("status"),
		RunsPerProject: runsPerProject,
	})
	if err != nil {
		slog.Error("operation", "name", "get_e2e_heatmap", "stage", "execute_failed", "request_id", requestID, "error", err)
		writeAppError(w, err)
		return
	}

	slog.Info("operation", "name", "get_e2e_heatmap", "stage", "success", "request_id", requestID, "groups", len(out.Groups), "duration_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, out)
}

func writeAppError(w http.ResponseWriter, err error) {
	var appErr *application.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case application.CodeInvalidArgument:
			writeError(w, http.StatusBadRequest, appErr)
		case application.CodeNotFound:
			writeError(w, http.StatusNotFound, appErr)
		case application.CodeUnauthenticated:
			writeError(w, http.StatusUnauthorized, appErr)
		default:
			writeError(w, http.StatusInternalServerError, appErr)
		}
		return
	}

	writeError(w, http.StatusInternalServerError, &application.AppError{
		Code:    application.CodeInternal,
		Message: "internal server error",
	})
}

func writeError(w http.ResponseWriter, status int, appErr *application.AppError) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    appErr.Code,
			"message": appErr.Message,
			"details": appErr.Details,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
