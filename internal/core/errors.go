// The two error classes and the single mappings from a class onto an exit code
// and an HTTP status (ADR-0041 clauses 1 to 4).
//
// Classification is carried by the type. A user error is something the operator
// can fix: the invocation, the configuration, or the repository. An internal
// error is a defect or an environment failure. Nothing else exists, and an
// unclassified error is internal, because guessing that an unrecognised failure
// is the operator's fault is the more damaging guess.
//
// Two renderings exist, because ADR-0067 makes the rule about paths depend on
// where a message is going rather than on how it is worded:
//
//   - Diagnostic, for standard error, may name the offending value, which may
//     be a path the operator supplied in this invocation (clause 3).
//   - Artifact, for anything that can leave the machine — the report, an API
//     response, an exported image — never names it (clause 2).
//
// Exit codes are ADR-0034 clause 5. HTTP statuses are the values the API
// already returns; this file is the only place either is produced from an
// error, so no call site judges severity (ADR-0041 clause 4).

package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Class is the classification every error carries (ADR-0041 clause 1).
type Class int

const (
	// ClassInternal is the zero value, so an error that reaches a mapping
	// without being classified is treated as a defect rather than as the
	// operator's mistake.
	ClassInternal Class = iota
	ClassUser
)

// String names the class for a diagnostic.
func (c Class) String() string {
	if c == ClassUser {
		return "user"
	}
	return "internal"
}

// Exit codes (ADR-0034 clause 5). These values are the command's contract: a
// continuous integration job branches on them without parsing messages.
const (
	ExitOK       = 0
	ExitInternal = 1
	ExitUser     = 2
)

// internalArtifactMessage is what an internal error says to a destination that
// can leave the machine. Wrapping context locates the origin for whoever reads
// the diagnostic; it is not a sentence for a consumer, and it may name paths.
const internalArtifactMessage = "an internal error occurred"

// UserError is something the operator can fix (ADR-0041 clause 2). It carries
// the reason code, the offending value and the remedy; a message stating
// neither the value nor the remedy is incomplete.
//
// Summary MUST NOT repeat Value. ADR-0067 clause 5 keeps the two apart so that
// the artifact rendering can drop the value without rewriting the sentence, and
// so that no message ever contains both a supplied and a resolved form.
type UserError struct {
	// Reason is a code from the enumeration. Construction goes through
	// NewUserError, which is what the catalogue checker reads to learn which
	// codes the tree can emit.
	Reason Reason

	// Value is the offending value exactly as the operator supplied it, never
	// resolved, absolutised or canonicalised (ADR-0067 clause 3). It is empty
	// where no value may be shown, which is every value that arrived in a
	// request rather than from the operator's own invocation.
	Value string

	// Summary states what happened.
	Summary string

	// Remedy states what to do about it.
	Remedy string

	// Err is the cause, for errors.Is and errors.As. It is never rendered:
	// a cause carries whatever text its origin chose, including paths.
	Err error
}

// NewUserError builds a user error. Every argument is required; the checker
// verifies that reason is one of the enumeration's constants and that remedy is
// not empty, so an incomplete user error fails the gate rather than review.
func NewUserError(reason Reason, value, remedy, format string, args ...any) *UserError {
	return &UserError{
		Reason:  reason,
		Value:   value,
		Summary: fmt.Sprintf(format, args...),
		Remedy:  remedy,
	}
}

// Wrapping records the cause and returns the same error, so a constructor call
// stays one expression.
func (e *UserError) Wrapping(cause error) *UserError {
	e.Err = cause
	return e
}

// Error is the diagnostic rendering and the value errors.As callers see.
func (e *UserError) Error() string { return e.Diagnostic() }

// Diagnostic renders the error for standard error: what happened, the offending
// value where there is one, and the remedy (ADR-0067 clause 3).
func (e *UserError) Diagnostic() string {
	var b strings.Builder
	b.WriteString(e.Summary)
	if e.Value != "" {
		b.WriteString(" (")
		b.WriteString(e.Value)
		b.WriteString(")")
	}
	if e.Remedy != "" {
		b.WriteString("\n\n")
		b.WriteString(e.Remedy)
	}
	return b.String()
}

