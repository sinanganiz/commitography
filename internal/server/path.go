package server

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sinanganiz/commitography/internal/collect"
	"github.com/sinanganiz/commitography/internal/gitcmd"
)

type pathValidationError struct {
	Code      string
	Message   string
	Forbidden bool
}

func (e *pathValidationError) Error() string { return e.Message }

func canonicalRoots(roots []string) ([]string, error) {
	if len(roots) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current working directory: %w", err)
		}
		roots = []string{cwd}
	}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		canonical, err := canonicalDirectory(root)
		if err != nil {
			return nil, fmt.Errorf("allowed root %q: %w", root, err)
		}
		out = append(out, canonical)
	}
	return out, nil
}

func canonicalDirectory(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(abs)
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

func (a *App) validateRepositoryPath(path string, allowShallow bool) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", &pathValidationError{Code: "invalid_repository_path", Message: err.Error()}
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", &pathValidationError{Code: "invalid_repository_path", Message: "repository path does not exist"}
	}
	canonical = filepath.Clean(canonical)
	if !withinAnyRoot(a.allowedRoots, canonical) {
		return "", &pathValidationError{
			Code:      "path_not_allowed",
			Message:   "repository path is outside the allowed roots",
			Forbidden: true,
		}
	}

	info, err := collect.Preflight(canonical)
	if err != nil {
		return "", &pathValidationError{Code: "invalid_repository", Message: err.Error()}
	}
	if info.IsShallow && !allowShallow {
		return "", &pathValidationError{Code: "shallow_repository", Message: (&collect.ShallowError{Path: canonical}).Error()}
	}
	gitDir, err := gitcmd.Run(canonical, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", &pathValidationError{Code: "invalid_repository", Message: "could not resolve the repository Git directory"}
	}
	gitDir, err = filepath.EvalSymlinks(gitDir)
	if err != nil || !withinAnyRoot(a.allowedRoots, gitDir) {
		return "", &pathValidationError{
			Code:      "path_not_allowed",
			Message:   "the repository Git directory is outside the allowed roots",
			Forbidden: true,
		}
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
