package checks

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
	"github.com/sinanganiz/commitography/internal/pipeline/replay"
)

// The replay stage against the generated fixtures (WP-0013). The stage's own
// tests drive its walk over histories built by hand; these drive it over real
// repositories, from the collect stage's records, as the pipeline runs it.

// replayRun is one fixture replayed.
type replayRun struct {
	dir     string
	history *model.History
	state   *core.ReplayState
	stats   replay.Stats
}

// replayFixture collects a repository under the built-in analysis plane and
// replays it with the given single-file-size limit.
func replayFixture(t *testing.T, dir string, limit int64) replayRun {
	t.Helper()
	history := collectFixture(t, newCollector(), dir)
	paths, err := filter.NewPathFilterFromAttributes(config.Default(), []byte(history.Attributes))
	if err != nil {
		fatal(t, 20, "building the path filter for %s: %v", filepath.Base(dir), err)
	}
	state, stats, err := replay.Run(replay.Options{
		Context:      context.Background(),
		RepoPath:     dir,
		History:      history,
		PathFilter:   paths,
		MaxFileBytes: limit,
	})
	if err != nil {
		fatal(t, 20, "replaying %s: %v", filepath.Base(dir), err)
	}
	return replayRun{dir: dir, history: history, state: state, stats: stats}
}

// fixtureDir is a generated fixture's directory.
func fixtureDir(t *testing.T, repo repository, fixture string) string {
	t.Helper()
	dir := filepath.Join(repo.root, "testdata", "fixtures", fixture)
	if !repo.isTracked(goldenFile(fixture, false)) && !repo.isTracked(goldenFile(fixture, true)) {
		fatal(t, 64, "%s is not a fixture the golden set knows", fixture)
	}
	return dir
}

// smallCollectedFixtures are the fixtures the analysis produces a report for,
// but for the designated large one, which the full gate measures.
func smallCollectedFixtures(t *testing.T, repo repository) []string {
	t.Helper()
	large := largeFixture(t, repo)
	var out []string
	for _, fixture := range collectedFixtures(t, repo) {
		if fixture != large {
			out = append(out, fixture)
		}
	}
	return out
}

// headFiles lists the analysed commit's files, read from git independently of
// the replay stage: each blob's path and object name, with its content.
type headFile struct {
	path, blob, content string
}

func headFiles(t *testing.T, dir, commit string) []headFile {
	t.Helper()
	entries, err := git.Records(context.Background(), git.At(dir, "ls-tree", "-r", "-z", commit).Pathspecs())
	if err != nil {
		fatal(t, 20, "listing %s's tree: %v", filepath.Base(dir), err)
	}
	var out []headFile
	for _, entry := range entries {
		meta, path, _ := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		content, err := git.Output(context.Background(), git.At(dir, "cat-file", "blob", fields[2]).WithTimeout(0))
		if err != nil {
			fatal(t, 20, "reading %s from %s: %v", path, filepath.Base(dir), err)
		}
		out = append(out, headFile{path: path, blob: fields[2], content: content})
	}
	return out
}

// lineCount is how many lines git counts in a content: each terminator ends
// one, and a last line without one is a line too.
func lineCount(content string) int {
	n := strings.Count(content, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		n++
	}
	return n
}

// TestReplayMapCoversEveryTrackedTextLine requires the ownership map to hold
// every tracked text line at the analysed commit (WP-0013, ADR-0020 clause 3):
// every file of the analysed commit that is not excluded, with the object name
// of its content there, and for a text file one owned line per line of that
// content. The files and their lines are read from git here, not from replay.
func TestReplayMapCoversEveryTrackedTextLine(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	for _, fixture := range smallCollectedFixtures(t, repo) {
		run := replayFixture(t, fixtureDir(t, repo, fixture), config.DefaultMaxFileBytes)
		ownership := run.state.Ownership
		if ownership == nil {
			report(t, 20, "%s: replay produced no ownership map: %s", fixture, run.state.Unavailable)
			continue
		}
		paths, err := filter.NewPathFilterFromAttributes(config.Default(), []byte(run.history.Attributes))
		if err != nil {
			fatal(t, 20, "%s: building the path filter: %v", fixture, err)
		}
		held := map[string]core.OwnedFile{}
		for _, f := range ownership.Files {
			held[f.Path] = f
		}
		want, lines := 0, 0
		for _, file := range headFiles(t, run.dir, run.history.Repository.HeadCommit) {
			if paths.Excluded(file.path) {
				continue
			}
			want++
			got, ok := held[file.path]
			switch {
			case !ok:
				report(t, 20, "%s: the ownership map does not hold %s", fixture, file.path)
			case got.Blob != file.blob:
				report(t, 20, "%s: the map holds %s at %s, and the analysed commit at %s", fixture, file.path,
					got.Blob, file.blob)
			case !isText(file.content):
				if !got.Binary || got.Lines != nil {
					report(t, 20, "%s: %s is binary and the map holds it as %+v", fixture, file.path, got)
				}
			case len(got.Lines) != lineCount(file.content):
				report(t, 20, "%s: %s has %d lines and the map holds %d", fixture, file.path,
					lineCount(file.content), len(got.Lines))
			}
			if isText(file.content) {
				lines += lineCount(file.content)
			}
		}
		if len(held) != want {
			report(t, 20, "%s: the map holds %d files and the analysed commit %d outside excluded paths", fixture,
				len(held), want)
		}
		if ownership.Lines() != lines {
			report(t, 20, "%s: the map holds %d lines and the analysed commit's text files %d", fixture,
				ownership.Lines(), lines)
		}
		if want == 0 {
			fatal(t, 64, "%s has no file outside excluded paths, so nothing was compared", fixture)
		}
	}
}

