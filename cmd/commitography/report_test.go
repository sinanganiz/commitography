package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

// The mount hint is keyed on the reason code, so the command knows only the two
// error classes and no per-package error type. It repeats the path only in the
// form the error carries as its offending value, which is the form the operator
// supplied (ADR-0067 clauses 3 and 5).
func TestReportExplainsMissingMountsOnlyInsideContainers(t *testing.T) {
	notRepository := core.NewUserError(core.ReasonNotARepository, "/repo",
		"Point at the directory that contains .git.", "the path is not a git repository")
	missingRoot := core.NewUserError(core.ReasonPathNotFound, "/repos",
		"Pass --allowed-root pointing at a directory that exists.", "an allowed root does not exist")

	for _, tc := range []struct {
		name        string
		err         error
		inContainer bool
		want        []string
		absent      []string
	}{
		{
			name: "native missing repository gets no hint",
			err:  notRepository,
			want: []string{"Error: the path is not a git repository (/repo)", "contains .git"},
			// The hint is a container-only explanation; nothing else changes.
			absent: []string{"hint:"},
		},
		{
			name:        "container missing repository gets a mount hint",
			err:         notRepository,
			inContainer: true,
			want: []string{
				"Error: the path is not a git repository (/repo)",
				"hint:",
				"--mount type=bind,source=<host path>,target=/repo,readonly",
				"mistyped",
			},
		},
		{
			name:   "native missing allowed root gets no hint",
			err:    missingRoot,
			want:   []string{"an allowed root does not exist (/repos)"},
			absent: []string{"hint:"},
		},
		{
			name:        "container missing allowed root gets a mount hint",
			err:         missingRoot,
			inContainer: true,
			want:        []string{"an allowed root does not exist (/repos)", "target=/repos,readonly"},
		},
		{
			name:        "a reason with no hint gets none",
			err:         collect.ShallowError("/repo"),
			inContainer: true,
			want:        []string{collect.ShallowSummary, collect.ShallowRemedy},
			absent:      []string{"hint:"},
		},
		{
			name:        "an internal error gets no hint and reveals no remedy",
			err:         core.Internalf(nil, "writing the report"),
			inContainer: true,
			want:        []string{"Error: writing the report"},
			absent:      []string{"hint:"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			report(&out, tc.err, tc.inContainer)
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output %q does not contain %q", out.String(), want)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(out.String(), absent) {
					t.Errorf("output %q contains %q", out.String(), absent)
				}
			}
		})
	}
}

// A malformed command line is a usage error, so it exits on the user code and
// not the internal one (ADR-0034 clause 5).
func TestMalformedCommandLineIsAUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"--no-such-flag"},
		{"--wrapped", "not-a-year"},
		{"one", "two"},
	} {
		err := execute(buildInfo{}, args)
		if err == nil {
			t.Fatalf("%v was accepted", args)
		}
		if got := core.ExitCode(err); got != core.ExitUser {
			t.Errorf("%v: exit code = %d, want %d", args, got, core.ExitUser)
		}
		if got := core.ReasonOf(err); got != core.ReasonInvalidInvocation {
			t.Errorf("%v: reason = %q, want %q", args, got, core.ReasonInvalidInvocation)
		}
		if core.Remedy(err) == "" {
			t.Errorf("%v: the refusal carries no remedy", args)
		}
	}
}

// --version is not a malformed command line, and neither is --help.
func TestVersionAndHelpSucceed(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--help"}} {
		if err := execute(buildInfo{version: "test"}, args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
}
