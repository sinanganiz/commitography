package core

import (
	"context"
	"time"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// Input is everything the aggregation stage needs. It is assembled by the CLI
// once collection and filtering have run. The metric families read it too,
// which is why it lives here rather than in the aggregation stage (ADR-0040
// clause 4).
type Input struct {
	Context    context.Context
	RepoPath   string
	Repository model.RepositoryInfo
	Config     config.Analysis
	Filtered   filter.Result
	Resolver   *identity.Resolver
	PathFilter *filter.PathFilter
	// Replay is the replay stage's state: the analysed commit's tree and its
	// line ownership (ADR-0020 clause 3). Aggregation reads the tree from here
	// and never lists it itself.
	Replay *ReplayState

	// Progress, when set, is called as each stage begins.
	Progress func(stage, detail string, current, total int)

	// ToolVersion is the build's version, recorded in the report's generation
	// metadata (ADR-0061 clause 6).
	ToolVersion string
}

// Analyzed returns the commits that count toward commit and temporal metrics:
// everything not excluded, bulk commits included.
func (in Input) Analyzed() []model.Commit {
	out := make([]model.Commit, 0, len(in.Filtered.Commits))
	for _, c := range in.Filtered.Commits {
		if c.Excluded {
			continue
		}
		// The year is an analysis value like any other (ADR-0026 clause 1): it
		// decides which commits are analysed, so every metric depends on it.
		if in.Config.Year != 0 && in.Date(c).Year() != in.Config.Year {
			continue
		}
		out = append(out, c)
	}
	return out
}

// LineScoped returns the commits that count toward line-based, coupling and
// churn metrics: analyzed commits minus bulk outliers.
func (in Input) LineScoped() []model.Commit {
	out := make([]model.Commit, 0, len(in.Filtered.Commits))
	for _, c := range in.Analyzed() {
		if c.IsBulk {
			continue
		}
		out = append(out, c)
	}
	return out
}

// Date returns the timestamp a commit is attributed to, in the author's own
// timezone. Hour-of-day analysis in UTC is meaningless, so the offset git
// recorded is preserved all the way through.
func (in Input) Date(c model.Commit) time.Time {
	return filter.CommitDate(c, in.Config)
}
