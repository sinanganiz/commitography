package coupling

import (
	"fmt"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// socialInput wires up an Input with a path filter that excludes nothing, so
// the tests measure the metric rather than the exclusion list.
func socialInput(t *testing.T) core.Input {
	t.Helper()
	cfg := config.Default()
	cfg.ExcludePaths = nil
	pf, err := filter.NewPathFilter(core.SystemFilesystem(), cfg, t.TempDir())
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	return core.Input{Config: cfg, PathFilter: pf}
}

// commitBy builds a commit attributed to identityID touching the given paths.
func commitBy(identityID string, day int, paths ...string) model.Commit {
	c := model.Commit{
		Hash:       fmt.Sprintf("%s-%d", identityID, day),
		IdentityID: identityID,
		AuthorDate: time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, day),
	}
	for _, p := range paths {
		c.Files = append(c.Files, model.FileChange{Path: p, Added: 10, Deleted: 5})
	}
	c.CommitterDate = c.AuthorDate
	return c
}

func TestCouplingFindsDeliberatePairAndRejectsWeakOne(t *testing.T) {
	t.Parallel()
	in := socialInput(t)
	var commits []model.Commit
	// alpha and beta change together twelve times; gamma joins only four.
	for i := 0; i < 12; i++ {
		paths := []string{"alpha.go", "beta.go"}
		if i < 4 {
			paths = append(paths, "gamma.go")
		}
		commits = append(commits, commitBy("ada@x", i, paths...))
	}

	family, warnings := Build(core.ScopedCommits(in, commits))
	pairs := family.Metrics.Pairs
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	var found *core.CoupledPair
	for i := range pairs {
		p := pairs[i]
		if p.A == "alpha.go" && p.B == "beta.go" {
			found = &pairs[i]
		}
		if p.A == "gamma.go" || p.B == "gamma.go" {
			t.Errorf("gamma.go appears in %d commits, below the support threshold of %d, but was reported",
				4, couplingMinSupport)
		}
	}
	if found == nil {
		t.Fatalf("the deliberately coupled pair was not reported; got %+v", pairs)
	}
	if found.Support != 12 {
		t.Errorf("support = %d, want 12", found.Support)
	}
	if found.Confidence != 1.0 {
		t.Errorf("confidence = %v, want 1.0", found.Confidence)
	}
	if found.Expected {
		t.Error("alpha.go and beta.go do not share a basename and must not be flagged expected")
	}
}

func TestCouplingFlagsSameStemPairsAsExpected(t *testing.T) {
	t.Parallel()
	in := socialInput(t)
	var commits []model.Commit
	for i := 0; i < 8; i++ {
		commits = append(commits, commitBy("ada@x", i, "Foo.ts", "Foo.test.ts"))
	}
	family, _ := Build(core.ScopedCommits(in, commits))
	pairs := family.Metrics.Pairs
	if len(pairs) == 0 {
		t.Fatal("expected the pair to be reported, not hidden")
	}
	if !pairs[0].Expected {
		t.Error("Foo.ts and Foo.test.ts share a basename and must be flagged expected")
	}
}

func TestCouplingSkipsVeryWideCommits(t *testing.T) {
	t.Parallel()
	in := socialInput(t)
	wide := make([]string, filter.CouplingMaxFilesPerCommit+1)
	for i := range wide {
		wide[i] = fmt.Sprintf("file%03d.go", i)
	}

	var commits []model.Commit
	for i := 0; i < 10; i++ {
		commits = append(commits, commitBy("ada@x", i, wide...))
	}
	family, _ := Build(core.ScopedCommits(in, commits))
	pairs := family.Metrics.Pairs
	if len(pairs) != 0 {
		t.Errorf("a commit touching %d files must be skipped by coupling, got %d pairs",
			len(wide), len(pairs))
	}
}

func TestStemOf(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"src/Foo.ts":      "foo",
		"src/Foo.test.ts": "foo",
		"Makefile":        "makefile",
		"a/b/c.tar.gz":    "c",
	}
	for in, want := range cases {
		if got := stemOf(in); got != want {
			t.Errorf("stemOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCouplingDegradesPastThePairLimit is docs/metrics.md section 12: the
// pairs are cut at the catalogue's limit, and the cut is stated.
func TestCouplingDegradesPastThePairLimit(t *testing.T) {
	t.Parallel()
	in := socialInput(t)
	var commits []model.Commit
	// One more disjoint pair than the limit, each changing together exactly
	// the minimum support number of times, so every pair qualifies.
	for p := 0; p <= core.LimitCouplingPairs; p++ {
		for i := 0; i < couplingMinSupport; i++ {
			commits = append(commits, commitBy("ada@x", i, fmt.Sprintf("a%03d.go", p), fmt.Sprintf("b%03d.go", p)))
		}
	}
	family, _ := Build(core.ScopedCommits(in, commits))
	if len(family.Metrics.Pairs) != core.LimitCouplingPairs {
		t.Errorf("pairs = %d, want the limit %d", len(family.Metrics.Pairs), core.LimitCouplingPairs)
	}
	if family.Status != core.StatusDegraded || family.Reasons[0] != core.ReasonCardinalityLimit {
		t.Errorf("family = %s %v, want degraded with cardinality_limit", family.Status, family.Reasons)
	}
}
