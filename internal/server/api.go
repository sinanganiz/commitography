package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/analysis"
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

type jobStatusResponse struct {
	ID                  string                  `json:"id"`
	Status              jobs.Status             `json:"status"`
	RepoName            string                  `json:"repoName"`
	RepoPath            string                  `json:"repoPath"`
	CreatedAt           time.Time               `json:"createdAt"`
	StartedAt           *time.Time              `json:"startedAt"`
	FinishedAt          *time.Time              `json:"finishedAt"`
	ElapsedMilliseconds int64                   `json:"elapsedMilliseconds"`
	Progress            *analysis.ProgressEvent `json:"progress"`
	Warnings            []string                `json:"warnings"`
	Error               *jobs.Failure           `json:"error"`
}

type createJobRequest struct {
	RepoPath string           `json:"repoPath"`
	Options  createJobOptions `json:"options"`
}

type createJobOptions struct {
	NoBlame      bool   `json:"noBlame"`
	PerAuthor    bool   `json:"perAuthor"`
	Anonymize    bool   `json:"anonymize"`
	AllowShallow bool   `json:"allowShallow"`
	CountMerges  bool   `json:"countMerges"`
	Since        string `json:"since"`
	Until        string `json:"until"`
}

type createJobResponse struct {
	ID     string      `json:"id"`
	Status jobs.Status `json:"status"`
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
		a.createJob(w, r)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (a *App) createJob(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request createJobRequest
	if err := decoder.Decode(&request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "request body must contain one JSON object")
		return
	}
	if strings.TrimSpace(request.RepoPath) == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_repository_path", "repoPath is required")
		return
	}
	canonicalPath, err := a.validateRepositoryPath(request.RepoPath, request.Options.AllowShallow)
	if err != nil {
		var pathErr *pathValidationError
		if errors.As(err, &pathErr) {
			status := http.StatusBadRequest
			if pathErr.Forbidden {
				status = http.StatusForbidden
			}
			writeAPIError(w, status, pathErr.Code, pathErr.Message)
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid_repository", err.Error())
		return
	}

	// countMerges mirrors the CLI flag: true overrides the repository config,
	// false leaves the repository's count_merges setting in charge.
	options := analysis.Options{
		RepoPath:         canonicalPath,
		NoBlame:          request.Options.NoBlame,
		PerAuthor:        request.Options.PerAuthor,
		Anonymize:        request.Options.Anonymize,
		AllowShallow:     request.Options.AllowShallow,
		CountMerges:      request.Options.CountMerges,
		CountMergesSet:   request.Options.CountMerges,
		Since:            request.Options.Since,
		Until:            request.Options.Until,
		CheckConsistency: true,
	}
	snapshot, err := a.Jobs.Start(canonicalPath, options)
	if err != nil {
		if errors.Is(err, jobs.ErrActiveJob) {
			writeAPIError(w, http.StatusConflict, "active_job", "another analysis job is already active")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "job_start_failed", "could not start the analysis job")
		return
	}
	writeJSON(w, http.StatusAccepted, createJobResponse{ID: snapshot.ID, Status: snapshot.Status})
}

func (a *App) jobRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/jobs/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			snapshot, err := a.Jobs.Get(id)
			if err != nil {
				writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
				return
			}
			writeJSON(w, http.StatusOK, statusResponse(snapshot))
		case http.MethodDelete:
			if err := a.Jobs.Delete(id); err != nil {
				if errors.Is(err, jobs.ErrJobNotFound) {
					writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
				} else {
					writeAPIError(w, http.StatusConflict, "invalid_job_state", "active jobs must be cancelled before deletion")
				}
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodDelete)
		}
		return
	}
	if len(parts) != 2 {
		writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
		return
	}
	switch parts[1] {
	case "report":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		report, err := a.Jobs.Report(id)
		if err != nil {
			if errors.Is(err, jobs.ErrJobNotFound) {
				writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
			} else {
				writeAPIError(w, http.StatusConflict, "report_not_ready", "job has no current report")
			}
			return
		}
		writeJSON(w, http.StatusOK, report)
	case "cancel":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		// Cancellation is idempotent: repeating it for a job that has already
		// stopped reports the terminal state instead of a conflict.
		if snapshot, err := a.Jobs.Get(id); err == nil && snapshot.Status == jobs.StatusCancelled {
			writeJSON(w, http.StatusOK, statusResponse(snapshot))
			return
		}
		if err := a.Jobs.Cancel(id); err != nil {
			if errors.Is(err, jobs.ErrJobNotFound) {
				writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
			} else {
				writeAPIError(w, http.StatusConflict, "invalid_job_state", "job cannot be cancelled in its current state")
			}
			return
		}
		snapshot, _ := a.Jobs.Get(id)
		writeJSON(w, http.StatusAccepted, statusResponse(snapshot))
	case "":
		writeAPIError(w, http.StatusNotFound, "not_found", "job not found")
	default:
		writeAPIError(w, http.StatusNotFound, "not_found", "job route not found")
	}
}

func statusResponse(snapshot jobs.Snapshot) jobStatusResponse {
	warnings := append([]string{}, snapshot.Warnings...)
	return jobStatusResponse{
		ID:                  snapshot.ID,
		Status:              snapshot.Status,
		RepoName:            snapshot.RepoName,
		RepoPath:            snapshot.RepoPath,
		CreatedAt:           snapshot.CreatedAt,
		StartedAt:           snapshot.StartedAt,
		FinishedAt:          snapshot.FinishedAt,
		ElapsedMilliseconds: snapshot.Elapsed.Milliseconds(),
		Progress:            snapshot.Progress,
		Warnings:            warnings,
		Error:               snapshot.Failure,
	}
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
