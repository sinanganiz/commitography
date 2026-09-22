package checks

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
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
			default:
				lines += len(got.Lines)
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
