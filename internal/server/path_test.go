package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWithinRootRejectsPrefixSibling(t *testing.T) {
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
	allowed := t.TempDir()
	outside := t.TempDir()
	app, err := NewAppWithAllowedRoots(nil, []string{allowed})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.validateRepositoryPath(outside, false)
	if err == nil {
		t.Fatal("outside repository was accepted")
	}
	pathErr, ok := err.(*pathValidationError)
	if !ok || !pathErr.Forbidden {
		t.Fatalf("error = %T %v, want forbidden path error", err, err)
	}
}

func TestValidateRepositoryAcceptsRepositoryInsideRoot(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	app, err := NewAppWithAllowedRoots(nil, []string{root})
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

func TestMissingAllowedRootIsReportedByName(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-mounted")
	_, err := NewAppWithAllowedRoots(nil, []string{missing})
	var missingRoot *MissingRootError
	if !errors.As(err, &missingRoot) || missingRoot.Root != missing {
		t.Fatalf("error = %T %v, want MissingRootError for %s", err, err, missing)
	}
	if want := fmt.Sprintf("allowed root %q does not exist", missing); err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
}

func TestEmptyAllowedRootsAreReported(t *testing.T) {
	empty := t.TempDir()
	populated := t.TempDir()
	if err := os.Mkdir(filepath.Join(populated, "repository"), 0o755); err != nil {
		t.Fatal(err)
	}
	app, err := NewAppWithAllowedRoots(nil, []string{empty, populated})
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalDirectory(empty)
	if err != nil {
		t.Fatal(err)
	}
	if got := app.EmptyAllowedRoots(); len(got) != 1 || got[0] != want {
		t.Fatalf("empty roots = %v, want [%s]", got, want)
	}
}

func TestValidateRepositoryRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	app, err := NewAppWithAllowedRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.validateRepositoryPath(link, false); err == nil {
		t.Fatal("symlink escape was accepted")
	}
}
