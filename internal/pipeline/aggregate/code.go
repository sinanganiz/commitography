package aggregate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/metrics/commitsize"
	"github.com/sinanganiz/commitography/internal/metrics/files"
	"github.com/sinanganiz/commitography/internal/metrics/ownership"
)

const (
	// blameSampleSize caps how many files are blamed. Blame is the most
	// expensive operation in the pipeline, so it is sampled rather than
	// exhaustive, and the sample is deterministic so two runs agree.
	blameSampleSize = 300

	// binarySniffBytes is how much of a file is inspected for a NUL byte when
	// deciding whether blame would say anything meaningful about it.
	binarySniffBytes = 8000
)

// buildCode assembles the code section. The commit-size and file-activity
// figures come from their families, and the code-age figures from the
// ownership family.
//
// Known deviation from ADR-0020 clause 3, which gives working tree access to
// the replay stage alone: this stage still lists the tracked files and runs
// sampled blame through git, and reads working tree files to detect binaries
// and count lines. Moving that work to replay now would reorder the server's
// progress events. WP-0013 removes the deviation.
func buildCode(in core.Input, analyzed, lineScoped []model.Commit) (core.CodeMetrics, []string, error) {
	var warnings []string
	m := core.CodeMetrics{
		MostTouchedFiles:     []core.TouchedFile{},
		FileTypeDistribution: []core.FileTypeShare{},
		CodeAge:              []core.YearLines{},
	}

	commitsize.BuildCommitSize(in, lineScoped, &m)

	tracked, err := trackedFilesContext(contextOf(in), in.RepoPath)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return m, warnings, err
		}
		warnings = append(warnings, fmt.Sprintf("could not list tracked files: %v", err))
	}

	included := files.BuildFiles(in, lineScoped, tracked, &m)

	textFiles := textCandidates(in.RepoPath, included)
	m.CodeAgeTotalFiles = len(textFiles)
	m.TrackedLines = countLines(in.RepoPath, textFiles)

	if in.NoBlame {
		return m, warnings, nil
	}

	sample := sampleFiles(textFiles, blameSampleSize)
	m.CodeAgeSampledFiles = len(sample)
	progress(in, "blame", fmt.Sprintf("%d of %d files", len(sample), len(textFiles)), 0, len(sample))

	ages, blameWarnings, err := blameYears(contextOf(in), in.RepoPath, sample, func(current, total int) {
		progress(in, "blame", fmt.Sprintf("%d of %d files", current, total), current, total)
	})
	if err != nil {
		return m, warnings, err
	}
	warnings = append(warnings, blameWarnings...)
	m.CodeAge = ages
	m.SurvivingFromFirstYear = ownership.SurvivingFromFirstYear(in, analyzed, ages)

	return m, warnings, nil
}

func trackedFiles(repoPath string) ([]string, error) {
	return trackedFilesContext(context.Background(), repoPath)
}

func trackedFilesContext(ctx context.Context, repoPath string) ([]string, error) {
	return git.LinesContext(ctx, repoPath, "ls-tree", "-r", "--name-only", "HEAD")
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
func blameYears(ctx context.Context, repoPath string, paths []string, progress func(current, total int)) ([]core.YearLines, []string, error) {
	var warnings []string
	years := map[int]int{}

	for index, p := range paths {
		if err := ctx.Err(); err != nil {
			return nil, warnings, err
		}
		out, err := git.RunContext(ctx, repoPath, "blame", "--line-porcelain", "-w", "-M", "HEAD", "--", p)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, warnings, err
			}
			warnings = append(warnings, fmt.Sprintf("blame failed for %s", p))
			if progress != nil {
				progress(index+1, len(paths))
			}
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
		if progress != nil {
			progress(index+1, len(paths))
		}
	}

	out := make([]core.YearLines, 0, len(years))
	for year, lines := range years {
		out = append(out, core.YearLines{Year: year, Lines: lines})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Year < out[j].Year })
	return out, warnings, nil
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
