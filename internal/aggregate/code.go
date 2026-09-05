package aggregate

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/filter"
	"github.com/sinanganiz/commitography/internal/gitcmd"
	"github.com/sinanganiz/commitography/internal/model"
)

const (
	// blameSampleSize caps how many files are blamed. Blame is the most
	// expensive operation in the pipeline, so it is sampled rather than
	// exhaustive, and the sample is deterministic so two runs agree.
	blameSampleSize = 300

	// mostTouchedLimit and fileTypeLimit keep the report proportional to the
	// dashboard rather than to the repository.
	mostTouchedLimit = 25
	fileTypeLimit    = 15

	// binarySniffBytes is how much of a file is inspected for a NUL byte when
	// deciding whether blame would say anything meaningful about it.
	binarySniffBytes = 8000
)

// TouchedFile is one entry in the most-modified-files table.
//
// Note on field naming: `deleted` is the deleted-line count, matching `added`
// beside it. Whether the file is gone from HEAD is reported separately as
// `deletedFromHead`, because one key cannot carry both a count and a flag.
type TouchedFile struct {
	Path            string `json:"path"`
	Commits         int    `json:"commits"`
	Added           int    `json:"added"`
	Deleted         int    `json:"deleted"`
	DeletedFromHead bool   `json:"deletedFromHead"`
}

// CommitRef identifies one commit in the report without embedding the record.
type CommitRef struct {
	Hash         string    `json:"hash"`
	Subject      string    `json:"subject"`
	Date         time.Time `json:"date"`
	LinesChanged int       `json:"linesChanged"`
	Added        int       `json:"added"`
	Deleted      int       `json:"deleted"`
	Files        int       `json:"files"`
}

// FileAge names a file and when it was last modified.
type FileAge struct {
	Path         string    `json:"path"`
	LastModified time.Time `json:"lastModified"`
}

// FileTypeShare is one row of the extension distribution.
type FileTypeShare struct {
	Extension string `json:"extension"`
	Changes   int    `json:"changes"`
	Added     int    `json:"added"`
	Deleted   int    `json:"deleted"`
}

// YearLines is one stratum of the code-age chart.
type YearLines struct {
	Year  int `json:"year"`
	Lines int `json:"lines"`
}

// CodeMetrics describes what the code looks like and how it is aging.
type CodeMetrics struct {
	MostTouchedFiles     []TouchedFile   `json:"mostTouchedFiles"`
	LargestCommit        *CommitRef      `json:"largestCommit"`
	AverageCommitSize    float64         `json:"averageCommitSize"`
	MedianCommitSize     float64         `json:"medianCommitSize"`
	TotalAdded           int             `json:"totalAdded"`
	TotalDeleted         int             `json:"totalDeleted"`
	OldestUntouchedFile  *FileAge        `json:"oldestUntouchedFile"`
	FileTypeDistribution []FileTypeShare `json:"fileTypeDistribution"`
	CodeAge              []YearLines     `json:"codeAge"`

	// SurvivingFromFirstYear is null when blame was skipped, so the dashboard
	// hides the section instead of showing a misleading zero.
	SurvivingFromFirstYear *float64 `json:"survivingFromFirstYear"`

	CodeAgeSampledFiles int `json:"codeAgeSampledFiles"`
	CodeAgeTotalFiles   int `json:"codeAgeTotalFiles"`

	TrackedFiles int `json:"trackedFiles"`
	TrackedLines int `json:"trackedLines"`
}

// fileStat accumulates per-path activity across the line-scoped commit set.
type fileStat struct {
	commits      int
	added        int
	deleted      int
	lastModified time.Time
}

func buildCode(in Input, analyzed, lineScoped []model.Commit) (CodeMetrics, []string) {
	var warnings []string
	m := CodeMetrics{
		MostTouchedFiles:     []TouchedFile{},
		FileTypeDistribution: []FileTypeShare{},
		CodeAge:              []YearLines{},
	}

	stats := map[string]*fileStat{}
	extStats := map[string]*FileTypeShare{}
	var sizes []int

	for _, c := range lineScoped {
		when := in.date(c)
		size := 0
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
				es = &FileTypeShare{Extension: ext}
				extStats[ext] = es
			}
			es.Changes++
			es.Added += f.Added
			es.Deleted += f.Deleted

			m.TotalAdded += f.Added
			m.TotalDeleted += f.Deleted
			size += f.Added + f.Deleted
		}
		if !c.IsMerge {
			sizes = append(sizes, size)
		}
	}

	m.AverageCommitSize = round(mean(sizes), 1)
	m.MedianCommitSize = round(median(sizes), 1)
	m.LargestCommit = largestCommit(in, lineScoped)
	m.FileTypeDistribution = topFileTypes(extStats)

	tracked, err := trackedFiles(in.RepoPath)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("could not list tracked files: %v", err))
	}
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

	textFiles := textCandidates(in.RepoPath, included)
	m.CodeAgeTotalFiles = len(textFiles)
	m.TrackedLines = countLines(in.RepoPath, textFiles)

	if in.NoBlame {
		return m, warnings
	}

	sample := sampleFiles(textFiles, blameSampleSize)
	m.CodeAgeSampledFiles = len(sample)
	in.progress("blame", fmt.Sprintf("%d of %d files", len(sample), len(textFiles)))

	ages, blameWarnings := blameYears(in.RepoPath, sample)
	warnings = append(warnings, blameWarnings...)
	m.CodeAge = ages

	if total := totalLines(ages); total > 0 && len(analyzed) > 0 {
		firstYear := in.date(earliestCommit(in, analyzed)).Year()
		surviving := 0
		for _, a := range ages {
			if a.Year == firstYear {
				surviving = a.Lines
			}
		}
		ratio := round(float64(surviving)/float64(total), 4)
		m.SurvivingFromFirstYear = &ratio
	}

	return m, warnings
}

