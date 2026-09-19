package checks

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
)

// TestErrorClassAtPackageBoundaries enforces ADR-0041 clause 1 where it can be
// observed rather than inferred: it drives the error-producing entry points of
// every package that has them and requires each failure to arrive as one of the
// two classes, with a reason code from the catalogue and a remedy when it is a
// user error.
//
// It is a behavioural checker rather than a static one because the property is
// behavioural. A static rule would have to decide which fmt.Errorf is on a
// return path to another package, and the answer is not in the syntax.
//
// Two packages are outside it by record. internal/core/config and
// internal/core/filter may not import internal/core (ADR-0066 clause 3), so
// they cannot classify their own errors; the pipeline root classifies them at
// the one place that consumes them, and the configuration cases below are what
// verify it. That narrowing is permanent, not a package's to widen.
//
// internal/checks/gatesummary is also outside it. It is gate tooling with its
// own main, not a package the product's errors cross, and it reports to a gate
// rather than to an operator.
func TestErrorClassAtPackageBoundaries(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	documented, _ := reasonCodes(t, repo)
	root := filepath.Join(repo.root, "testdata", "fixtures")

	fixture := func(name string) string {
		t.Helper()
		dir := filepath.Join(root, name)
		if _, err := os.Stat(dir); err != nil {
			fatal(t, 64, "fixture %q is missing; the gates generate it with `make fixtures`", name)
		}
		return dir
	}
	analyse := func(opts pipeline.Options) error {
		opts.OperatorSupplied = true
		_, err := newAnalyzer().Run(context.Background(), opts, nil)
		return err
	}

	// A configuration file whose date source is invalid. The value is rejected
	// by internal/core/config, which cannot classify it itself.
	badConfig := filepath.Join(t.TempDir(), "bad.yml")
	if err := os.WriteFile(badConfig, []byte("date_source: midnight\n"), 0o600); err != nil {
		fatal(t, 41, "writing a configuration fixture: %v", err)
	}
	badPattern := filepath.Join(t.TempDir(), "pattern.yml")
	if err := os.WriteFile(badPattern, []byte("exclude_paths:\n  - \"[\"\n"), 0o600); err != nil {
		fatal(t, 41, "writing a configuration fixture: %v", err)
	}

	for _, tc := range []struct {
		name   string
		err    error
		class  core.Class
		reason core.Reason
	}{
		{
			name:   "collect: the path is not a repository",
			err:    second(newCollector().Preflight(context.Background(), t.TempDir(), "./not-a-repository")),
			class:  core.ClassUser,
			reason: core.ReasonNotARepository,
		},
		{
			name:   "collect: the repository has no commits",
			err:    second(newCollector().Preflight(context.Background(), fixture("empty"), "./empty")),
			class:  core.ClassUser,
			reason: core.ReasonEmptyRepository,
		},
		{
			name:   "collect: the history artifact cannot be read",
			err:    second(collect.ReadHistory(filepath.Join(t.TempDir(), "absent.json"))),
			class:  core.ClassInternal,
			reason: "",
		},
		{
			name:   "pipeline: a shallow clone without the override",
			err:    analyse(pipeline.Options{RepoPath: fixture("shallow")}),
			class:  core.ClassUser,
			reason: core.ReasonShallowClone,
		},
		{
			name:   "pipeline: an empty repository",
			err:    analyse(pipeline.Options{RepoPath: fixture("empty")}),
			class:  core.ClassUser,
			reason: core.ReasonEmptyRepository,
		},
		{
			name:   "pipeline: a configuration value the analysis cannot use",
			err:    analyse(pipeline.Options{RepoPath: fixture("basic"), ConfigPath: badConfig}),
			class:  core.ClassUser,
			reason: core.ReasonInvalidConfiguration,
		},
		{
			name:   "pipeline: an exclude_paths pattern that does not compile",
			err:    analyse(pipeline.Options{RepoPath: fixture("basic"), ConfigPath: badPattern}),
			class:  core.ClassUser,
			reason: core.ReasonInvalidConfiguration,
		},
		{
			name:   "pipeline: a year with too little activity",
			err:    analyse(pipeline.Options{RepoPath: fixture("basic"), Year: 1999}),
			class:  core.ClassUser,
			reason: core.ReasonYearBelowThreshold,
		},
		{
			// A failed git invocation is internal: whether it means the
			// repository should be refused is the caller's decision, and
			// classifying it here keeps git's stderr out of every artifact.
			name:   "git: an invocation that fails",
			err:    second(git.Run(t.TempDir(), "rev-parse", "--git-dir")),
			class:  core.ClassInternal,
			reason: "",
		},
		{
			name:   "render: the report cannot be written",
			err:    render.WriteReportJSON(&core.Report{}, unwritablePath(t)),
			class:  core.ClassInternal,
			reason: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				fatal(t, 41, "%s produced no error", tc.name)
			}
			if got := core.ClassOf(tc.err); got != tc.class {
				report(t, 41, "%s: class = %v, want %v (%v)", tc.name, got, tc.class, tc.err)
			}
			if got := core.ReasonOf(tc.err); got != tc.reason {
				report(t, 41, "%s: reason = %q, want %q (%v)", tc.name, got, tc.reason, tc.err)
			}
			if tc.class != core.ClassUser {
				// An internal error's contract is wrapping context, which an
				// empty message could not provide (ADR-0041 clause 3).
				if tc.err.Error() == "" {
					report(t, 41, "%s: the internal error carries no context", tc.name)
				}
				return
			}
			if !documented[string(core.ReasonOf(tc.err))] {
				report(t, 62, "%s: the reason code %q is not listed in %s section 13",
					tc.name, core.ReasonOf(tc.err), metricsCatalogue)
			}
			if core.Remedy(tc.err) == "" {
				report(t, 41, "%s: the user error carries no remedy", tc.name)
			}
		})
	}
}

// TestExitCodeAndStatusHaveOneSiteEach enforces ADR-0041 clause 4 from the
// outside: the documented values still come out of the two mappings, so the
// single construction site cannot be a mapping nobody uses.
func TestExitCodeAndStatusHaveOneSiteEach(t *testing.T) {
	t.Parallel()
	user := core.NewUserError(core.ReasonEmptyRepository, "", "Make a commit.", "the repository has no commits")
	internal := core.Internalf(nil, "writing the report")

	for _, tc := range []struct {
		name   string
		err    error
		exit   int
		status int
	}{
		{"no error", nil, 0, http.StatusOK},
		{"user error", user, 2, http.StatusBadRequest},
		{"internal error", internal, 1, http.StatusInternalServerError},
	} {
		if got := core.ExitCode(tc.err); got != tc.exit {
			report(t, 34, "%s: exit code = %d, want %d", tc.name, got, tc.exit)
		}
		if got := core.HTTPStatus(tc.err); got != tc.status {
			report(t, 41, "%s: HTTP status = %d, want %d", tc.name, got, tc.status)
		}
	}
}

// second returns the error of a two-result call, discarding the value.
func second[T any](_ T, err error) error { return err }

// unwritablePath returns a path that cannot be written on any supported
// platform, because one of its directory components is a regular file.
func unwritablePath(t *testing.T) string {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		fatal(t, 41, "writing a blocking file: %v", err)
	}
	return filepath.Join(blocker, "report.json")
}
