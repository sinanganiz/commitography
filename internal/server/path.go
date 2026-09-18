package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

// Remedies for the conditions a repository request is refused for.
//
// An allowed root is named in its supplied form: the operator passed it on the
// command line, which ADR-0067 clause 3 permits a diagnostic to repeat. A
// requested repository path is never named at all, because it arrived in a
// request.
const (
	allowedRootRemedy = "Pass --allowed-root pointing at a directory that exists; " +
		"inside a container, mount it there first."

	repositoryPathRemedy = "Give a path that exists on the machine running the server, " +
		"inside one of its allowed roots."

	outsideRootsRemedy = "Restart with --allowed-root pointing at a directory that contains " +
		"the repository, including its git directory for a linked worktree."
)

// EmptyAllowedRoots returns the allowed roots that contain nothing, in the form
// the operator supplied them (ADR-0067 clause 3). An empty root cannot hold a
// repository; as a container mount it usually means the source path was
// mistyped and Docker created an empty folder in its place.
func (a *App) EmptyAllowedRoots() []string {
	var empty []string
	for i, root := range a.allowedRoots {
		if entries, err := os.ReadDir(root); err == nil && len(entries) == 0 {
			empty = append(empty, a.suppliedRoots[i])
		}
	}
	return empty
}

// canonicalRoots resolves each allowed root and returns the resolved forms
// alongside the supplied forms, so that no later message has to guess which
// form it is holding (ADR-0067 clause 5).
func canonicalRoots(roots []string) (canonical, supplied []string, err error) {
	if len(roots) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil, core.Internalf(err, "getting the current working directory")
		}
		resolved, err := canonicalDirectory(cwd)
		if err != nil {
			return nil, nil, core.Internalf(err, "resolving the working directory as the allowed root")
		}
		// The default root is the working directory, which the operator did not
		// type. Its own name is the only form that stands for it without
		// printing a resolved path.
		return []string{resolved}, []string{"."}, nil
	}
	canonical = make([]string, 0, len(roots))
	for _, root := range roots {
		resolved, err := canonicalDirectory(root)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, core.NewUserError(core.ReasonPathNotFound, root, allowedRootRemedy,
				"an allowed root does not exist")
		}
		if err != nil {
			return nil, nil, core.NewUserError(core.ReasonPathNotFound, root, allowedRootRemedy,
				"an allowed root could not be used").Wrapping(err)
		}
		canonical = append(canonical, resolved)
	}
	return canonical, roots, nil
}

func canonicalDirectory(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical, err := resolvePath(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}
	return filepath.Clean(canonical), nil
}

// validateRepositoryPath resolves a requested repository path and refuses it
// unless it exists, lies inside an allowed root together with its git
// directory, and is a repository this build can analyse.
//
// No refusal names the path. It arrived in a request, which ADR-0067 clause 3
// excludes from the paths a diagnostic may repeat, and clause 2 excludes from
// an API response outright. The offending value is therefore empty on every
// error below, and each message carries its remedy instead.
func (a *App) validateRepositoryPath(path string, allowShallow bool) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", core.NewUserError(core.ReasonPathNotFound, "", repositoryPathRemedy,
			"the requested repository path could not be resolved").Wrapping(err)
	}
	canonical, err := resolvePath(abs)
	if err != nil {
		return "", core.NewUserError(core.ReasonPathNotFound, "", repositoryPathRemedy,
			"the requested repository path does not exist").Wrapping(err)
	}
	canonical = filepath.Clean(canonical)
	if !withinAnyRoot(a.allowedRoots, canonical) {
		return "", core.NewUserError(core.ReasonPathOutsideAllowedRoots, "", outsideRootsRemedy,
			"the requested repository path is outside the allowed roots")
	}

	// Preflight's refusals already carry a reason code and a remedy, so they
	// pass through unchanged. It is given the empty supplied path, which is
	// what keeps the request's own path out of the response.
	info, err := collect.New(core.SystemClock(), core.SystemFilesystem()).Preflight(context.Background(), canonical, "")
	if err != nil {
		return "", err
	}
	if info.IsShallow && !allowShallow {
		return "", collect.ShallowError("")
	}
	gitDir, err := git.Run(canonical, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", core.NewUserError(core.ReasonNotARepository, "", repositoryPathRemedy,
			"the repository's git directory could not be resolved").Wrapping(err)
	}
	gitDir, err = resolvePath(gitDir)
	if err != nil || !withinAnyRoot(a.allowedRoots, gitDir) {
		return "", core.NewUserError(core.ReasonPathOutsideAllowedRoots, "", outsideRootsRemedy,
			"the repository's git directory is outside the allowed roots")
	}
	return canonical, nil
}

func withinAnyRoot(roots []string, path string) bool {
	for _, root := range roots {
		if withinRoot(root, path) {
			return true
		}
	}
	return false
}

func withinRoot(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		root = strings.ToLower(root)
		path = strings.ToLower(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
