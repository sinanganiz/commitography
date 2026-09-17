// The server's single error-response site.
//
// Two kinds of failure reach an HTTP route. A **classified error** is one of
// the two classes of ADR-0041, arriving from the analysis or from path
// validation; its status comes from the one mapping over the class, in
// core.HTTPStatus, and it carries a reason code from docs/metrics.md section
// 13. A **protocol condition** is an answer the HTTP contract owes and the two
// classes do not describe: an unknown job, a wrong method, a missing session, a
// job in the wrong state, a body that is not one JSON object. Section 13 has no
// code for any of those, and inventing one would need a decision (WP-0006
// clause 2), so they carry the status and code the contract already publishes,
// decided once in condition.response.
//
// Neither kind lets a route choose a status. errorResponse below is the only
// place either is produced, and condition.response is the only place an
// http.Status value appears for an error.

package server

import (
	"errors"
	"net/http"

	"github.com/sinanganiz/commitography/internal/core"
)

// condition names a protocol-level answer. WP-0037 owns this set, together with
// the API version it belongs to.
type condition int

const (
	conditionJobNotFound condition = iota + 1
	conditionJobRouteNotFound
	conditionJobNotDeletable
	conditionJobNotCancellable
	conditionReportNotReady
	conditionActiveJob
	conditionMethodNotAllowed
	conditionInvalidSession
	conditionInvalidOrigin
	conditionInvalidHost
	conditionInvalidJSON
	conditionRepoPathMissing
)

// response is the only place a protocol condition's status, code and message
// are decided.
func (c condition) response() (status int, code, message string) {
	switch c {
	case conditionJobNotFound:
		return http.StatusNotFound, "not_found", "job not found"
	case conditionJobRouteNotFound:
		return http.StatusNotFound, "not_found", "job route not found"
	case conditionJobNotDeletable:
		return http.StatusConflict, "invalid_job_state", "active jobs must be cancelled before deletion"
	case conditionJobNotCancellable:
		return http.StatusConflict, "invalid_job_state", "job cannot be cancelled in its current state"
	case conditionReportNotReady:
		return http.StatusConflict, "report_not_ready", "job has no current report"
	case conditionActiveJob:
		return http.StatusConflict, "active_job", "another analysis job is already active"
	case conditionMethodNotAllowed:
		return http.StatusMethodNotAllowed, "method_not_allowed", "method is not allowed"
	case conditionInvalidSession:
		return http.StatusUnauthorized, "invalid_session", "a valid local session is required"
	case conditionInvalidOrigin:
		return http.StatusForbidden, "invalid_origin", "request origin is not allowed"
	case conditionInvalidHost:
		return http.StatusForbidden, "invalid_host",
			"request host is not allowed; open the dashboard at http://127.0.0.1 or http://localhost"
	case conditionInvalidJSON:
		return http.StatusBadRequest, "invalid_json", "request body is not valid JSON"
	case conditionRepoPathMissing:
		return http.StatusBadRequest, "invalid_repository_path", "repoPath is required"
	default:
		return http.StatusInternalServerError, "internal_error", internalResponseMessage
	}
}

// internalResponseMessage is what a 500 says. An internal error's own text
// locates a defect and may name anything, so none of it travels (ADR-0067
// clause 2).
const internalResponseMessage = "an internal error occurred"

// conditionError is a protocol condition as an error value, optionally with a
// message that replaces the condition's default.
type conditionError struct {
	condition condition
	detail    string
}

// Error returns the message the response carries.
func (e *conditionError) Error() string {
	_, _, message := e.condition.response()
	if e.detail != "" {
		return e.detail
	}
	return message
}

// refuse builds a protocol condition's error.
func refuse(c condition) error { return &conditionError{condition: c} }

// refuseWith builds a protocol condition's error with a specific message.
func refuseWith(c condition, detail string) error {
	return &conditionError{condition: c, detail: detail}
}

// errorResponse produces the status, code, reason and message of any failure a
// route reports. It is the server's only status construction site for an error.
func errorResponse(err error) (status int, code string, reason core.Reason, message string) {
	var cond *conditionError
	if errors.As(err, &cond) {
		status, code, message := cond.condition.response()
		if cond.detail != "" {
			message = cond.detail
		}
		return status, code, "", message
	}
	r := core.ReasonOf(err)
	return core.HTTPStatus(err), publishedCode(r), r, core.Artifact(err)
}

// The API publishes two code vocabularies, both older than reason codes
// covering user errors: one for a rejected request and one for a job's terminal
// failure. The reason code is the single identity a condition has (ADR-0041
// clause 7) and travels beside them in its own field. Both published
// vocabularies are kept exactly as they are, because the embedded frontend
// switches on them to choose its guidance and its retry action, and web/ cannot
// change in this package. WP-0037 replaces both with the reason code in the
// same change as the frontend that reads them.

// publishedCode is the vocabulary of a rejected request.
func publishedCode(reason core.Reason) string {
	switch reason {
	case core.ReasonPathNotFound:
		return "invalid_repository_path"
	case core.ReasonPathOutsideAllowedRoots, core.ReasonSymlinkEscapedRoot:
		return "path_not_allowed"
	case core.ReasonNotARepository, core.ReasonEmptyRepository,
		core.ReasonGitUnavailable, core.ReasonGitVersionUnsupported:
		return "invalid_repository"
	case core.ReasonShallowClone:
		return "shallow_repository"
	case core.ReasonInvalidConfiguration:
		return "invalid_configuration"
	case core.ReasonYearBelowThreshold:
		return "invalid_wrapped_year"
	case core.ReasonRequestTooLarge:
		return "request_too_large"
	case "":
		return "internal_error"
	default:
		return string(reason)
	}
}

// publishedFailureCode is the vocabulary of a job's terminal failure. It
// distinguishes the class rather than the condition, which is all the job
// status contract ever published, with the requested year as its one special
// case.
func publishedFailureCode(err error) string {
	if core.ReasonOf(err) == core.ReasonYearBelowThreshold {
		return "invalid_wrapped_year"
	}
	if core.ClassOf(err) == core.ClassUser {
		return "invalid_analysis_request"
	}
	return "analysis_failed"
}
