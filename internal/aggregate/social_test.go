package aggregate

import (
	"fmt"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/filter"
	"github.com/sinanganiz/commitography/internal/model"
)

// socialInput wires up an Input with a path filter that excludes nothing, so
// the tests measure the metric rather than the exclusion list.
func socialInput(t *testing.T) Input {
	t.Helper()
	cfg := config.Default()
	cfg.ExcludePaths = nil
	pf, err := filter.NewPathFilter(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("NewPathFilter: %v", err)
	}
	return Input{Config: cfg, PathFilter: pf}
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

func TestCouplingFindsDeliberatePairAndRejectsWeakOne(t *testing.T) {
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

	m, warnings := buildSocial(in, commits)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	var found *CoupledPair
	for i := range m.Coupling {
		p := m.Coupling[i]
		if p.A == "alpha.go" && p.B == "beta.go" {
			found = &m.Coupling[i]
		}
		if p.A == "gamma.go" || p.B == "gamma.go" {
			t.Errorf("gamma.go appears in %d commits, below the support threshold of %d, but was reported",
				4, couplingMinSupport)
		}
	}
	if found == nil {
		t.Fatalf("the deliberately coupled pair was not reported; got %+v", m.Coupling)
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
	in := socialInput(t)
	var commits []model.Commit
	for i := 0; i < 8; i++ {
		commits = append(commits, commitBy("ada@x", i, "Foo.ts", "Foo.test.ts"))
	}
	m, _ := buildSocial(in, commits)
	if len(m.Coupling) == 0 {
		t.Fatal("expected the pair to be reported, not hidden")
	}
	if !m.Coupling[0].Expected {
		t.Error("Foo.ts and Foo.test.ts share a basename and must be flagged expected")
	}
}

func TestCouplingSkipsVeryWideCommits(t *testing.T) {
	in := socialInput(t)
	wide := make([]string, filter.CouplingMaxFilesPerCommit+1)
	for i := range wide {
		wide[i] = fmt.Sprintf("file%03d.go", i)
	}

	var commits []model.Commit
	for i := 0; i < 10; i++ {
		commits = append(commits, commitBy("ada@x", i, wide...))
	}
	m, _ := buildSocial(in, commits)
	if len(m.Coupling) != 0 {
		t.Errorf("a commit touching %d files must be skipped by coupling, got %d pairs",
			len(wide), len(m.Coupling))
	}
}

func TestChurnHotspotDetection(t *testing.T) {
	in := socialInput(t)
	var commits []model.Commit
	// hot.go is touched six times inside a fortnight.
	for i := 0; i < 6; i++ {
		commits = append(commits, commitBy("ada@x", i*2, "hot.go"))
	}
	// cold.go is touched six times, but spread over a year.
	for i := 0; i < 6; i++ {
		commits = append(commits, commitBy("grace@x", i*60, "cold.go"))
	}

	m, _ := buildSocial(in, commits)
	if len(m.Churn) != 1 {
		t.Fatalf("churn = %+v, want exactly one hotspot", m.Churn)
	}
	if m.Churn[0].Path != "hot.go" {
		t.Errorf("hotspot = %q, want hot.go", m.Churn[0].Path)
	}
	if m.Churn[0].MaxCommitsInWindow != 6 {
		t.Errorf("maxCommitsInWindow = %d, want 6", m.Churn[0].MaxCommitsInWindow)
	}
	if m.Churn[0].TotalCommits != 6 {
		t.Errorf("totalCommits = %d, want 6", m.Churn[0].TotalCommits)
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

	m, _ := buildSocial(in, commits)

	byPath := map[string]DirectoryBusFactor{}
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

	shares := map[string]KnowledgeShare{}
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
	m, _ := buildSocial(in, commits)
	for _, d := range m.DirectoryBusFactor {
		if d.Path == "quiet" {
			t.Errorf("a directory with %d commits should be below the reporting threshold", directoryMinWork-1)
		}
	}
}

func TestStemOf(t *testing.T) {
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