// commitsOldestFirst returns the records in the order they were committed.
func commitsOldestFirst(h *model.History) []model.Commit {
	out := append([]model.Commit(nil), h.Commits...)
	sort.SliceStable(out, func(a, b int) bool { return out[a].CommitterDate.Before(out[b].CommitterDate) })
	return out
}

// ownerOf renders a line's owner and day as the identity digest and the date.
func ownerOf(o *core.Ownership, line core.OwnedLine) string {
	return fmt.Sprintf("%s %s", o.Identities[line.Owner], dayDate(line.Day))
}

// dayDate writes a day number, days since 1970-01-01, as its calendar date.
func dayDate(day int32) string {
	return time.Unix(int64(day)*86400, 0).UTC().Format("2006-01-02")
}

// TestReplayMergedSideBranchOwnership checks the merge rule of ADR-0073
// clause 5 against a fixture whose every surviving line has one owner by
// construction: lines written on a side branch are their side-branch author's,
// a conflict resolution that matches neither parent is the merging author's,
// a branch merged with -s ours leaves nothing, and a branch merged into a main
// line that had not moved keeps its author.
func TestReplayMergedSideBranchOwnership(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	run := replayFixture(t, fixtureDir(t, repo, "merged-side-branch"), config.DefaultMaxFileBytes)
	ownership := run.state.Ownership
	if ownership == nil {
		fatal(t, 20, "replay produced no ownership map for the merged-side-branch fixture: %s", run.state.Unavailable)
	}
	identity := map[string]string{}
	for _, c := range run.history.Commits {
		identity[c.AuthorName] = c.IdentityID
	}
	owner := func(name, date string) string { return identity[name] + " " + date }
	ada, grace, alan := "Ada Lovelace", "Grace Hopper", "Alan Turing"
	written := owner(ada, "2025-10-01")
	want := map[string][]string{
		"app.txt": {
			written, written, written,
			owner(ada, "2025-10-04"), // the resolution, matching no parent
			written, written, written,
			owner(alan, "2025-10-03"), // the first parent's own change
			written, written,
			owner(grace, "2025-10-02"), owner(grace, "2025-10-02"), // the side branch's
		},
		"side.txt":  {owner(grace, "2025-10-02"), owner(grace, "2025-10-02")},
		"notes.txt": {owner(alan, "2025-10-07"), owner(alan, "2025-10-07")},
	}
	for _, f := range ownership.Files {
		expected, ok := want[f.Path]
		if !ok {
			report(t, 73, "the map holds %s, which the fixture's analysed commit does not", f.Path)
			continue
		}
		delete(want, f.Path)
		var got []string
		for _, line := range f.Lines {
			got = append(got, ownerOf(ownership, line))
		}
		if strings.Join(got, "\n") != strings.Join(expected, "\n") {
			report(t, 73, "%s is owned\n  %s\nand by construction it is\n  %s", f.Path,
				strings.Join(got, "\n  "), strings.Join(expected, "\n  "))
		}
	}
	for path := range want {
		report(t, 73, "the map does not hold %s", path)
	}
}

// blameOwners reads git blame's commit for every line of a file at a commit,
// with the given detection options, from its incremental output.
//
// That output is line-framed, which ADR-0072 clause 3 forbids for records
// that carry attacker-controlled content. A fixture is the product's own
// generated content, not an attacker's, and only the entry headers are read:
// an object name, then three decimal numbers, a form no other line of the
// output takes, because every other line begins with its key.
func blameOwners(t *testing.T, dir, commit, path string, options ...string) []string {
	t.Helper()
	args := append(append([]string{"blame", "--incremental"}, options...), commit)
	out, err := git.Output(context.Background(), git.At(dir, args...).Pathspecs(path))
	if err != nil {
		fatal(t, 19, "blaming %s in %s: %v", path, filepath.Base(dir), err)
	}
	entry := regexp.MustCompile(`^([0-9a-f]{40}) [0-9]+ ([0-9]+) ([0-9]+)$`)
	var owners []string
	for _, line := range strings.Split(out, "\n") {
		m := entry.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		final, _ := strconv.Atoi(m[2])
		count, _ := strconv.Atoi(m[3])
		for len(owners) < final-1+count {
			owners = append(owners, "")
		}
		for i := 0; i < count; i++ {
			owners[final-1+i] = m[1]
		}
	}
	return owners
}

