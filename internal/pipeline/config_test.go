package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

// runWith analyses the basic fixture under one configuration file and returns
// the result.
func runWith(t *testing.T, body string, opts Options) *Result {
	t.Helper()
	path := filepath.Join(t.TempDir(), "explicit.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	opts.RepoPath = fixture(t, "basic")
	opts.ConfigPath = path
	result, err := newAnalyzer().Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("analysing under %q: %v", strings.TrimSpace(body), err)
	}
	return result
}

// The date bounds and the year are analysis values: set in a file, they decide
// which commits are analysed, exactly as the flags do.
func TestDateBoundsAndYearFromTheConfigurationFile(t *testing.T) {
	t.Parallel()
	all := runWith(t, "", Options{})
	bounded := runWith(t, "since: 2026-01-01T00:00:00Z\n", Options{})
	first := func(r *Result) string {
		t.Helper()
		if r.Report.Families.Temporal.Metrics.FirstCommitDate == nil {
			t.Fatal("the temporal family reported no first commit date, so the bound cannot be observed")
		}
		return *r.Report.Families.Temporal.Metrics.FirstCommitDate
	}
	if first(bounded) <= first(all) {
		t.Errorf("a lower date bound of 2026-01-01 left the first commit at %s, and the unbounded run starts %s",
			first(bounded), first(all))
	}

	year := runWith(t, "year: 2025\n", Options{})
	if year.Analysis.Year != 2025 {
		t.Errorf("the resolved year is %d, want 2025 from the file", year.Analysis.Year)
	}
	if first := year.Report.Identities[0].FirstCommitDate; !strings.HasPrefix(first, "2025") {
		t.Errorf("the year filtered nothing: the first identity starts %s", first)
	}
	// An explicit parameter replaces the file's value (ADR-0026 clause 6).
	both := runWith(t, "year: 2025\nsince: 2019-01-01T00:00:00Z\n", Options{Year: 2026, Since: "2020-01-01T00:00:00Z"})
	if both.Analysis.Year != 2026 || both.Analysis.Since != "2020-01-01T00:00:00Z" {
		t.Errorf("the parameters did not replace the file: year %d, since %q", both.Analysis.Year, both.Analysis.Since)
	}
}

// A bound reaches the report as the instant git resolved it to, not as the
// words the operator wrote, because git completes a bare date with the time of
// day (ADR-0026 clause 4).
func TestDateBoundsAreResolvedToAnInstant(t *testing.T) {
	t.Parallel()
	result := runWith(t, "since: 2026-01-01\nuntil: 2026-02-01\n", Options{})
	for _, bound := range []struct{ name, value string }{
		{"since", result.Analysis.Since},
		{"until", result.Analysis.Until},
	} {
		at, err := time.Parse(time.RFC3339, bound.value)
		if err != nil {
			t.Errorf("the resolved %s is %q, which is not an instant: %v", bound.name, bound.value, err)
			continue
		}
		// The collector's clock reads testTime, and git completes a bare date
		// with the hour of that clock.
		if got, want := at.Hour(), testTime().Hour(); got != want {
			t.Errorf("the resolved %s is at hour %d, and the injected clock reads %d", bound.name, got, want)
		}
	}
	if result.Analysis.Since == "" || result.Analysis.Until == "" {
		t.Error("a bound resolved to nothing")
	}

	// No bound stays no bound: an empty expression must never reach git, which
	// would read it as now.
	unbounded := runWith(t, "", Options{})
	if unbounded.Analysis.Since != "" || unbounded.Analysis.Until != "" {
		t.Errorf("unset bounds resolved to %q and %q", unbounded.Analysis.Since, unbounded.Analysis.Until)
	}
}

// Cardinality limits are analysis-plane and verified, never applied.
func TestCardinalityLimitsAreVerifiedNotApplied(t *testing.T) {
	t.Parallel()
	agreeing := runWith(t, "cardinality_limits:\n  coupling_pairs: 200\n", Options{})
	if got := agreeing.Analysis.CardinalityLimits["coupling_pairs"]; got != core.LimitCouplingPairs {
		t.Errorf("the resolved coupling pair limit is %d, want the catalogue's %d", got, core.LimitCouplingPairs)
	}
	if len(agreeing.Analysis.CardinalityLimits) != len(core.LimitValues()) {
		t.Errorf("the resolved limits are %v, want every limit of the catalogue", agreeing.Analysis.CardinalityLimits)
	}

	path := filepath.Join(t.TempDir(), "explicit.yml")
	if err := os.WriteFile(path, []byte("cardinality_limits:\n  coupling_pairs: 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := newAnalyzer().Run(context.Background(), Options{
		RepoPath: fixture(t, "basic"), ConfigPath: path, OperatorSupplied: true,
	}, nil)
	if err == nil {
		t.Fatal("a limit that disagrees with the catalogue was accepted")
	}
	if got := core.ReasonOf(err); got != core.ReasonInvalidConfiguration {
		t.Errorf("reason = %q, want invalid_configuration", got)
	}
}
