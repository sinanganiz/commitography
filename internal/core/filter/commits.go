package filter

import (
	"time"

	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/identity"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// CouplingMaxFilesPerCommit caps how many files a commit may touch before the
// change-coupling metric skips it. A commit touching hundreds of files couples
// everything to everything and produces noise, not signal.
const CouplingMaxFilesPerCommit = 50

// dateLayout is the calendar-date form the report writes a date in.
const dateLayout = "2006-01-02"

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

// Annotate decides, for every commit, the per-commit definitions of
// docs/metrics.md section 1 and writes them into the record: its identity,
// analysed-commit membership and the reasons against it, each file's excluded
// path, effective lines, the bulk flag, local time and the active date. The
// collect stage calls it once (WP-0012); a later stage reads what it wrote.
// The input slice is not modified; the returned commits are a copy, files
// included.
func Annotate(commits []model.Commit, cfg config.Analysis, r *identity.Resolver, pf *PathFilter) []model.Commit {
	out := make([]model.Commit, len(commits))
	copy(out, commits)

	for i := range out {
		c := &out[i]
		c.Files = append([]model.FileChange(nil), c.Files...)
		c.IdentityID = r.Resolve(c.AuthorName, c.AuthorEmail)

		// Membership. Author exclusion is decided per resolved identity, which
		// is why the resolver is needed here at all: an identity is excluded
		// when its name or any address folded into it matches.
		c.MergeExcluded = c.IsMerge && !cfg.CountMerges
		c.AuthorExcluded = false
		if id, ok := r.Lookup(c.IdentityID); ok && id.IsBot {
			c.AuthorExcluded = true
		}
		c.Excluded = c.MergeExcluded || c.AuthorExcluded

		// Excluded paths and effective lines. A binary change carries no line
		// counts, so leaving it out changes no sum; it is left out anyway, so
		// the rule does not depend on how git reports one.
		c.EffectiveLines = 0
		for j := range c.Files {
			f := &c.Files[j]
			f.Excluded = pf.Excluded(f.Path)
			if f.Excluded || f.IsBinary {
				continue
			}
			c.EffectiveLines += f.Added + f.Deleted
		}

		// Bulk detection runs on what survives path exclusion, so a commit is
		// not condemned by a lockfile that is already being ignored.
		c.IsBulk = !c.Excluded && c.EffectiveLines > cfg.OutlierThresholdLines

		c.LocalTime = CommitDate(*c, cfg)
		c.ActiveDate = c.LocalTime.Format(dateLayout)
	}
	return out
}

// Summarize counts what annotated records say about themselves. It decides
// nothing: every value it reads was written by Annotate.
func Summarize(commits []model.Commit) Result {
	res := Result{Commits: commits, TotalCommits: len(commits)}
	for _, c := range commits {
		if c.MergeExcluded {
			res.ExcludedMerges++
		}
		if c.AuthorExcluded {
			res.ExcludedBots++
		}
		if c.Excluded {
			continue
		}
		res.AnalyzedCommits++
		if c.IsBulk {
			res.BulkCommits = append(res.BulkCommits, BulkCommit{
				Hash:         c.Hash,
				Subject:      c.Subject,
				Date:         c.LocalTime,
				LinesChanged: c.EffectiveLines,
			})
		}
	}
	return res
}

// Apply annotates commits and counts the result: Annotate followed by
// Summarize. The input slice is not modified; the returned commits are a copy.
func Apply(commits []model.Commit, cfg config.Analysis, r *identity.Resolver, pf *PathFilter) Result {
	return Summarize(Annotate(commits, cfg, r, pf))
}

// CommitDate returns the timestamp a commit is attributed to, honouring the
// configured date source. Author date is the default because rebase rewrites
// the committer date and squash destroys it entirely.
func CommitDate(c model.Commit, cfg config.Analysis) time.Time {
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
