package temporal

import (
	"sort"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// BuildPerAuthor computes the opt-in per-contributor section.
func BuildPerAuthor(in core.Input, commits []model.Commit) *core.PerAuthor {
	type acc struct {
		summary core.AuthorSummary
		files   map[string]bool
		days    map[string]bool
	}
	byID := map[string]*acc{}

	for _, c := range commits {
		a, ok := byID[c.IdentityID]
		if !ok {
			a = &acc{
				summary: core.AuthorSummary{
					IdentityID:    c.IdentityID,
					HourHistogram: make([]int, 24),
				},
				files: map[string]bool{},
				days:  map[string]bool{},
			}
			if id, found := in.Resolver.Lookup(c.IdentityID); found {
				a.summary.DisplayName = id.DisplayName
				a.summary.Emails = append([]string(nil), id.Emails...)
			}
			byID[c.IdentityID] = a
		}

		when := in.Date(c)
		a.summary.Commits++
		a.summary.HourHistogram[when.Hour()]++
		a.days[when.Format(dateLayout)] = true

		if a.summary.FirstCommit.IsZero() || when.Before(a.summary.FirstCommit) {
			a.summary.FirstCommit = when
		}
		if when.After(a.summary.LastCommit) {
			a.summary.LastCommit = when
		}

		// Line counts follow the same rule as everywhere else: bulk commits are
		// left out so one vendor drop does not define a person's history.
		if c.IsBulk {
			continue
		}
		for _, f := range filter.IncludedFiles(c, in.PathFilter) {
			a.summary.Added += f.Added
			a.summary.Deleted += f.Deleted
			a.files[f.Path] = true
		}
	}

	out := make([]core.AuthorSummary, 0, len(byID))
	for _, a := range byID {
		a.summary.FilesTouched = len(a.files)
		a.summary.ActiveDays = len(a.days)
		out = append(out, a.summary)
	}

	// Sorted by arrival, not by volume. A list ordered by commit count reads as
	// a leaderboard however it is labelled.
	sort.Slice(out, func(i, j int) bool {
		if !out[i].FirstCommit.Equal(out[j].FirstCommit) {
			return out[i].FirstCommit.Before(out[j].FirstCommit)
		}
		return out[i].IdentityID < out[j].IdentityID
	})

	return &core.PerAuthor{Authors: out}
}
