// Package commitsize is the commit-size metric family (ADR-0024, ADR-0040):
// how large the analyzed commits are.
package commitsize

import (
	"sort"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// bulkCommitsShown caps the notable-events list. Beyond a handful, bulk commits
// stop being curiosities and start being a table.
const bulkCommitsShown = 10

// BuildCommitSize fills the commit-size figures of the code section from the
// line-scoped commits: line totals, the average and median commit size, and
// the largest commit.
func BuildCommitSize(in core.Input, lineScoped []model.Commit, m *core.CodeMetrics) {
	var sizes []int

	for _, c := range lineScoped {
		size := 0
		for _, f := range filter.IncludedFiles(c, in.PathFilter) {
			m.TotalAdded += f.Added
			m.TotalDeleted += f.Deleted
			size += f.Added + f.Deleted
		}
		if !c.IsMerge {
			sizes = append(sizes, size)
		}
	}

	m.AverageCommitSize = core.Round(core.Mean(sizes), 1)
	m.MedianCommitSize = core.Round(core.Median(sizes), 1)
	m.LargestCommit = largestCommit(in, lineScoped)
}

func largestCommit(in core.Input, commits []model.Commit) *core.CommitRef {
	var best *core.CommitRef
	for _, c := range commits {
		added, deleted, files := 0, 0, 0
		for _, f := range filter.IncludedFiles(c, in.PathFilter) {
			added += f.Added
			deleted += f.Deleted
			files++
		}
		size := added + deleted
		if best != nil && size <= best.LinesChanged {
			continue
		}
		best = &core.CommitRef{
			Hash:         c.Hash,
			Subject:      c.Subject,
			Date:         in.Date(c),
			LinesChanged: size,
			Added:        added,
			Deleted:      deleted,
			Files:        files,
		}
	}
	return best
}

// BulkCommits returns the bulk commits for the notable events, most recent
// first.
func BulkCommits(in core.Input) []filter.BulkCommit {
	// Bulk commits, most recent first. Kept as an empty array rather than null
	// so the renderer sees "none" instead of "missing".
	bulk := append([]filter.BulkCommit{}, in.Filtered.BulkCommits...)
	sort.Slice(bulk, func(i, j int) bool { return bulk[i].Date.After(bulk[j].Date) })
	if len(bulk) > bulkCommitsShown {
		bulk = bulk[:bulkCommitsShown]
	}
	return bulk
}