// Artifact renders the error for a destination that can leave the machine: the
// summary and the remedy, never the offending value (ADR-0067 clause 2).
func (e *UserError) Artifact() string {
	if e.Remedy == "" {
		return e.Summary
	}
	// A summary is a clause without a full stop and a remedy is a sentence, so
	// one full stop joins them into prose a consumer can display as it stands.
	return e.Summary + ". " + e.Remedy
}

// Unwrap exposes the cause.
func (e *UserError) Unwrap() error { return e.Err }

// Class implements the classification.
func (e *UserError) Class() Class { return ClassUser }

// InternalError is a defect or an environment failure (ADR-0041 clause 3). It
// carries wrapping context sufficient to locate its origin, which is why its
// rendering is never sent anywhere that can leave the machine.
type InternalError struct {
	// Op names what was being done, in enough detail to find the site.
	Op string

	// Err is the cause.
	Err error
}

// Internalf builds an internal error whose context locates the origin.
func Internalf(cause error, format string, args ...any) *InternalError {
	return &InternalError{Op: fmt.Sprintf(format, args...), Err: cause}
}

// Error returns the wrapping context and the cause.
func (e *InternalError) Error() string {
	if e.Err == nil {
		return e.Op
	}
	return e.Op + ": " + e.Err.Error()
}

// Unwrap exposes the cause.
func (e *InternalError) Unwrap() error { return e.Err }

// Class implements the classification.
func (e *InternalError) Class() Class { return ClassInternal }

// classified is what both classes implement. An error type outside this file
// that implements it takes part in the mappings below; the class checker is
// what keeps the set to two.
type classified interface {
	error
	Class() Class
}

// ClassOf classifies an error. Anything that is not a user error is internal,
// including a nil-free error from a package that has not been migrated.
func ClassOf(err error) Class {
	var c classified
	if errors.As(err, &c) {
		return c.Class()
	}
	return ClassInternal
}

// ReasonOf returns the reason code an error carries, or the empty reason when
// it carries none. Only a user error carries one.
func ReasonOf(err error) Reason {
	var user *UserError
	if errors.As(err, &user) {
		return user.Reason
	}
	return ""
}

// OffendingValue returns the value an error names as offending, or the empty
// string. It is safe for a diagnostic and not for an artifact (ADR-0067
// clauses 2 and 3).
func OffendingValue(err error) string {
	var user *UserError
	if errors.As(err, &user) {
		return user.Value
	}
	return ""
}

// Remedy returns the remedy an error carries, or the empty string.
func Remedy(err error) string {
	var user *UserError
	if errors.As(err, &user) {
		return user.Remedy
	}
	return ""
}

// Artifact renders an error for a destination that can leave the machine
// (ADR-0067 clause 2). An internal error yields one fixed sentence, because its
// wrapping context exists to locate a defect and may name anything.
func Artifact(err error) string {
	var user *UserError
	if errors.As(err, &user) {
		return user.Artifact()
	}
	return internalArtifactMessage
}

// ExitCode is the only site that produces an exit code (ADR-0041 clause 4).
// The mapping is over the class and nothing else, so no call site can decide
// that its own failure deserves a different one.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	if ClassOf(err) == ClassUser {
		return ExitUser
	}
	return ExitInternal
}

// HTTPStatus is the only site that produces an HTTP status from an error
// (ADR-0041 clause 4).
//
// The class chooses the family: a user error is a client error, anything else
// is 500. Inside the client-error family the reason code selects the status,
// because HTTP distinguishes conditions the two classes do not — a path outside
// the allowed roots is refused, not malformed, and an oversized body has its
// own status. Those are the values the API already returns, and the refinement
// lives here rather than at a call site, which is what clause 4 requires.
//
// A reason with no case below is a client error at 400. Adding a condition
// therefore means adding a reason code, not choosing a number somewhere else.
func HTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if ClassOf(err) != ClassUser {
		return http.StatusInternalServerError
	}
	switch ReasonOf(err) {
	case ReasonPathOutsideAllowedRoots, ReasonSymlinkEscapedRoot:
		return http.StatusForbidden
	case ReasonRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusBadRequest
	}
}
