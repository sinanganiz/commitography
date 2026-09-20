package git

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

// repositoryRoot is this checkout, which every test here reads as an
// ordinary repository.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// ResolveDate reads "now" from its caller, so a bare date, which git completes
// with the time of day, resolves differently at a different hour, while an
// instant resolves to itself whatever the hour. The second property is what
// makes a resolved bound reproducible (ADR-0026 clause 4).
func TestResolveDateReadsNowFromTheCaller(t *testing.T) {
	t.Parallel()
	repo := repositoryRoot(t)
	morning := time.Date(2026, 3, 4, 6, 0, 0, 0, time.UTC)
	evening := morning.Add(9 * time.Hour)

	resolve := func(bound, expression string, now time.Time) time.Time {
		t.Helper()
		at, err := ResolveDate(context.Background(), repo, bound, expression, now)
		if err != nil {
			t.Fatalf("ResolveDate(%s, %q): %v", bound, expression, err)
		}
		return at
	}

	early := resolve("since", "2026-02-01", morning)
	late := resolve("since", "2026-02-01", evening)
	if early.Equal(late) {
		t.Errorf("the bare date resolved to %s at both hours; git completes it with the time of day", early)
	}
	if !early.Before(late) {
		t.Errorf("the earlier hour resolved to %s, the later to %s", early, late)
	}

	instant := "2026-02-01T09:30:00Z"
	for _, now := range []time.Time{morning, evening} {
		got := resolve("since", instant, now)
		if want := time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC); !got.Equal(want) {
			t.Errorf("the instant %s resolved to %s at %s, want %s", instant, got, now, want)
		}
	}
	// The two bounds are read from different rev-parse output, so both forms
	// are exercised.
	if got := resolve("until", instant, morning); !got.Equal(time.Date(2026, 2, 1, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("the until bound resolved to %s, want the instant itself", got)
	}
	// A relative expression is resolved against the caller's now as well.
	if got := resolve("since", "2 days ago", morning); !got.Equal(morning.AddDate(0, 0, -2)) {
		t.Errorf("two days before %s resolved to %s", morning, got)
	}
}

func TestResolveDateRejectsAnUnknownBound(t *testing.T) {
	t.Parallel()
	if _, err := ResolveDate(context.Background(), "", "between", "2026-02-01", time.Unix(0, 0)); err == nil {
		t.Error("a bound that is neither since nor until was accepted")
	}
}

// TestRecordsAreNulDelimited covers the output half of ADR-0065 clause 2 on a
// real invocation: the tracked file list arrives as records, not as lines.
func TestRecordsAreNulDelimited(t *testing.T) {
	t.Parallel()
	repo := repositoryRoot(t)
	records, err := Records(context.Background(), At(repo, "ls-files", "-z").Pathspecs())
	if err != nil {
		t.Fatalf("Records: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("ADR-0064: the repository listed no tracked files, so the checker cannot fail")
	}
	for _, record := range records {
		if strings.Contains(record, "\n") {
			t.Errorf("the record %q spans a line break, so the output was split on the wrong byte", record)
		}
	}
	if !contains(records, "go.mod") {
		t.Errorf("the tracked file list does not contain go.mod, so it is not the repository's own")
	}
}

// TestOutputIsBounded covers the size limit of ADR-0065 clause 2: a read past
// the limit is refused rather than grown.
func TestOutputIsBounded(t *testing.T) {
	t.Parallel()
	repo := repositoryRoot(t)
	spec := At(repo, "ls-files", "-z").Pathspecs()
	spec.Limit = 16
	if _, err := Output(context.Background(), spec); err == nil {
		t.Error("an invocation producing more than its limit returned no error")
	}
	if _, err := Records(context.Background(), spec); err == nil {
		t.Error("a record read past its limit returned no error")
	}
}

// TestSplitRecordsKeepsTheSeparatorsGitProduces pins the framing the collect
// stage relies on: an empty record between two terminators is a record, not
// something to skip, because that is how git separates commits.
func TestSplitRecordsKeepsTheSeparatorsGitProduces(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{in: "a\x00b\x00", want: []string{"a", "b"}},
		{in: "a\x00\x00b\x00", want: []string{"a", "", "b"}},
		{in: "a\nb\x00", want: []string{"a\nb"}},
		{in: "trailing", want: []string{"trailing"}},
	} {
		got := splitAll(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("%q split into %q, want %q", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%q split into %q, want %q", tc.in, got, tc.want)
				break
			}
		}
	}
}

