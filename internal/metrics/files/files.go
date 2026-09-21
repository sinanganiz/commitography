// Package files is the files metric family (ADR-0024, ADR-0040): which files
// the history keeps returning to. Its metrics are those of docs/metrics.md
// section 5 (ADR-0062), and its list limits are the catalogue's, taken from
// core (ADR-0053 clause 3).
package files

import (
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// dateLayout is the calendar-date form used everywhere in the report.
const dateLayout = "2006-01-02"

// version is the family version (ADR-0031 clause 2).
func version() core.Version { return core.Version{Major: 2, Minor: 0} }

// fileStat accumulates per-path activity across the line-scoped commit set.
type fileStat struct {
	commits      int
	lastModified time.Time
}

// Tree is what the family needs from the analysed commit's tree: every
// tracked path, and the count of tracked text files among those that survive
// path exclusion.
type Tree struct {
	Tracked       []string
	TextFileCount int
}

// Included returns the tracked paths that survive path exclusion.
func Included(in core.Input, tracked []string) []string {
	out := make([]string, 0, len(tracked))
	for _, p := range tracked {
		if !in.PathFilter.Excluded(p) {
			out = append(out, p)
		}
	}
	return out
}

// Build computes the files family from the line-scoped commits and the tree
// at the analysed commit. Exceeding the most-modified limit degrades the
// family with cardinality_limit.
func Build(in core.Input, lineScoped []model.Commit, tree Tree) core.Family[core.FilesMetrics] {
	stats := map[string]*fileStat{}
	for _, c := range lineScoped {
		when := in.Date(c)
		for _, f := range filter.IncludedFiles(c, in.PathFilter) {
			st, ok := stats[f.Path]
			if !ok {
				st = &fileStat{}
				stats[f.Path] = st
			}
			st.commits++
			if when.After(st.lastModified) {
				st.lastModified = when
			}
		}
	}

	inHead := make(map[string]bool, len(tree.Tracked))
	for _, p := range tree.Tracked {
		inHead[p] = true
	}

	mostModified, truncated := mostModified(stats, inHead)
	headFiles, textFiles := len(tree.Tracked), tree.TextFileCount
	f := core.Computed(version(), core.FilesMetrics{
		MostModified:         mostModified,
		OldestUntouched:      oldestUntouched(Included(in, tree.Tracked), stats),
		HeadFileCount:        &headFiles,
		TrackedTextFileCount: &textFiles,
	})
	if truncated {
		f.Degrade(core.ReasonCardinalityLimit, core.ConfidencePartial)
	}
	return f
}

// mostModified ranks paths by how many commits touched them. Paths since
// removed from the analysed commit stay in the ranking, flagged, because
// "what did this team keep returning to" is a historical question. It reports
// whether the ranking was cut at its limit.
func mostModified(stats map[string]*fileStat, inHead map[string]bool) ([]core.ModifiedPath, bool) {
	out := make([]core.ModifiedPath, 0, len(stats))
	for p, st := range stats {
		out = append(out, core.ModifiedPath{Path: p, Commits: st.commits, Removed: !inHead[p]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Commits != out[j].Commits {
			return out[i].Commits > out[j].Commits
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > core.LimitMostModifiedFiles {
		return out[:core.LimitMostModifiedFiles], true
	}
	return out, false
}

// oldestUntouched is the file still present at the analysed commit whose most
// recent modifying commit is the oldest: the part of the codebase nobody has
// needed to think about in the longest time.
func oldestUntouched(inHead []string, stats map[string]*fileStat) *core.PathDate {
	var best string
	var bestAt time.Time
	for _, p := range inHead {
		st, ok := stats[p]
		if !ok || st.lastModified.IsZero() {
			continue
		}
		if best == "" || st.lastModified.Before(bestAt) || (st.lastModified.Equal(bestAt) && p < best) {
			best, bestAt = p, st.lastModified
		}
	}
	if best == "" {
		return nil
	}
	return &core.PathDate{Path: best, Date: bestAt.Format(dateLayout)}
}
