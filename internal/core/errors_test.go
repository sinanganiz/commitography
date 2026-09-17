package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestReasonEnumerationMatchesReasons(t *testing.T) {
	seen := map[Reason]bool{}
	for _, r := range Reasons() {
		if seen[r] {
			t.Errorf("Reasons lists %q twice", r)
		}
		seen[r] = true
		if !ValidReason(r) {
			t.Errorf("ValidReason(%q) = false", r)
		}
	}
	if ValidReason("invented_code") {
		t.Error("ValidReason accepted a code the enumeration does not declare")
	}
	if ValidReason("") {
		t.Error("ValidReason accepted the empty reason")
	}
}

func TestExitCodeComesFromTheClass(t *testing.T) {
	user := NewUserError(ReasonEmptyRepository, "repo", "Make a commit.", "the repository has no commits")
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"no error", nil, ExitOK},
		{"user error", user, ExitUser},
		{"wrapped user error", fmt.Errorf("outer: %w", user), ExitUser},
		{"internal error", Internalf(errors.New("cause"), "writing report"), ExitInternal},
		{"unclassified error", errors.New("bare"), ExitInternal},
	} {
		if got := ExitCode(tc.err); got != tc.want {
			t.Errorf("%s: ExitCode = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestHTTPStatusComesFromTheClassAndReason(t *testing.T) {
	for _, tc := range []struct {
		reason Reason
		want   int
	}{
		{ReasonPathNotFound, http.StatusBadRequest},
		{ReasonNotARepository, http.StatusBadRequest},
		{ReasonShallowClone, http.StatusBadRequest},
		{ReasonPathOutsideAllowedRoots, http.StatusForbidden},
		{ReasonSymlinkEscapedRoot, http.StatusForbidden},
		{ReasonRequestTooLarge, http.StatusRequestEntityTooLarge},
	} {
		err := NewUserError(tc.reason, "", "Fix it.", "something happened")
		if got := HTTPStatus(err); got != tc.want {
			t.Errorf("reason %q: HTTPStatus = %d, want %d", tc.reason, got, tc.want)
		}
	}
	if got := HTTPStatus(Internalf(errors.New("cause"), "starting job")); got != http.StatusInternalServerError {
		t.Errorf("internal error: HTTPStatus = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := HTTPStatus(errors.New("bare")); got != http.StatusInternalServerError {
		t.Errorf("unclassified error: HTTPStatus = %d, want %d", got, http.StatusInternalServerError)
	}
	if got := HTTPStatus(nil); got != http.StatusOK {
		t.Errorf("no error: HTTPStatus = %d, want %d", got, http.StatusOK)
	}
}

// The artifact rendering is the one ADR-0067 clause 2 governs, so it must not
// carry the offending value even though the diagnostic does.
func TestArtifactRenderingDropsTheOffendingValue(t *testing.T) {
	err := NewUserError(ReasonNotARepository, "/private/typed/path", "Point at a directory that contains .git.",
		"the path is not a git repository")

	diagnostic := err.Diagnostic()
	if !strings.Contains(diagnostic, "/private/typed/path") {
		t.Errorf("diagnostic %q does not name the offending value", diagnostic)
	}
	if !strings.Contains(diagnostic, "contains .git") {
		t.Errorf("diagnostic %q does not name the remedy", diagnostic)
	}

	artifact := Artifact(err)
	if strings.Contains(artifact, "/private/typed/path") {
		t.Errorf("artifact rendering %q names the offending value", artifact)
	}
	if !strings.Contains(artifact, "contains .git") {
		t.Errorf("artifact rendering %q does not name the remedy", artifact)
	}
}

func TestArtifactRenderingOfAnInternalErrorRevealsNothing(t *testing.T) {
	err := Internalf(errors.New("open /home/someone/.config: permission denied"), "loading configuration")
	if got := Artifact(err); got != internalArtifactMessage {
		t.Errorf("Artifact = %q, want %q", got, internalArtifactMessage)
	}
	if !strings.Contains(err.Error(), "loading configuration") {
		t.Errorf("diagnostic %q does not locate the origin", err.Error())
	}
}

func TestReasonAndRemedySurviveWrapping(t *testing.T) {
	cause := errors.New("cause")
	user := NewUserError(ReasonInvalidConfiguration, "theme: dark", "Set theme to default.",
		"the configuration carries an invalid value").Wrapping(cause)
	wrapped := fmt.Errorf("loading configuration: %w", user)

	if got := ReasonOf(wrapped); got != ReasonInvalidConfiguration {
		t.Errorf("ReasonOf = %q, want %q", got, ReasonInvalidConfiguration)
	}
	if got := Remedy(wrapped); got != "Set theme to default." {
		t.Errorf("Remedy = %q", got)
	}
	if !errors.Is(wrapped, cause) {
		t.Error("the cause is not recoverable with errors.Is")
	}
	if ClassOf(wrapped) != ClassUser {
		t.Errorf("ClassOf = %v, want user", ClassOf(wrapped))
	}
}

func TestReasonOfAnUnclassifiedErrorIsEmpty(t *testing.T) {
	if got := ReasonOf(errors.New("bare")); got != "" {
		t.Errorf("ReasonOf = %q, want the empty reason", got)
	}
	if got := ClassOf(nil); got != ClassInternal {
		t.Errorf("ClassOf(nil) = %v, want internal", got)
	}
}
