package pipeline

import (
	"testing"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
)

// analyze runs the full identity + filter pipeline over a fixture.
func analyze(t *testing.T, name string, cfg config.Config) (filter.Result, *filter.PathFilter) {
	t.Helper()
	repo := fixture(t, name)
	h, err := newCollector().Collect(collect.Options{RepoPath: repo, UseMailmap: cfg.UseMailmap})
	if err != nil {
		t.Fatalf("Collect(%s): %v", name, err)
	}
	pf, err := filter.NewPathFilter(core.SystemFilesystem(), cfg, repo)
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	r := identity.NewResolver(cfg, h.Commits)
	return filter.Apply(h.Commits, cfg, r, pf), pf
}

func TestLockfileContributesNoLines(t *testing.T) {
	res, pf := analyze(t, "noise", config.Default())

	var lockfileLines, includedLines int
	for _, c := range res.Commits {
		for _, f := range c.Files {
			if f.Path == "package-lock.json" {
				lockfileLines += f.Added
			}
		}
		if !filter.CountsForLines(c) {
			continue
		}
		for _, f := range filter.IncludedFiles(c, pf) {
			includedLines += f.Added
		}
	}

	if lockfileLines < 39000 {
		t.Fatalf("fixture lockfile added only %d lines; expected roughly 40,000", lockfileLines)
	}
	if pf.Excluded("package-lock.json") != true {
		t.Fatal("package-lock.json is not excluded by default")
	}
	if includedLines >= lockfileLines {
		t.Errorf("aggregate added lines (%d) still include the lockfile (%d)", includedLines, lockfileLines)
	}
}

func TestMergesAreExcludedByDefault(t *testing.T) {
	res, _ := analyze(t, "merges", config.Default())
	if res.ExcludedMerges != 5 {
		t.Errorf("ExcludedMerges = %d, want 5", res.ExcludedMerges)
	}
	for _, c := range res.Commits {
		if c.IsMerge && !c.Excluded {
			t.Errorf("merge commit %s was not excluded", c.Hash)
		}
	}
}

func TestCountMergesKeepsMerges(t *testing.T) {
	cfg := config.Default()
	cfg.CountMerges = true
	res, _ := analyze(t, "merges", cfg)
	if res.ExcludedMerges != 0 {
		t.Errorf("ExcludedMerges = %d, want 0 when count_merges is set", res.ExcludedMerges)
	}
}

func TestBulkCommitIsFlagged(t *testing.T) {
	res, _ := analyze(t, "noise", config.Default())

	if len(res.BulkCommits) == 0 {
		t.Fatal("no bulk commit detected in the noise fixture")
	}
	found := false
	for _, c := range res.Commits {
		if c.IsBulk {
			found = true
			if c.Excluded {
				t.Error("a bulk commit must stay in commit and temporal metrics")
			}
			if filter.CountsForLines(c) {
				t.Error("a bulk commit must be kept out of line-based metrics")
			}
		}
	}
	if !found {
		t.Error("expected the lockfile-and-data-table commit to be flagged as bulk")
	}
}

func TestBotCommitsAreExcluded(t *testing.T) {
	res, _ := analyze(t, "bots", config.Default())
	if res.ExcludedBots != 2 {
		t.Errorf("ExcludedBots = %d, want 2", res.ExcludedBots)
	}
	if res.AnalyzedCommits != res.TotalCommits-2 {
		t.Errorf("analyzed %d of %d commits, want %d", res.AnalyzedCommits, res.TotalCommits, res.TotalCommits-2)
	}
	for _, c := range res.Commits {
		if c.IdentityID == "" {
			t.Errorf("commit %s was not assigned an identity", c.Hash)
		}
	}
}
