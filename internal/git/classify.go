// Failure classification for git invocations (ADR-0041 clause 1, WP-0011
// clause 5). Three conditions are the operator's to fix and carry a reason
// code; everything else is internal, because guessing that an unrecognised
// failure is the operator's fault is the more damaging guess.
//
// The conditions are recognised from git's own standard error. That is only
// sound because every invocation runs under a fixed C locale (harden.go), so
// the sentences below are the ones git emits whatever the operator's
// language is.
//
// Git's standard error is repository-influenced and free to name paths, so it
// never reaches a rendered message. A classified failure carries a fixed
// summary and remedy, and keeps git's own words in the wrapped cause, which
// no rendering prints (ADR-0045, ADR-0067 clause 2).

package git

import (
	"errors"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
)

// Remedies for the conditions a failed invocation is refused for. They are
// constants because a remedy is part of the error's contract, not a sentence
// a call site composes (ADR-0041 clause 2), and they live beside the
// detection so that one condition has one remedy wherever it surfaces.
const (
	// NotARepositoryRemedy is exported because the collect stage refuses the
	// same condition from its own check, before any invocation fails.
	NotARepositoryRemedy = "Point at the directory that contains .git, and check that git can read it."

	// ShallowRemedy is exported for the same reason, and because the server
	// reports the condition for a repository it was asked to analyse.
	ShallowRemedy = `Fix it with:
  git fetch --unshallow

In CI, configure a full clone:
  GitHub Actions      -> actions/checkout with fetch-depth: 0
  GitLab CI           -> GIT_DEPTH: 0
  Bitbucket Pipelines -> clone: depth: full

To analyze anyway and accept incorrect results, pass --allow-shallow.`

	// ShallowSummary is exported for the same reason as ShallowRemedy.
	ShallowSummary = "this repository is a shallow clone, so its history is incomplete and all statistics would be wrong"
)

// classify turns a failed invocation into a classified error. stderr is what
// the invocation wrote to standard error; cause is what os/exec reported.
func classify(spec Spec, stderr string, cause error) error {
	message := strings.TrimSpace(stderr)
	wrapped := cause
	if message != "" {
		// git's own words locate the failure far better than "exit status 1",
		// and an internal error's wrapping context is never rendered to an
		// artifact.
		wrapped = errors.New(message)
	}
	switch condition(message) {
	case conditionNotARepository:
		return core.NewUserError(core.ReasonNotARepository, "", NotARepositoryRemedy,
			"the path is not a git repository").Wrapping(wrapped)
	case conditionShallow:
		return core.NewUserError(core.ReasonShallowClone, "", ShallowRemedy,
			"%s", ShallowSummary).Wrapping(wrapped)
	case conditionMissingCredential:
		// Recorded deviation, removed by WP-0041: ADR-0016 clause 5 requires
		// this to be reported as a remote needing credentials the environment
		// did not provide, which is a user error. No such reason code exists,
		// and docs/metrics.md is the only place one can be created
		// (ADR-0062 clause 6), which this package may not edit. WP-0041 owns
		// ADR-0016 and the invocations that can reach this condition —
		// nothing here clones or fetches — so it adds the code and swaps the
		// constructor. Until then the condition is named in the diagnostic
		// rather than silently lumped in with the rest.
		return core.Internalf(wrapped, "%s needs a credential the environment did not provide", spec.describe())
	default:
		return core.Internalf(wrapped, "running %s", spec.describe())
	}
}

// condition is what git's standard error says went wrong.
type conditionCode int

const (
	conditionUnknown conditionCode = iota
	conditionNotARepository
	conditionShallow
	conditionMissingCredential
)

// condition recognises the three conditions that are the operator's to fix.
// The phrases are git's, in the C locale; a phrase git stops using stops
// matching, which turns a user error into an internal one rather than into a
// wrong remedy.
func condition(stderr string) conditionCode {
	lower := strings.ToLower(stderr)
	for _, phrase := range []string{
		"not a git repository",
		"does not appear to be a git repository",
		"no such file or directory", // -C onto a path that is not there
		"cannot change to ",         // what -C says when the directory is unusable
	} {
		if strings.Contains(lower, phrase) {
			return conditionNotARepository
		}
	}
	if strings.Contains(lower, "shallow") {
		return conditionShallow
	}
	for _, phrase := range []string{
		"could not read username",
		"could not read password",
		"authentication failed",
		"terminal prompts disabled",
		"permission denied (publickey)",
	} {
		if strings.Contains(lower, phrase) {
			return conditionMissingCredential
		}
	}
	return conditionUnknown
}
