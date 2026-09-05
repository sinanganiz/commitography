package collect

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sinanganiz/commitography/internal/model"
)

// MinGitVersion is the oldest git release commitography supports. Older
// versions lack flags the collector depends on.
const MinGitVersion = "2.22.0"

// ErrGitNotFound is returned when no git executable is present in PATH.
var ErrGitNotFound = errors.New("git executable not found in PATH; commitography requires git to be installed")

// ShallowError reports that the analyzed repository is a shallow clone, whose
// truncated history would make every statistic wrong.
type ShallowError struct {
	Path string
}

// ShallowMessage is the exact guidance printed when a shallow clone is refused.
const ShallowMessage = `this repository is a shallow clone, so its history is incomplete and all statistics would be wrong.

Fix it with:
  git fetch --unshallow

In CI, configure a full clone:
  GitHub Actions      -> actions/checkout with fetch-depth: 0
  GitLab CI           -> GIT_DEPTH: 0
  Bitbucket Pipelines -> clone: depth: full

To analyze anyway and accept incorrect results, pass --allow-shallow.`

// Error implements the error interface.
func (e *ShallowError) Error() string { return ShallowMessage }

var gitVersionRe = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// Preflight validates that the given path is analyzable and returns repository
// metadata. Checks run in a fixed order and fail fast, so the first error a
// user sees is the root cause rather than a downstream symptom.
func Preflight(repoPath string) (model.RepositoryInfo, error) {
	var info model.RepositoryInfo

	// 1. git availability.
	if _, err := exec.LookPath("git"); err != nil {
		return info, ErrGitNotFound
	}
	raw, err := runGit("", "--version")
	if err != nil {
		return info, ErrGitNotFound
	}

	// 2. git version.
	detected, ok := parseGitVersion(raw)
	if !ok {
		return info, fmt.Errorf("could not parse git version from %q; commitography requires git %s or newer", raw, MinGitVersion)
	}
	if compareVersions(detected, mustParseVersion(MinGitVersion)) < 0 {
		return info, fmt.Errorf("git %s is too old; commitography requires git %s or newer",
			formatVersion(detected), MinGitVersion)
	}

	// 3. Path is a repository.
	if _, err := runGit(repoPath, "rev-parse", "--git-dir"); err != nil {
		return info, fmt.Errorf("%s is not a git repository", repoPath)
	}

	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return info, fmt.Errorf("resolving %s: %w", repoPath, err)
	}
	info.Path = abs

	// 4. Shallow check.
	shallow, err := runGit(repoPath, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return info, err
	}
	info.IsShallow = shallow == "true"

	// 5. Graft check.
	gitDir, err := runGit(repoPath, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return info, err
	}
	info.HasGrafts = fileExists(filepath.Join(gitDir, "shallow")) ||
		fileExists(filepath.Join(gitDir, "info", "grafts"))

	// 6. Empty repository check.
	head, err := runGit(repoPath, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return info, errors.New("repository has no commits")
	}

	// 7. Head and default branch.
	info.HeadCommit = head
	if branch, err := runGit(repoPath, "symbolic-ref", "--short", "HEAD"); err == nil {
		info.DefaultBranch = branch
	}

	// 8. Name.
	info.Name = strings.TrimSuffix(filepath.Base(abs), ".git")

	return info, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseGitVersion extracts the leading numeric components of a `git --version`
// line. Distribution builds append arbitrary suffixes (".windows.3", ".msysgit"),
// so only the first three numbers are considered.
func parseGitVersion(s string) ([3]int, bool) {
	m := gitVersionRe.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, false
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

func mustParseVersion(s string) [3]int {
	v, ok := parseGitVersion(s)
	if !ok {
		panic("invalid version constant: " + s)
	}
	return v
}

func compareVersions(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}

func formatVersion(v [3]int) string {
	return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2])
}
