// Package commitsize is the commit-size metric family (ADR-0024, ADR-0040):
// how large the analysed commits are. Its metrics are those of
// docs/metrics.md section 3 (ADR-0062); a metric that section does not define
// is not computed here.
package commitsize

import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// dateLayout is the calendar-date form used everywhere in the report.
const dateLayout = "2006-01-02"

// version is the family version (ADR-0031 clause 2).
func version() core.Version { return core.Version{Major: 1, Minor: 0} }

// Build computes the commit-size family over the line-scoped commits: the
// analysed commits minus bulk commits. With none, every metric is absent and
// the family is degraded with empty_population.
func Build(in core.Input, lineScoped []model.Commit) core.Family[core.CommitSizeMetrics] {
	if len(lineScoped) == 0 {
		f := core.Computed(version(), core.CommitSizeMetrics{})
		f.Degrade(core.ReasonEmptyPopulation, core.ConfidencePartial)
		return f
	}

	var sizes []int
	for _, c := range lineScoped {
		size := 0
		for _, f := range filter.IncludedFiles(c, in.PathFilter) {
			size += f.Added + f.Deleted
		}
		if !c.IsMerge {
			sizes = append(sizes, size)
		}
	}

	mean := core.Round(core.Mean(sizes), 1)
	median := core.Round(core.Median(sizes), 1)
	return core.Computed(version(), core.CommitSizeMetrics{
		MeanLines:     &mean,
		MedianLines:   &median,
		LargestCommit: largestCommit(in, lineScoped),
	})
}

func largestCommit(in core.Input, commits []model.Commit) *core.LargestCommit {
	var best *core.LargestCommit
	for _, c := range commits {
		lines, files := 0, 0
		for _, f := range filter.IncludedFiles(c, in.PathFilter) {
			lines += f.Added + f.Deleted
			files++
		}
		if best != nil && lines <= best.Lines {
			continue
		}
		best = &core.LargestCommit{
			Hash:  c.Hash,
			Date:  in.Date(c).Format(dateLayout),
			Lines: lines,
			Files: files,
		}
	}
	return best
}
