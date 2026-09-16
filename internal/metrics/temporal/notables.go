package temporal

import (
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// BuildNotables computes the notable events over the analyzed commits. The
// bulk commit list is the commit-size family's and is filled in by the
// aggregation stage.
func BuildNotables(in core.Input, commits []model.Commit) core.Notables {
	n := core.Notables{
		TimezoneSpread: []core.TimezoneShare{},
	}

	// Merges are reported whatever count_merges is set to: the number is a
	// fact about the repository, not about the analysis.
	for _, c := range in.Filtered.Commits {
		if c.IsMerge {
			n.MergeCount++
		}
	}

	if len(commits) == 0 {
		return n
	}

	weekend := 0
	offsets := map[int]int{}
	var root model.Commit
	haveRoot := false

	for _, c := range commits {
		t := in.Date(c)

		if weekdayIndex(t) >= 5 {
			weekend++
		}
		if (t.Month() == time.December && t.Day() == 25) ||
			(t.Month() == time.January && t.Day() == 1) {
			n.HolidayCommits++
		}
		offsets[c.AuthorTZOffsetMinutes]++

		// The root commit. A history can have several roots, and a history
		// narrowed by --since may have none, so prefer a parentless commit and
		// fall back to the earliest one.
		if !haveRoot || betterRoot(c, root, t, in.Date(root)) {
			root, haveRoot = c, true
		}

		hour := t.Hour()
		if hour < 6 {
			if n.LatestNightCommit == nil || closerToThree(t, n.LatestNightCommit.Date) {
				n.LatestNightCommit = commitRef(in, c)
			}
		}
		if hour >= 4 && hour < 8 {
			if n.EarliestMorningCommit == nil || localTimeOfDay(t) < localTimeOfDay(n.EarliestMorningCommit.Date) {
				n.EarliestMorningCommit = commitRef(in, c)
			}
		}
	}

	n.WeekendRatio = core.Round(float64(weekend)/float64(len(commits)), 4)

	if haveRoot {
		subject := root.Subject
		n.FirstCommitSubject = &subject
	}

	for offset, count := range offsets {
		n.TimezoneSpread = append(n.TimezoneSpread, core.TimezoneShare{OffsetMinutes: offset, Commits: count})
	}
	sort.Slice(n.TimezoneSpread, func(i, j int) bool {
		return n.TimezoneSpread[i].OffsetMinutes < n.TimezoneSpread[j].OffsetMinutes
	})

	return n
}

// betterRoot reports whether candidate should replace current as the commit
// the history starts from. A parentless commit always wins over one with
// parents; between equals, the earlier one wins.
func betterRoot(candidate, current model.Commit, candidateAt, currentAt time.Time) bool {
	candidateIsRoot := len(candidate.Parents) == 0
	currentIsRoot := len(current.Parents) == 0
	if candidateIsRoot != currentIsRoot {
		return candidateIsRoot
	}
	return candidateAt.Before(currentAt)
}

// localTimeOfDay is minutes past local midnight.
func localTimeOfDay(t time.Time) int { return t.Hour()*60 + t.Minute() }

// closerToThree reports whether a is nearer to 03:00 local than b, which is the
// deepest point of the night and so the most notable hour to be committing.
func closerToThree(a, b time.Time) bool {
	const threeAM = 3 * 60
	da := localTimeOfDay(a) - threeAM
	db := localTimeOfDay(b) - threeAM
	if da < 0 {
		da = -da
	}
	if db < 0 {
		db = -db
	}
	return da < db
}

func commitRef(in core.Input, c model.Commit) *core.CommitRef {
	ref := &core.CommitRef{
		Hash:    c.Hash,
		Subject: c.Subject,
		Date:    in.Date(c),
	}
	for _, f := range filter.IncludedFiles(c, in.PathFilter) {
		ref.Added += f.Added
		ref.Deleted += f.Deleted
		ref.Files++
	}
	ref.LinesChanged = ref.Added + ref.Deleted
	return ref
}