// splitAll runs the package's split function over a whole string.
func splitAll(in string) []string {
	var out []string
	data := []byte(in)
	for len(data) > 0 {
		advance, token, _ := splitRecords(data, true)
		if advance == 0 {
			break
		}
		out = append(out, string(token))
		data = data[advance:]
	}
	return out
}

// TestFailureClassification enforces WP-0011 clause 5 over the conditions the
// classifier recognises. The phrases are git's own, in the C locale the
// hardening set pins, so they are checked as data rather than reproduced by
// arranging each condition.
func TestFailureClassification(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		stderr string
		class  core.Class
		reason core.Reason
	}{
		{
			name:   "a path that is not a repository",
			stderr: "fatal: not a git repository (or any of the parent directories): .git",
			class:  core.ClassUser,
			reason: core.ReasonNotARepository,
		},
		{
			name:   "a remote that is not a repository",
			stderr: "fatal: repository 'x' does not appear to be a git repository",
			class:  core.ClassUser,
			reason: core.ReasonNotARepository,
		},
		{
			name:   "a shallow history",
			stderr: "fatal: attempt to fetch/clone from a shallow repository",
			class:  core.ClassUser,
			reason: core.ReasonShallowClone,
		},
		{
			name:   "a credential the environment did not provide",
			stderr: "fatal: could not read Username for 'https://example.com': terminal prompts disabled",
			class:  core.ClassInternal,
			reason: "",
		},
		{
			name:   "anything else",
			stderr: "fatal: bad object 0000000000000000000000000000000000000000",
			class:  core.ClassInternal,
			reason: "",
		},
		{
			name:   "no diagnostic at all",
			stderr: "",
			class:  core.ClassInternal,
			reason: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := classify(At("", "rev-parse"), tc.stderr, errors.New("exit status 128"))
			if got := core.ClassOf(err); got != tc.class {
				t.Errorf("class = %v, want %v (%v)", got, tc.class, err)
			}
			if got := core.ReasonOf(err); got != tc.reason {
				t.Errorf("reason = %q, want %q (%v)", got, tc.reason, err)
			}
			if tc.class == core.ClassUser && core.Remedy(err) == "" {
				t.Errorf("a user error carries no remedy (ADR-0041 clause 2)")
			}
			// git's own words locate the failure but may name a path, so
			// they stay in the cause and never in the rendering
			// (ADR-0067 clause 2).
			if tc.stderr != "" && strings.Contains(core.Artifact(err), tc.stderr) {
				t.Errorf("the artifact rendering repeats git's standard error")
			}
		})
	}
}

// TestFailedInvocationIsClassified is the same property end to end, on the
// one condition that is reachable without a remote.
func TestFailedInvocationIsClassified(t *testing.T) {
	t.Parallel()
	_, err := Output(context.Background(), At(t.TempDir(), "rev-parse", "--git-dir"))
	if err == nil {
		t.Fatal("a directory that is not a repository was accepted")
	}
	if got := core.ReasonOf(err); got != core.ReasonNotARepository {
		t.Errorf("reason = %q, want %q (%v)", got, core.ReasonNotARepository, err)
	}
	if core.ClassOf(err) != core.ClassUser {
		t.Errorf("class = %v, want the operator's (%v)", core.ClassOf(err), err)
	}
}
