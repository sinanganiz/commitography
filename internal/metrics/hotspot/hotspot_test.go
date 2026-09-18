package hotspot

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

func TestChurnHotspotDetection(t *testing.T) {
	t.Parallel()
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

	var m core.SocialMetrics
	m.Churn = BuildChurn(core.ScopedCommits(in, commits))
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
