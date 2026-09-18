// Package hotspot is the hotspot metric family (ADR-0024, ADR-0040): the files
// that attract change. Its metrics are those of docs/metrics.md section 10
// (ADR-0062), and its churn limit is the catalogue's, taken from core
// (ADR-0053 clause 3). Of that section it computes the churn files; the
// complexity proxy and the hotspot score arrive with WP-0026.
package hotspot

import (
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
)

const (
	// churnWindowDays and churnMinCommits define a churn file: one receiving
	// at least churnMinCommits commits inside any rolling churnWindowDays.
	churnWindowDays = 30
	churnMinCommits = 5
)

// version is the family version (ADR-0031 clause 2).
func version() core.Version { return core.Version{Major: 1, Minor: 0} }

// Build finds the churn files: files touched at least churnMinCommits times
// inside any rolling 30-day window. A file being rewritten that often is
// either the heart of the system or a place nobody has got right yet.
// Exceeding the churn limit degrades the family with cardinality_limit.
func Build(scoped []core.ScopedCommit) core.Family[core.HotspotMetrics] {
	byPath := map[string][]time.Time{}
	for _, sc := range scoped {
		for _, f := range sc.Files {
			byPath[f.Path] = append(byPath[f.Path], sc.When)
		}
	}

	type churn struct {
		file  core.ChurnFile
		total int
	}
	var found []churn
	window := time.Duration(churnWindowDays) * 24 * time.Hour

	for p, touches := range byPath {
		if len(touches) < churnMinCommits {
			continue
		}
		sort.Slice(touches, func(i, j int) bool { return touches[i].Before(touches[j]) })

		best, left := 0, 0
		for right := range touches {
			for touches[right].Sub(touches[left]) > window {
				left++
			}
			if n := right - left + 1; n > best {
				best = n
			}
		}
		if best < churnMinCommits {
			continue
		}
		found = append(found, churn{core.ChurnFile{Path: p, CommitsInWindow: best}, len(touches)})
	}

	sort.Slice(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.file.CommitsInWindow != b.file.CommitsInWindow {
			return a.file.CommitsInWindow > b.file.CommitsInWindow
		}
		if a.total != b.total {
			return a.total > b.total
		}
		return a.file.Path < b.file.Path
	})

	truncated := len(found) > core.LimitChurnFiles
	if truncated {
		found = found[:core.LimitChurnFiles]
	}
	files := make([]core.ChurnFile, 0, len(found))
	for _, c := range found {
		files = append(files, c.file)
	}
	f := core.Computed(version(), core.HotspotMetrics{ChurnFiles: files})
	if truncated {
		f.Degrade(core.ReasonCardinalityLimit, core.ConfidencePartial)
	}
	return f
}
