package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline"
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
	ID           string     `json:"id"`
	Status       Status     `json:"status"`
	RepoName     string     `json:"repoName"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	WarningCount int        `json:"warningCount"`
}

// jobStatusResponse deliberately carries no repository path. An API response
// is an artifact that can leave the machine, and ADR-0067 clause 2 admits no
// exception: the name identifies the repository to its owner, and the path
// would identify the machine to anyone else. The frontend renders the name and
// treats the path as optional, so nothing breaks by its absence; WP-0046 drops
// the field from the frontend's own type.
type jobStatusResponse struct {
	ID                  string                  `json:"id"`
	Status              Status                  `json:"status"`
	RepoName            string                  `json:"repoName"`
	CreatedAt           time.Time               `json:"createdAt"`
	StartedAt           *time.Time              `json:"startedAt"`
	FinishedAt          *time.Time              `json:"finishedAt"`
	ElapsedMilliseconds int64                   `json:"elapsedMilliseconds"`
	Progress            *pipeline.ProgressEvent `json:"progress"`
	Warnings            []string                `json:"warnings"`
	Error               *Failure                `json:"error"`
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
	ID     string `json:"id"`
	Status Status `json:"status"`
}

type apiError struct {
	Error apiErrorBody `json:"error"`
}

// apiErrorBody carries both identities of a failure during this API version.
// Reason is the code from docs/metrics.md section 13, which is the one identity
// a condition has wherever it surfaces (ADR-0041 clause 7); it is empty for a
// protocol condition, which section 13 does not describe. Code is what the API
// has published since before reason codes covered user errors, kept stable
// because the embedded frontend switches on it. WP-0037 collapses the two.
type apiErrorBody struct {
	Code    string      `json:"code"`
	Reason  core.Reason `json:"reason,omitempty"`
	Message string      `json:"message"`
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
		ReportSchemaVersion: core.DocumentVersion().Major,
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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	decoder.DisallowUnknownFields()
	var request createJobRequest
	if err := decoder.Decode(&request); err != nil {
		writeDecodeError(w, err, "request body is not valid JSON")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeDecodeError(w, err, "request body must contain one JSON object")
		return
	}
	if strings.TrimSpace(request.RepoPath) == "" {
		writeAPIError(w, refuse(conditionRepoPathMissing))
		return
	}
	canonicalPath, err := a.validateRepositoryPath(request.RepoPath, request.Options.AllowShallow)
	if err != nil {
		// Every refusal validateRepositoryPath returns is classified, so its
		// status comes from the one mapping and this route judges nothing.
		writeAPIError(w, err)
		return
	}

	// countMerges mirrors the CLI flag: true overrides the repository config,
	// false leaves the repository's count_merges setting in charge.
	options := pipeline.Options{
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
		if errors.Is(err, ErrActiveJob) {
			writeAPIError(w, refuse(conditionActiveJob))
			return
		}
		writeAPIError(w, core.Internalf(err, "starting the analysis job"))
		return
	}
	writeJSON(w, http.StatusAccepted, createJobResponse{ID: snapshot.ID, Status: snapshot.Status})
}

func (a *App) jobRoute(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/jobs/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, refuse(conditionJobNotFound))
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			snapshot, err := a.Jobs.Get(id)
			if err != nil {
				writeAPIError(w, refuse(conditionJobNotFound))
				return
			}
			writeJSON(w, http.StatusOK, statusResponse(snapshot))
		case http.MethodDelete:
			if err := a.Jobs.Delete(id); err != nil {
				if errors.Is(err, ErrJobNotFound) {
					writeAPIError(w, refuse(conditionJobNotFound))
				} else {
					writeAPIError(w, refuse(conditionJobNotDeletable))
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
		writeAPIError(w, refuse(conditionJobNotFound))
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
			if errors.Is(err, ErrJobNotFound) {
				writeAPIError(w, refuse(conditionJobNotFound))
			} else {
				writeAPIError(w, refuse(conditionReportNotReady))
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
		if snapshot, err := a.Jobs.Get(id); err == nil && snapshot.Status == StatusCancelled {
			writeJSON(w, http.StatusOK, statusResponse(snapshot))
			return
		}
		if err := a.Jobs.Cancel(id); err != nil {
			if errors.Is(err, ErrJobNotFound) {
				writeAPIError(w, refuse(conditionJobNotFound))
			} else {
				writeAPIError(w, refuse(conditionJobNotCancellable))
			}
			return
		}
		snapshot, _ := a.Jobs.Get(id)
		writeJSON(w, http.StatusAccepted, statusResponse(snapshot))
	case "":
		writeAPIError(w, refuse(conditionJobNotFound))
	default:
		writeAPIError(w, refuse(conditionJobRouteNotFound))
	}
}

// maxRequestBody bounds a job request. The contract maps a larger body to 413.
const maxRequestBody = 1 << 20

// writeDecodeError distinguishes a body over the size limit from one that is
// not a single valid JSON object. The first is a user error and carries a
// reason code; the second is a protocol condition, because section 13 lists no
// code for a malformed body and inventing one needs a decision.
func writeDecodeError(w http.ResponseWriter, err error, message string) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeAPIError(w, core.NewUserError(core.ReasonRequestTooLarge, "",
			"Send a smaller request body; the cap is 1 MiB.",
			"the request body is larger than the server accepts").Wrapping(err))
		return
	}
	writeAPIError(w, refuseWith(conditionInvalidJSON, message))
}

func statusResponse(snapshot Snapshot) jobStatusResponse {
	warnings := append([]string{}, snapshot.Warnings...)
	return jobStatusResponse{
		ID:                  snapshot.ID,
		Status:              snapshot.Status,
		RepoName:            snapshot.RepoName,
		CreatedAt:           snapshot.CreatedAt,
		StartedAt:           snapshot.StartedAt,
		FinishedAt:          snapshot.FinishedAt,
		ElapsedMilliseconds: snapshot.Elapsed.Milliseconds(),
		Progress:            snapshot.Progress,
		Warnings:            warnings,
		Error:               snapshot.Failure,
	}
}

func summarizeJob(snapshot Snapshot) jobSummary {
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

// writeAPIError is the only site that writes an error response. The status
// comes from errorResponse, never from the route (ADR-0041 clause 4).
func writeAPIError(w http.ResponseWriter, err error) {
	status, code, reason, message := errorResponse(err)
	writeJSON(w, status, apiError{Error: apiErrorBody{Code: code, Reason: reason, Message: message}})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeAPIError(w, refuse(conditionMethodNotAllowed))
}
