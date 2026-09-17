// The reason code enumeration. One set serves both family status reporting and
// user errors, so the same condition has one identity wherever it surfaces
// (ADR-0041 clause 7, ADR-0032 clause 4).
//
// `docs/metrics.md` section 13 is authoritative: a code absent from it does not
// exist (ADR-0062 clause 6). This file is the code's copy of that section, and
// TestReasonCodeCatalogue in internal/checks compares the two in both
// directions, so neither can gain or lose a code alone.
//
// Adding a code here without adding it there, or the reverse, fails the gate.
// Adding one to both requires a decision about what condition it names, not an
// assumption (WP-0006 clause 2).

package core

// Reason is a machine-readable reason code from docs/metrics.md section 13.
type Reason string

// Family status codes, carried by a skipped or degraded family (ADR-0032
// clause 2). WP-0008 wires these into the report document; this package only
// declares them, so the enumeration is complete from here on and each later
// package fills a slot rather than extending the set.
const (
	ReasonWorktreeUnavailable         Reason = "worktree_unavailable"
	ReasonNotImplemented              Reason = "not_implemented"
	ReasonEmptyPopulation             Reason = "empty_population"
	ReasonCardinalityLimit            Reason = "cardinality_limit"
	ReasonLowClassificationConfidence Reason = "low_classification_confidence"
	ReasonShallowClone                Reason = "shallow_clone"
	ReasonLimitReachedCommits         Reason = "limit_reached_commits"
	ReasonLimitReachedSize            Reason = "limit_reached_size"
	ReasonLimitReachedDuration        Reason = "limit_reached_duration"
	ReasonLimitReachedMemory          Reason = "limit_reached_memory"
	ReasonSymlinkEscapedRoot          Reason = "symlink_escaped_root"
	ReasonCapabilityUnavailableInMode Reason = "capability_unavailable_in_mode"
	ReasonExternalServiceUnavailable  Reason = "external_service_unavailable"
	ReasonBinaryFileSkipped           Reason = "binary_file_skipped"
	ReasonHistoryRewritten            Reason = "history_rewritten"
)

// User error codes, carried by an error that refuses a run or a request
// (ADR-0041 clause 2).
//
// ReasonShallowClone is above rather than here because it belongs to both
// groups: it refuses a run without the override and marks a family degraded
// with it.
const (
	ReasonInvalidInvocation       Reason = "invalid_invocation"
	ReasonInvalidConfiguration    Reason = "invalid_configuration"
	ReasonGitUnavailable          Reason = "git_unavailable"
	ReasonGitVersionUnsupported   Reason = "git_version_unsupported"
	ReasonPathNotFound            Reason = "path_not_found"
	ReasonNotARepository          Reason = "not_a_repository"
	ReasonPathOutsideAllowedRoots Reason = "path_outside_allowed_roots"
	ReasonEmptyRepository         Reason = "empty_repository"
	ReasonYearBelowThreshold      Reason = "year_below_threshold"
	ReasonRequestTooLarge         Reason = "request_too_large"
)

// Reasons returns every code in the enumeration. The checker compares this
// against docs/metrics.md section 13, so a constant that is declared above and
// omitted here is caught rather than silently unlisted.
func Reasons() []Reason {
	return []Reason{
		ReasonWorktreeUnavailable,
		ReasonNotImplemented,
		ReasonEmptyPopulation,
		ReasonCardinalityLimit,
		ReasonLowClassificationConfidence,
		ReasonShallowClone,
		ReasonLimitReachedCommits,
		ReasonLimitReachedSize,
		ReasonLimitReachedDuration,
		ReasonLimitReachedMemory,
		ReasonSymlinkEscapedRoot,
		ReasonCapabilityUnavailableInMode,
		ReasonExternalServiceUnavailable,
		ReasonBinaryFileSkipped,
		ReasonHistoryRewritten,
		ReasonInvalidInvocation,
		ReasonInvalidConfiguration,
		ReasonGitUnavailable,
		ReasonGitVersionUnsupported,
		ReasonPathNotFound,
		ReasonNotARepository,
		ReasonPathOutsideAllowedRoots,
		ReasonEmptyRepository,
		ReasonYearBelowThreshold,
		ReasonRequestTooLarge,
	}
}

// ValidReason reports whether r is in the enumeration.
func ValidReason(r Reason) bool {
	for _, known := range Reasons() {
		if known == r {
			return true
		}
	}
	return false
}
