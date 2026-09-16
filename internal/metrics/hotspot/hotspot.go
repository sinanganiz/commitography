// Package hotspot is the hotspot metric family (ADR-0024, ADR-0040): files
// rewritten repeatedly inside a short window.
package hotspot

import (
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

const (
	// churnWindowDays and churnMinCommits define a hotspot: a file rewritten
	// repeatedly inside one month.
	churnWindowDays = 30
	churnMinCommits = 5
	churnLimit      = 25
)

// BuildChurn finds files touched at least churnMinCommits times inside any
// rolling 30-day window. A file being rewritten that often is either the heart
// of the system or a place nobody has got right yet.
func BuildChurn(scoped []core.ScopedCommit) []core.ChurnHotspot {
	type touch struct {
		when    time.Time
		added   int
		deleted int
	}
	byPath := map[string][]touch{}
	for _, sc := range scoped {
		for _, f := range sc.Files {
			byPath[f.Path] = append(byPath[f.Path], touch{sc.When, f.Added, f.Deleted})
		}
	}

	out := []core.ChurnHotspot{}
	window := time.Duration(churnWindowDays) * 24 * time.Hour

	for p, touches := range byPath {
		if len(touches) < churnMinCommits {
			continue
		}
		sort.Slice(touches, func(i, j int) bool { return touches[i].when.Before(touches[j].when) })

		best, bestStart := 0, time.Time{}
		left := 0
		for right := range touches {
			for touches[right].when.Sub(touches[left].when) > window {
				left++
			}
			if n := right - left + 1; n > best {
				best, bestStart = n, touches[left].when
			}
		}
		if best < churnMinCommits {
			continue
		}

		hotspot := core.ChurnHotspot{
			Path:               p,
			MaxCommitsInWindow: best,
			WindowStart:        bestStart,
			TotalCommits:       len(touches),
		}
		for _, t := range touches {
			hotspot.Added += t.added
			hotspot.Deleted += t.deleted
		}
		out = append(out, hotspot)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.MaxCommitsInWindow != b.MaxCommitsInWindow {
			return a.MaxCommitsInWindow > b.MaxCommitsInWindow
		}
		if a.TotalCommits != b.TotalCommits {
			return a.TotalCommits > b.TotalCommits
		}
		return a.Path < b.Path
	})
	if len(out) > churnLimit {
		out = out[:churnLimit]
	}
	return out
}
