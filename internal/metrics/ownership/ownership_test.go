package ownership

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

func TestBusFactorOfDominatedRepositoryIsOne(t *testing.T) {
	counts := map[string]int{"a@x": 80, "b@x": 10, "c@x": 5, "d@x": 5}
	if got := busFactor(counts); got != 1 {
		t.Errorf("busFactor = %d, want 1 when one identity authored 80%%", got)
	}
}

func TestBusFactorOfEvenlySplitRepositoryIsTwo(t *testing.T) {
	counts := map[string]int{"a@x": 25, "b@x": 25, "c@x": 25, "d@x": 25}
	if got := busFactor(counts); got != 2 {
		t.Errorf("busFactor = %d, want 2 for four identities at 25%% each", got)
	}
}

func TestBusFactorEdgeCases(t *testing.T) {
	if got := busFactor(nil); got != 0 {
		t.Errorf("busFactor(nil) = %d, want 0", got)
	}
	if got := busFactor(map[string]int{"solo@x": 12}); got != 1 {
		t.Errorf("busFactor of a single contributor = %d, want 1", got)
	}
	// Exactly half from the leader still satisfies "at least 50%".
	if got := busFactor(map[string]int{"a@x": 5, "b@x": 3, "c@x": 2}); got != 1 {
		t.Errorf("busFactor = %d, want 1 when the leader holds exactly half", got)
	}
}

func TestDirectoryBusFactorAndKnowledgeConcentration(t *testing.T) {
	in := socialInput(t)
	var commits []model.Commit
	// One person owns src/ entirely; two share web/.
	for i := 0; i < 12; i++ {
		commits = append(commits, commitBy("ada@x", i, "src/core/main.go"))
	}
	for i := 0; i < 12; i++ {
		who := "grace@x"
		if i%2 == 0 {
			who = "alan@x"
		}
		commits = append(commits, commitBy(who, 100+i, "web/src/app.ts"))
	}

	var m core.SocialMetrics
	BuildOwnership(in, core.ScopedCommits(in, commits), &m)

	byPath := map[string]core.DirectoryBusFactor{}
	for _, d := range m.DirectoryBusFactor {
		byPath[d.Path] = d
	}
	if got := byPath["src"].BusFactor; got != 1 {
		t.Errorf("src bus factor = %d, want 1", got)
	}
	if got := byPath["web"].BusFactor; got != 1 {
		t.Errorf("web bus factor = %d, want 1 (either of two equal contributors covers half)", got)
	}
	if _, ok := byPath["src/core"]; !ok {
		t.Error("depth-2 directories should be reported too")
	}

	shares := map[string]core.KnowledgeShare{}
	for _, k := range m.KnowledgeConcentration {
		shares[k.Path] = k
	}
	if shares["src"].LargestShare != 1.0 {
		t.Errorf("src concentration = %v, want 1.0", shares["src"].LargestShare)
	}
	if shares["web"].LargestShare != 0.5 {
		t.Errorf("web concentration = %v, want 0.5", shares["web"].LargestShare)
	}
	if shares["src"].TopContributor != "" {
		t.Error("the leading contributor must not be named without --per-author")
	}
	if _, present := shares["src/core"]; present {
		t.Error("knowledge concentration is a depth-1 metric only")
	}
}

func TestDirectoriesBelowActivityThresholdAreOmitted(t *testing.T) {
	in := socialInput(t)
	var commits []model.Commit
	for i := 0; i < directoryMinWork-1; i++ {
		commits = append(commits, commitBy("ada@x", i, "quiet/file.go"))
	}
	var m core.SocialMetrics
	BuildOwnership(in, core.ScopedCommits(in, commits), &m)
	for _, d := range m.DirectoryBusFactor {
		if d.Path == "quiet" {
			t.Errorf("a directory with %d commits should be below the reporting threshold", directoryMinWork-1)
		}
	}
}