// topTouchedFiles ranks paths by how many commits touched them. Files since
// removed from HEAD stay in the ranking, flagged, because "what did this team
// keep returning to" is a historical question.
func topTouchedFiles(stats map[string]*fileStat, inHead map[string]bool) []TouchedFile {
	out := make([]TouchedFile, 0, len(stats))
	for p, st := range stats {
		out = append(out, TouchedFile{
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

func topFileTypes(extStats map[string]*FileTypeShare) []FileTypeShare {
	out := make([]FileTypeShare, 0, len(extStats))
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
func oldestUntouchedFile(inHead []string, stats map[string]*fileStat) *FileAge {
	var best *FileAge
	for _, p := range inHead {
		st, ok := stats[p]
		if !ok || st.lastModified.IsZero() {
			continue
		}
		if best == nil || st.lastModified.Before(best.LastModified) ||
			(st.lastModified.Equal(best.LastModified) && p < best.Path) {
			best = &FileAge{Path: p, LastModified: st.lastModified}
		}
	}
	return best
}

func largestCommit(in Input, commits []model.Commit) *CommitRef {
	var best *CommitRef
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
		best = &CommitRef{
			Hash:         c.Hash,
			Subject:      c.Subject,
			Date:         in.date(c),
			LinesChanged: size,
			Added:        added,
			Deleted:      deleted,
			Files:        files,
		}
	}
	return best
}

func trackedFiles(repoPath string) ([]string, error) {
	return gitcmd.Lines(repoPath, "ls-tree", "-r", "--name-only", "HEAD")
}

// textCandidates removes files blame cannot say anything useful about: those
// carrying a NUL byte near the start, and those absent from the working tree.
// The result is sorted so sampling is reproducible.
func textCandidates(repoPath string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if looksBinary(filepath.Join(repoPath, filepath.FromSlash(p))) {
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func looksBinary(absPath string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		// Present in HEAD but missing from the working tree, as with a sparse
		// checkout. Leave it out rather than guess at its contents.
		return true
	}
	defer f.Close()

	buf := make([]byte, binarySniffBytes)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return true
	}
	return bytes.IndexByte(buf[:n], 0) >= 0
}

// sampleFiles picks a reproducible subset by walking the lexicographically
// sorted list at a fixed stride, so the same repository always yields the same
// sample and therefore the same code-age chart.
func sampleFiles(paths []string, limit int) []string {
	if len(paths) <= limit {
		return paths
	}
	stride := len(paths) / limit
	if stride < 1 {
		stride = 1
	}
	out := make([]string, 0, limit)
	for i := 0; i < len(paths) && len(out) < limit; i += stride {
		out = append(out, paths[i])
	}
	return out
}

// blameYears aggregates blamed lines by the author-date year of the commit that
// last touched each line.
func blameYears(repoPath string, paths []string) ([]YearLines, []string) {
	var warnings []string
	years := map[int]int{}

	for _, p := range paths {
		out, err := gitcmd.Run(repoPath, "blame", "--line-porcelain", "-w", "-M", "HEAD", "--", p)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("blame failed for %s", p))
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			rest, ok := strings.CutPrefix(line, "author-time ")
			if !ok {
				continue
			}
			seconds, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
			if err != nil {
				continue
			}
			years[time.Unix(seconds, 0).UTC().Year()]++
		}
	}

	out := make([]YearLines, 0, len(years))
	for year, lines := range years {
		out = append(out, YearLines{Year: year, Lines: lines})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Year < out[j].Year })
	return out, warnings
}

func totalLines(ages []YearLines) int {
	total := 0
	for _, a := range ages {
		total += a.Lines
	}
	return total
}

func countLines(repoPath string, paths []string) int {
	total := 0
	for _, p := range paths {
		data, err := os.ReadFile(filepath.Join(repoPath, filepath.FromSlash(p)))
		if err != nil {
			continue
		}
		total += bytes.Count(data, []byte{'\n'})
		if len(data) > 0 && data[len(data)-1] != '\n' {
			total++
		}
	}
	return total
}

func extensionOf(p string) string {
	ext := strings.ToLower(path.Ext(path.Base(p)))
	if ext == "" || ext == "." {
		return "(none)"
	}
	return strings.TrimPrefix(ext, ".")
}

func earliestCommit(in Input, commits []model.Commit) model.Commit {
	best := commits[0]
	for _, c := range commits[1:] {
		if in.date(c).Before(in.date(best)) {
			best = c
		}
	}
	return best
}
