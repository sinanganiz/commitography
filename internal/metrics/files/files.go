// Package files is the files metric family (ADR-0024, ADR-0040): which files
// the history keeps returning to.
package files

import (
	"path"
	"sort"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

const (
	// mostTouchedLimit and fileTypeLimit keep the report proportional to the
	// dashboard rather than to the repository.
	mostTouchedLimit = 25
	fileTypeLimit    = 15
)

// fileStat accumulates per-path activity across the line-scoped commit set.
type fileStat struct {
	commits      int
	added        int
	deleted      int
	lastModified time.Time
}

// BuildFiles fills the file-activity figures of the code section from the
// line-scoped commits and the paths tracked at HEAD. It returns the tracked
// paths that survive path exclusion.
func BuildFiles(in core.Input, lineScoped []model.Commit, tracked []string, m *core.CodeMetrics) []string {
	stats := map[string]*fileStat{}
	extStats := map[string]*core.FileTypeShare{}

	for _, c := range lineScoped {
		when := in.Date(c)
		for _, f := range filter.IncludedFiles(c, in.PathFilter) {
			st, ok := stats[f.Path]
			if !ok {
				st = &fileStat{}
				stats[f.Path] = st
			}
			st.commits++
			st.added += f.Added
			st.deleted += f.Deleted
			if when.After(st.lastModified) {
				st.lastModified = when
			}

			ext := extensionOf(f.Path)
			es, ok := extStats[ext]
			if !ok {
				es = &core.FileTypeShare{Extension: ext}
				extStats[ext] = es
			}
			es.Changes++
			es.Added += f.Added
			es.Deleted += f.Deleted
		}
	}

	m.FileTypeDistribution = topFileTypes(extStats)
	m.TrackedFiles = len(tracked)

	inHead := make(map[string]bool, len(tracked))
	included := make([]string, 0, len(tracked))
	for _, p := range tracked {
		inHead[p] = true
		if !in.PathFilter.Excluded(p) {
			included = append(included, p)
		}
	}

	m.MostTouchedFiles = topTouchedFiles(stats, inHead)
	m.OldestUntouchedFile = oldestUntouchedFile(included, stats)
	return included
}

// topTouchedFiles ranks paths by how many commits touched them. Files since
// removed from HEAD stay in the ranking, flagged, because "what did this team
// keep returning to" is a historical question.
func topTouchedFiles(stats map[string]*fileStat, inHead map[string]bool) []core.TouchedFile {
	out := make([]core.TouchedFile, 0, len(stats))
	for p, st := range stats {
		out = append(out, core.TouchedFile{
			Path:            p,
			Commits:         st.commits,
			Added:           st.added,
			Deleted:         st.deleted,
			DeletedFromHead: !inHead[p],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Commits != out[j].Commits {
			return out[i].Commits > out[j].Commits
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > mostTouchedLimit {
		out = out[:mostTouchedLimit]
	}
	return out
}

func topFileTypes(extStats map[string]*core.FileTypeShare) []core.FileTypeShare {
	out := make([]core.FileTypeShare, 0, len(extStats))
	for _, es := range extStats {
		out = append(out, *es)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Changes != out[j].Changes {
			return out[i].Changes > out[j].Changes
		}
		return out[i].Extension < out[j].Extension
	})
	if len(out) > fileTypeLimit {
		out = out[:fileTypeLimit]
	}
	return out
}

// oldestUntouchedFile is the file still present in HEAD whose most recent
// modifying commit is the oldest: the part of the codebase nobody has needed to
// think about in the longest time.
func oldestUntouchedFile(inHead []string, stats map[string]*fileStat) *core.FileAge {
	var best *core.FileAge
	for _, p := range inHead {
		st, ok := stats[p]
		if !ok || st.lastModified.IsZero() {
			continue
		}
		if best == nil || st.lastModified.Before(best.LastModified) ||
			(st.lastModified.Equal(best.LastModified) && p < best.Path) {
			best = &core.FileAge{Path: p, LastModified: st.lastModified}
		}
	}
	return best
}

func extensionOf(p string) string {
	ext := strings.ToLower(path.Ext(path.Base(p)))
	if ext == "" || ext == "." {
		return "(none)"
	}
	return strings.TrimPrefix(ext, ".")
}
