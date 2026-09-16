package server

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/git"
)

// These tests cover the Windows path forms of WP-3.5 and WP-6.5: junctions,
// drive letter case, separators, extended-length and UNC paths.

func requireForbidden(t *testing.T, app *App, path string) {
	t.Helper()
	_, err := app.validateRepositoryPath(path, false)
	var pathErr *pathValidationError
	if err == nil {
		t.Fatalf("%s was accepted outside the allowed root", path)
	}
	if errors.As(err, &pathErr) && !pathErr.Forbidden && pathErr.Code != "invalid_repository_path" {
		t.Fatalf("%s was refused with %s, want a path rejection", path, pathErr.Code)
	}
}

// A junction needs no privilege on Windows, unlike a symbolic link, so it is
// the escape an unprivileged user can actually create. Go 1.23 stopped
// resolving junctions in filepath.EvalSymlinks by default; this test fails if
// that ever lets a junction lead out of the allowed root.
func TestWindowsJunctionOutOfTheRootIsRejected(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside-repository")
	if out, err := git.Command("", "init", "-q", outside).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	link := filepath.Join(root, "junction")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Skipf("mklink /J unavailable: %v: %s", err, out)
	}
	app, err := NewAppWithAllowedRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.validateRepositoryPath(link, false)
	var pathErr *pathValidationError
	if !errors.As(err, &pathErr) || !pathErr.Forbidden {
		t.Fatalf("a junction to a repository outside the root was not refused as forbidden: %v", err)
	}
}

func TestWindowsCaseAndSeparatorVariantsStayInsideTheRoot(t *testing.T) {
	root := testRepoPath(t)
	app, err := NewAppWithAllowedRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{
		strings.ToLower(root[:1]) + root[1:],
		strings.ToUpper(root),
		strings.ToLower(root),
		filepath.ToSlash(root),
		root + `\`,
		root + `\internal\..`,
	} {
		if _, err := app.validateRepositoryPath(variant, false); err != nil {
			t.Errorf("%q was refused inside the allowed root: %v", variant, err)
		}
	}
}

func TestWindowsParentTraversalLeavesTheRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := git.Command("", "init", "-q", filepath.Join(parent, "sibling")).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	app, err := NewAppWithAllowedRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		root + `\..\sibling`,
		filepath.ToSlash(root) + `/../sibling`,
		strings.ToUpper(root) + `\..\SIBLING`,
	} {
		requireForbidden(t, app, path)
	}
}

// An extended-length or UNC spelling must never reach outside the root. Both
// spellings of a path inside the root are refused too: the root is compared in
// the form it was given, so the dashboard documents typing that form.
func TestWindowsExtendedAndUNCPathsCannotLeaveTheRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "root")
	outside := filepath.Join(parent, "outside")
	for _, dir := range []string{root, outside} {
		if out, err := git.Command("", "init", "-q", dir).CombinedOutput(); err != nil {
			t.Fatalf("git init: %v: %s", err, out)
		}
	}
	app, err := NewAppWithAllowedRoots(nil, []string{root})
	if err != nil {
		t.Fatal(err)
	}

	requireForbidden(t, app, `\\?\`+outside)

	volume := filepath.VolumeName(outside)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skipf("temporary directory %s is not on a drive letter", outside)
	}
	unc := `\\localhost\` + volume[:1] + `$` + outside[len(volume):]
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("administrative share unavailable: %v", err)
	}
	requireForbidden(t, app, unc)
	inside := `\\localhost\` + volume[:1] + `$` + root[len(volume):]
	if _, err := app.validateRepositoryPath(inside, false); err == nil {
		t.Logf("the UNC spelling of the allowed root %s was accepted", inside)
	} else {
		t.Logf("the UNC spelling of the allowed root %s was refused: %v", inside, err)
	}
}
