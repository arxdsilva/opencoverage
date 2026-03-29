package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/arxdsilva/coverage-api/internal/application"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	ingest           *application.IngestCoverageRunUseCase
	listProjects     *application.ListProjectsUseCase
	getProject       *application.GetProjectUseCase
	listRuns         *application.ListCoverageRunsUseCase
	latestComparison *application.GetLatestComparisonUseCase
	listBranches     *application.ListBranchesUseCase
}

func NewHandler(
	ingest *application.IngestCoverageRunUseCase,
	listProjects *application.ListProjectsUseCase,
	getProject *application.GetProjectUseCase,
	listRuns *application.ListCoverageRunsUseCase,
	latestComparison *application.GetLatestComparisonUseCase,
	listBranches *application.ListBranchesUseCase,
) *Handler {
	return &Handler{
		ingest:           ingest,
		listProjects:     listProjects,
		getProject:       getProject,
		listRuns:         listRuns,
		latestComparison: latestComparison,
		listBranches:     listBranches,
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