// divergence is the share of a map's lines whose owner or day differs from the
// commit git blame names for the line, with the given detection options.
func divergence(t *testing.T, run replayRun, options ...string) (float64, int) {
	t.Helper()
	ownership := run.state.Ownership
	commits := map[string]model.Commit{}
	for _, c := range run.history.Commits {
		commits[c.Hash] = c
	}
	differ, total := 0, 0
	for _, f := range ownership.Files {
		if f.Binary || f.Lines == nil {
			continue
		}
		blamed := blameOwners(t, run.dir, ownership.Commit, f.Path, options...)
		if len(blamed) != len(f.Lines) {
			fatal(t, 19, "blame gives %s %d lines and the map %d", f.Path, len(blamed), len(f.Lines))
		}
		for i, line := range f.Lines {
			c, ok := commits[blamed[i]]
			if !ok {
				fatal(t, 19, "blame names commit %.12s for %s line %d, which is not in the records", blamed[i],
					f.Path, i+1)
			}
			total++
			if ownerOf(ownership, line) != c.IdentityID+" "+c.ActiveDate {
				differ++
			}
		}
	}
	if total == 0 {
		fatal(t, 64, "the %s fixture has no line to compare with blame", filepath.Base(run.dir))
	}
	return float64(differ) / float64(total), total
}

// The blame divergence thresholds of ADR-0019 clause 6, per fixture and per
// blame. Each is the measured divergence, as a share of lines, with nothing
// added: a change to the alignment or the walk that moves more lines away
// from blame fails here and has to say why.
//
// Plain blame follows a file's renames, as replay does, and on these fixtures
// agrees with replay on every line. Blame with -M -C -C also finds lines moved
// within a file and lines copied from another file of the same commit or of
// the commit that created the file, which replay does not reproduce
// (docs/metrics.md section 7): on the renames-and-copied-block fixture that is
// the ten-line block copied into src/format/printer.go, which blame gives to
// the parser's author and replay to the commit that copied it.
func blameThresholds() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"renames-and-copied-block": {"blame": 0, "blame -M -C -C": 10.0 / 44},
		"merged-side-branch":       {"blame": 0, "blame -M -C -C": 0},
	}
}

// TestReplayBlameDivergence measures replay-derived ownership against git
// blame on the renames-and-copied-block fixture and on a fixture with a
// merged side branch, and requires each divergence to stay at or below its
// recorded threshold (ADR-0019 clause 6, ADR-0073). A line diverges when its
// owner or its day differs from those of the commit blame names for it.
func TestReplayBlameDivergence(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	for fixture, thresholds := range blameThresholds() {
		run := replayFixture(t, fixtureDir(t, repo, fixture), config.DefaultMaxFileBytes)
		if run.state.Ownership == nil {
			fatal(t, 19, "replay produced no ownership map for %s: %s", fixture, run.state.Unavailable)
		}
		for name, options := range map[string][]string{"blame": nil, "blame -M -C -C": {"-M", "-C", "-C"}} {
			share, lines := divergence(t, run, options...)
			// ADR-0050 clause 3 asks for measurements to be recorded on every
			// enforcing run.
			t.Logf("%s: %d lines, %.4f diverge from %s", fixture, lines, share, name)
			if share > thresholds[name]+1e-9 {
				report(t, 19, "%s: %.4f of %d lines diverge from %s, over the recorded threshold of %.4f",
					fixture, share, lines, name, thresholds[name])
			}
		}
	}
}

// TestReplayLinearHistoryOwnership checks line ownership on a linear history
// against what its generator wrote: the basic fixture appends one line to
// src/main.go in every commit, so each line is owned by the commit that
// appended it, on that commit's local date.
func TestReplayLinearHistoryOwnership(t *testing.T) {
	t.Parallel()
	repo := openRepository(t)
	run := replayFixture(t, fixtureDir(t, repo, "basic"), config.DefaultMaxFileBytes)
	ownership := run.state.Ownership
	if ownership == nil {
		fatal(t, 20, "replay produced no ownership map for the basic fixture: %s", run.state.Unavailable)
	}
	var main core.OwnedFile
	for _, f := range ownership.Files {
		if f.Path == "src/main.go" {
			main = f
		}
	}
	commits := commitsOldestFirst(run.history)
	if len(main.Lines) != len(commits) {
		fatal(t, 20, "src/main.go holds %d lines and the fixture %d commits", len(main.Lines), len(commits))
	}
	for i, c := range commits {
		want := fmt.Sprintf("%s %s", c.IdentityID, c.ActiveDate)
		if got := ownerOf(ownership, main.Lines[i]); got != want {
			report(t, 20, "src/main.go line %d is owned by %s, and the commit that wrote it is %s", i+1, got, want)
		}
	}
}
