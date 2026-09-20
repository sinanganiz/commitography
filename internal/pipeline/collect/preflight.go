package collect

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
)

// MinGitVersion is the oldest git release commitography supports. Older
// versions lack flags the collector depends on.
const MinGitVersion = "2.22.0"

// Remedies for the conditions preflight refuses. They are constants because a
// remedy is part of the error's contract, not a sentence a call site composes
// (ADR-0041 clause 2).
const (
	gitMissingRemedy = "Install git and make sure it is on the path; commitography needs no other service."

	gitOldRemedy = "Upgrade git to " + MinGitVersion + " or newer."

	emptyRepositoryRemedy = "Make at least one commit, then run the analysis again."

	// The two conditions below are also recognised from a failed invocation,
	// so their wording lives with the recognition in the git package and is
	// named from here rather than repeated: one condition has one remedy
	// wherever it surfaces (ADR-0041 clause 2).
	notRepositoryRemedy = git.NotARepositoryRemedy

	// ShallowRemedy is exported because the server reports the same condition
	// for a repository it was asked to analyse.
	ShallowRemedy = git.ShallowRemedy

	// ShallowSummary is exported for the same reason as ShallowRemedy.
	ShallowSummary = git.ShallowSummary
)

// ShallowError builds the refusal for a shallow clone. suppliedPath is named in
// the diagnostic exactly as it arrived, so it must be the operator's own form
// and never a resolved one (ADR-0067 clauses 3 and 5); the server passes the
// empty string, because a path from a request may not be echoed at all.
func ShallowError(suppliedPath string) error {
	return core.NewUserError(core.ReasonShallowClone, suppliedPath, ShallowRemedy, "%s", ShallowSummary)
}

// Preflight validates that the given path is analyzable and returns repository
// metadata. Checks run in a fixed order and fail fast, so the first error a
// user sees is the root cause rather than a downstream symptom.
//
// repoPath is used for the git invocations and is therefore whatever form the
// caller resolved. Messages name suppliedPath instead, which the caller passes
// unresolved; Preflight takes both so no message has to guess which form it
// holds (ADR-0067 clause 5). Its git commands are cancelled with ctx.
func (c *Collector) Preflight(ctx context.Context, repoPath, suppliedPath string) (model.RepositoryInfo, error) {
	var info model.RepositoryInfo

	// 1. git availability.
	if _, err := git.LookPath(); err != nil {
		return info, gitUnavailable(err)
	}
	raw, err := git.Output(ctx, git.At("", "--version"))
	if err != nil {
		return info, gitUnavailable(err)
	}

	// 2. git version.
	detected, ok := parseGitVersion(raw)
	if !ok {
		return info, core.NewUserError(core.ReasonGitUnavailable, raw, gitMissingRemedy,
			"the version of the installed git could not be determined")
	}
	if compareVersions(detected, mustParseVersion(MinGitVersion)) < 0 {
		return info, core.NewUserError(core.ReasonGitVersionUnsupported, formatVersion(detected), gitOldRemedy,
			"the installed git is older than %s, which commitography requires", MinGitVersion)
	}

	// 3. Path is a repository.
	if _, err := git.Output(ctx, git.At(repoPath, "rev-parse", "--git-dir")); err != nil {
		return info, core.NewUserError(core.ReasonNotARepository, suppliedPath, notRepositoryRemedy,
			"the path is not a git repository").Wrapping(err)
	}

	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return info, core.Internalf(err, "resolving the repository path")
	}
	info.Path = abs

	// 4. Shallow check.
	shallow, err := git.Output(ctx, git.At(repoPath, "rev-parse", "--is-shallow-repository"))
	if err != nil {
		return info, core.Internalf(err, "reading whether the repository is shallow")
	}
	info.IsShallow = shallow == "true"

	// 5. Graft check.
	gitDir, err := git.Output(ctx, git.At(repoPath, "rev-parse", "--absolute-git-dir"))
	if err != nil {
		return info, core.NewUserError(core.ReasonNotARepository, suppliedPath, notRepositoryRemedy,
			"the repository's git directory could not be resolved").Wrapping(err)
	}
	info.HasGrafts = c.fileExists(filepath.Join(gitDir, "shallow")) ||
		c.fileExists(filepath.Join(gitDir, "info", "grafts"))

	// 6. Empty repository check.
	head, err := git.Output(ctx, git.At(repoPath, "rev-parse", "--verify", "HEAD"))
	if err != nil {
		return info, core.NewUserError(core.ReasonEmptyRepository, suppliedPath, emptyRepositoryRemedy,
			"the repository has no commits")
	}

	// 7. Head and default branch.
	info.HeadCommit = head
	if branch, err := git.Output(ctx, git.At(repoPath, "symbolic-ref", "--short", "HEAD")); err == nil {
		info.DefaultBranch = branch
	}

	// 8. Name.
	info.Name = strings.TrimSuffix(filepath.Base(abs), ".git")

	return info, nil
}

// gitUnavailable is the refusal for git being absent or unrunnable. The
// offending value is empty: git's location is not something the operator
// supplied in this invocation, and naming a resolved one would put an absolute
// path in a diagnostic (ADR-0067 clause 3).
func gitUnavailable(cause error) error {
	return core.NewUserError(core.ReasonGitUnavailable, "", gitMissingRemedy,
		"git was not found on the path, and commitography cannot read a repository without it").Wrapping(cause)
}

func (c *Collector) fileExists(path string) bool {
	_, err := c.files.Stat(path)
	return err == nil
}

// parseGitVersion extracts the leading numeric components of a `git --version`
// line. Distribution builds append arbitrary suffixes (".windows.3", ".msysgit"),
// so only the first three numbers are considered.
func parseGitVersion(s string) ([3]int, bool) {
	m := regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`).FindStringSubmatch(s)
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
