package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/jobs"
)

type capabilitiesResponse struct {
	APIVersion          string `json:"apiVersion"`
	ReportSchemaVersion int    `json:"reportSchemaVersion"`
	MaxRecentJobs       int    `json:"maxRecentJobs"`
	ActiveJobLimit      int    `json:"activeJobLimit"`
	PollIntervalMS      int    `json:"pollIntervalMilliseconds"`
	SupportsCancel      bool   `json:"supportsCancel"`
	SupportsOpen        bool   `json:"supportsOpen"`
}

type jobsResponse struct {
	Jobs []jobSummary `json:"jobs"`
}

type jobSummary struct {
	ID           string      `json:"id"`
	Status       jobs.Status `json:"status"`
	RepoName     string      `json:"repoName"`
	CreatedAt    time.Time   `json:"createdAt"`
	StartedAt    *time.Time  `json:"startedAt"`
	FinishedAt   *time.Time  `json:"finishedAt"`
	WarningCount int         `json:"warningCount"`
}

type apiError struct {
	Error apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a *App) apiHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/capabilities", a.capabilities)
	mux.HandleFunc("/jobs", a.jobsRoute)
	mux.HandleFunc("/jobs/", a.jobRoute)
	return mux
}

func (a *App) capabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, capabilitiesResponse{
		APIVersion:          "v1",
		ReportSchemaVersion: 1,
		MaxRecentJobs:       10,
		ActiveJobLimit:      1,
		PollIntervalMS:      750,
		SupportsCancel:      true,
		SupportsOpen:        true,
	})
}

func (a *App) jobsRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items := a.Jobs.List()
		response := jobsResponse{Jobs: make([]jobSummary, 0, len(items))}
		for _, item := range items {
			response.Jobs = append(response.Jobs, summarizeJob(item))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		writeAPIError(w, http.StatusNotImplemented, "not_implemented", "job creation is not available yet")
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (a *App) jobRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/jobs/")
	if path == "" || strings.Contains(path, "/") {
		writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	writeAPIError(w, http.StatusNotImplemented, "not_implemented", "job lifecycle routes are not available yet")
}

func summarizeJob(snapshot jobs.Snapshot) jobSummary {
	return jobSummary{
		ID:           snapshot.ID,
		Status:       snapshot.Status,
		RepoName:     snapshot.RepoName,
		CreatedAt:    snapshot.CreatedAt,
		StartedAt:    snapshot.StartedAt,
		FinishedAt:   snapshot.FinishedAt,
		WarningCount: snapshot.WarningCount,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Error: apiErrorBody{Code: code, Message: message}})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method is not allowed")
}
