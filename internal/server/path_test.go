package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
)

func TestWithinRootRejectsPrefixSibling(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	sibling := root + "-other"
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if withinRoot(root, sibling) {
		t.Fatal("sibling path was accepted as a child")
	}
}

func TestValidateRepositoryRejectsOutsideAllowedRoot(t *testing.T) {
	t.Parallel()
	allowed := t.TempDir()
	outside := t.TempDir()
	app, err := newAppWithRoots(nil, []string{allowed})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.validateRepositoryPath(outside, false)
	if err == nil {
		t.Fatal("outside repository was accepted")
	}
	if got := core.ReasonOf(err); got != core.ReasonPathOutsideAllowedRoots {
		t.Fatalf("reason = %q, want %q", got, core.ReasonPathOutsideAllowedRoots)
	}
}

func TestValidateRepositoryAcceptsRepositoryInsideRoot(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAppWithRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := app.validateRepositoryPath(root, false)
	if err != nil {
		t.Fatalf("validate repository: %v", err)
	}
	if canonical == "" {
		t.Fatal("canonical path is empty")
	}
}

// An allowed root comes from the command line, so the refusal names it, in the
// form it was given (ADR-0067 clause 3).
func TestMissingAllowedRootIsReportedByName(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "not-mounted")
	_, err := newAppWithRoots(nil, []string{missing})
	if got := core.ReasonOf(err); got != core.ReasonPathNotFound {
		t.Fatalf("reason = %q, want %q", got, core.ReasonPathNotFound)
	}
	if got := core.OffendingValue(err); got != missing {
		t.Fatalf("offending value = %q, want %q", got, missing)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("message %q does not name the root", err.Error())
	}
	if core.Remedy(err) == "" {
		t.Fatal("the refusal carries no remedy")
	}
}

func TestEmptyAllowedRootsAreReported(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()
	populated := t.TempDir()
	if err := os.Mkdir(filepath.Join(populated, "repository"), 0o755); err != nil {
		t.Fatal(err)
	}
	app, err := newAppWithRoots(nil, []string{empty, populated})
	if err != nil {
		t.Fatal(err)
	}
	// The supplied form is reported, never the resolved one (ADR-0067 clause 3).
	if got := app.EmptyAllowedRoots(); len(got) != 1 || got[0] != empty {
		t.Fatalf("empty roots = %v, want [%s]", got, empty)
	}
}

func TestValidateRepositoryRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	app, err := newAppWithRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.validateRepositoryPath(link, false); err == nil {
		t.Fatal("symlink escape was accepted")
	}
}
