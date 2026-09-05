package filter

import (
	"time"

	"github.com/sinanganiz/commitography/internal/config"
	"github.com/sinanganiz/commitography/internal/identity"
	"github.com/sinanganiz/commitography/internal/model"
)

// CouplingMaxFilesPerCommit caps how many files a commit may touch before the
// change-coupling metric skips it. A commit touching hundreds of files couples
// everything to everything and produces noise, not signal.
const CouplingMaxFilesPerCommit = 50

// Result is the annotated commit set plus the counts needed to explain to the
// reader what was left out and why.
type Result struct {
	Commits         []model.Commit
	TotalCommits    int
	AnalyzedCommits int
	ExcludedMerges  int
	ExcludedBots    int
	BulkCommits     []BulkCommit
}

// BulkCommit is an outlier large enough to distort every line-based
// distribution: an initial import, a vendor drop, a generated-code refresh.
type BulkCommit struct {
	Hash         string    `json:"hash"`
	Subject      string    `json:"subject"`
	Date         time.Time `json:"date"`
	LinesChanged int       `json:"linesChanged"`
}

// Apply annotates commits with identity, exclusion, and bulk flags. The input
// slice is not modified; the returned commits are a copy.
func Apply(commits []model.Commit, cfg config.Config, r *identity.Resolver, pf *PathFilter) Result {
	out := make([]model.Commit, len(commits))
	copy(out, commits)

	res := Result{TotalCommits: len(out)}

	for i := range out {
		c := &out[i]
		c.IdentityID = r.Resolve(c.AuthorName, c.AuthorEmail)

		if c.IsMerge && !cfg.CountMerges {
			c.Excluded = true
			res.ExcludedMerges++
		}

		if id, ok := r.Lookup(c.IdentityID); ok && id.IsBot {
			if !c.Excluded {
				c.Excluded = true
			}
			res.ExcludedBots++
		}

		if c.Excluded {
			continue
		}
		res.AnalyzedCommits++

		// Bulk detection runs on what survives path exclusion, so a commit is
		// not condemned by a lockfile that is already being ignored.
		lines := 0
		for _, f := range c.Files {
			if f.IsBinary || pf.Excluded(f.Path) {
				continue
			}
			lines += f.Added + f.Deleted
		}
		if lines > cfg.OutlierThresholdLines {
			c.IsBulk = true
			res.BulkCommits = append(res.BulkCommits, BulkCommit{
				Hash:         c.Hash,
				Subject:      c.Subject,
				Date:         CommitDate(*c, cfg),
				LinesChanged: lines,
			})
		}
	}

	res.Commits = out
	return res
}

// CommitDate returns the timestamp a commit is attributed to, honouring the
// configured date source. Author date is the default because rebase rewrites
// the committer date and squash destroys it entirely.
func CommitDate(c model.Commit, cfg config.Config) time.Time {
	if cfg.DateSource == config.DateSourceCommitter {
		return c.CommitterDate
	}
	return c.AuthorDate
}

// IncludedFiles returns a commit's file changes with excluded paths removed.
func IncludedFiles(c model.Commit, pf *PathFilter) []model.FileChange {
	out := make([]model.FileChange, 0, len(c.Files))
	for _, f := range c.Files {
		if pf.Excluded(f.Path) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// CountsForLines reports whether a commit contributes to line-based, coupling
// and churn metrics. Bulk commits still count toward commit and temporal
// metrics, so they are only filtered out here.
func CountsForLines(c model.Commit) bool { return !c.Excluded && !c.IsBulk }
