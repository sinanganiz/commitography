package collect

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
)

// Each line-count entry takes the object names of the raw entry in its
// position: the all-zero name of an absent side and a submodule's commit are
// no content, and a symbolic link's target is (WP-0013 clause 2).
func TestReplayCollectPairsObjectNamesWithLineCounts(t *testing.T) {
	t.Parallel()
	link := "3333333333333333333333333333333333333333"
	sub := "4444444444444444444444444444444444444444"
	stream := header(testHash, "chore: every kind of change") + "\n" +
		rawObjects("A", noObject, newObject, "added.go") + "\x00" +
		rawObjects("D", oldObject, noObject, "deleted.go") + "\x00" +
		rawObjects("M", oldObject, newObject, "modified.go") + "\x00" +
		":000000 120000 " + noObject + " " + link + " A\x00link" + "\x00" +
		":000000 160000 " + noObject + " " + sub + " A\x00vendor/sub" + "\x00" +
		rawObjects("R100", oldObject, oldObject, "old.go", "new.go") + "\x00" +
		"1\t0\tadded.go\x00" + "0\t1\tdeleted.go\x00" + "1\t1\tmodified.go\x00" + "1\t0\tlink\x00" +
		"1\t0\tvendor/sub\x00" + "0\t0\t\x00old.go\x00new.go\x00\x00"

	parsed := parseStream(Options{}, 0, stream)
	if parsed.failed != 0 || len(parsed.commits) != 1 {
		t.Fatalf("got %d commits and %d failures, want one commit", len(parsed.commits), parsed.failed)
	}
	want := map[string][2]string{
		"added.go":    {"", newObject},
		"deleted.go":  {oldObject, ""},
		"modified.go": {oldObject, newObject},
		"link":        {"", link},
		"vendor/sub":  {"", ""},
		"new.go":      {oldObject, oldObject},
	}
	files := parsed.commits[0].Files
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %+v", len(files), len(want), files)
	}
	for _, f := range files {
		if got := [2]string{f.OldBlob, f.NewBlob}; got != want[f.Path] {
			t.Errorf("%s has content %q, want %q", f.Path, got, want[f.Path])
		}
	}
}

// A commit whose line counts and raw entries do not pair is not the stream
// this parser reads. It is counted and skipped, and the commit after it is
// read as usual.
func TestReplayCollectSkipsACommitWhoseEntriesDoNotPair(t *testing.T) {
	t.Parallel()
	next := "0123456789abcdef0123456789abcdef01234567"
	following := header(next, "feat: next") + "\n" + raw("M", "main.go") + "\x001\t0\tmain.go\x00\x00"
	for name, entries := range map[string]string{
		"a raw entry too many":           raw("M", "a.go") + "\x00" + raw("M", "b.go") + "\x001\t0\ta.go\x00",
		"a line-count entry too many":    raw("M", "a.go") + "\x001\t0\ta.go\x001\t0\tb.go\x00",
		"entries naming other paths":     raw("M", "a.go") + "\x001\t0\tb.go\x00",
		"a rename on one side only":      raw("R100", "a.go", "b.go") + "\x001\t0\tb.go\x00",
		"a raw entry that cannot parse":  ":100644 100644 xyz " + newObject + " M\x00a.go\x001\t0\ta.go\x00",
		"a line count that cannot parse": raw("M", "a.go") + "\x00one\t0\ta.go\x00",
		"no raw entry at all":            "1\t0\ta.go\x00",
	} {
		stream := header(testHash, "chore: broken") + "\n" + entries + "\x00" + following
		var warnings []string
		parsed := parseStream(Options{OnWarning: func(m string) { warnings = append(warnings, m) }}, 0, stream)
		if parsed.failed != 1 || parsed.total != 2 || len(warnings) != 1 {
			t.Errorf("%s: %d of %d failed with warnings %q, want the broken commit counted once",
				name, parsed.failed, parsed.total, warnings)
		}
		if len(parsed.commits) != 1 || parsed.commits[0].Hash != next || len(parsed.commits[0].Files) != 1 {
			t.Errorf("%s: the commits read were %+v, want only the one after the broken commit", name, parsed.commits)
		}
	}
}

// A merge's header comes once per parent it differs from. Its raw diffs are
// kept one per header and its line counts are not, so its Files stays empty
// (ADR-0073 clause 5).
func TestReplayCollectKeepsAMergesDiffPerHeader(t *testing.T) {
	t.Parallel()
	first, second := "1"+testHash[1:], "2"+testHash[1:]
	merge := mergeHeader(testHash, "Merge branch 'topic'", first, second)
	next := "0123456789abcdef0123456789abcdef01234567"
	stream := merge + "\n" + raw("M", "shared.go") + "\x00" + raw("R090", "old.go", "new.go") + "\x00" +
		"1\t1\tshared.go\x00" + "2\t0\t\x00old.go\x00new.go\x00\x00" +
		merge + "\n" + raw("M", "shared.go") + "\x00" + "3\t0\tshared.go\x00\x00" +
		header(next, "feat: next") + "\n" + raw("M", "main.go") + "\x001\t0\tmain.go\x00\x00"

	parsed := parseStream(Options{}, 0, stream)
	if parsed.failed != 0 || parsed.total != 2 || len(parsed.commits) != 2 {
		t.Fatalf("got %d commits and %d of %d failed, want the merge and the commit after it",
			len(parsed.commits), parsed.failed, parsed.total)
	}
	if m := parsed.commits[0]; !m.IsMerge || len(m.Files) != 0 {
		t.Errorf("the merge came back as %+v, want a merge with no files", m)
	}
	sections := parsed.sections[testHash]
	if len(sections) != 2 || len(sections[0]) != 2 || len(sections[1]) != 1 {
		t.Fatalf("the merge's diffs came back as %+v, want two entries and then one", sections)
	}
	if r := sections[0][1]; r.previousPath != "old.go" || r.path != "new.go" || r.status != 'R' {
		t.Errorf("the rename in the first diff came back as %+v", r)
	}
	if r := sections[1][0]; r.path != "shared.go" {
		t.Errorf("the second diff came back as %+v", r)
	}
}

// A merge's diffs are placed against its parents by tree, never by position:
// a parent whose tree is the merge's has no diff, and git shows none for it.
func TestReplayCollectPlacesMergeDiffsByTree(t *testing.T) {
	t.Parallel()
	p1, p2, p3, m := "p1", "p2", "p3", "m"
	change := func(path, before, after string) rawChange {
		return rawChange{oldMode: "100644", newMode: "100644", oldBlob: before, newBlob: after, status: 'M', path: path}
	}
	onP1 := change("f", oldObject, newObject)
	onP2 := change("f", "5555555555555555555555555555555555555555", newObject)
	commit := func(tree string, parents ...string) model.Commit {
		return model.Commit{Hash: m, Tree: tree, Parents: parents, IsMerge: len(parents) > 1}
	}
	trees := map[string]string{p1: "t1", p2: "t2", p3: "t3"}

	cases := []struct {
		name   string
		merge  model.Commit
		shown  [][]rawChange
		trees  map[string]string
		want   []model.MergeChange
		placed bool
	}{
		{
			name:  "a merge that kept its first parent's tree shows only the second parent's diff",
			merge: commit("t1", p1, p2), shown: [][]rawChange{{onP2}}, trees: trees, want: nil, placed: true,
		},
		{
			name:  "a merge that took its second parent's tree shows only the first parent's diff",
			merge: commit("t2", p1, p2), shown: [][]rawChange{{onP1}}, trees: trees, placed: true,
			want: []model.MergeChange{{Path: "f", NewBlob: newObject, Parents: []model.ParentVersion{
				{Path: "f", Blob: oldObject}, {Path: "f", Blob: newObject}}}},
		},
		{
			name:  "a merge unlike either parent shows both diffs",
			merge: commit("tm", p1, p2), shown: [][]rawChange{{onP1}, {onP2}}, trees: trees, placed: true,
			want: []model.MergeChange{{Path: "f", NewBlob: newObject, Parents: []model.ParentVersion{
				{Path: "f", Blob: oldObject}, {Path: "f", Blob: onP2.oldBlob}}}},
		},
		{
			name:  "an octopus merge skips the parent it equals",
			merge: commit("t2", p1, p2, p3), shown: [][]rawChange{{onP1}, {change("f", "6666666666666666666666666666666666666666", newObject)}},
			trees: trees, placed: true,
			want: []model.MergeChange{{Path: "f", NewBlob: newObject, Parents: []model.ParentVersion{
				{Path: "f", Blob: oldObject}, {Path: "f", Blob: newObject},
				{Path: "f", Blob: "6666666666666666666666666666666666666666"}}}},
		},
		{
			name:  "a merge with every parent's tree shows one empty diff",
			merge: commit("t1", p1, p1), shown: [][]rawChange{nil}, trees: trees, want: nil, placed: true,
		},
		{
			name:  "a merge with a parent outside the records and a parent not shown cannot be placed",
			merge: commit("t2", p1, "absent"), shown: [][]rawChange{{onP1}}, trees: trees, want: nil,
		},
		{
			name:  "a merge with a parent outside the records and every parent shown is placed",
			merge: commit("tm", p1, "absent"), shown: [][]rawChange{{onP1}, {onP2}}, trees: trees, placed: true,
			want: []model.MergeChange{{Path: "f", NewBlob: newObject, Parents: []model.ParentVersion{
				{Path: "f", Blob: oldObject}, {Path: "f", Blob: onP2.oldBlob}}}},
		},
	}
	for _, tc := range cases {
		placed, ok, err := placeSections(tc.merge, tc.shown, tc.trees)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if ok != tc.placed {
			t.Errorf("%s: placed = %v, want %v", tc.name, ok, tc.placed)
			continue
		}
		if ok {
			if got := mergeChanges(placed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s: changes = %+v, want %+v", tc.name, got, tc.want)
			}
		}
	}

	// A diff count that no reading of the trees explains is refused, never
	// guessed at.
	if _, _, err := placeSections(commit("tm", p1, p2), [][]rawChange{{onP1}}, trees); err == nil {
		t.Errorf("a merge unlike both parents with one diff was placed")
	}
	if _, _, err := placeSections(commit("t1", p1, p1), [][]rawChange{{onP1}}, trees); err == nil {
		t.Errorf("a merge with every parent's tree and a diff was placed")
	}
}

// treeBlobs lists a tree's files by path, with their object names. A
// submodule is not a file, and is left out.
func treeBlobs(t *testing.T, repo, tree string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, entry := range gitRecords(t, repo, "ls-tree", "-r", "-z", tree) {
		meta, path, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			t.Fatalf("an ls-tree entry of %s does not parse", tree)
		}
		if fields[1] == "blob" {
			out[path] = fields[2]
		}
	}
	return out
}

// scriptedRepository builds a repository with the merge shapes the fixtures do
// not hold: a merge that keeps its first parent's tree, one that takes its
// second parent's, a rename on a side branch, a conflict resolved by hand, a
// deletion in a merge, and an octopus.
func scriptedRepository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	stamp := 1700000000
	invoke := func(args ...string) error {
		stamp += 3600
		date := strconv.Itoa(stamp) + " +0000"
		spec := git.At(dir, args...).WithEnv(
			"GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL=ada@example.com", "GIT_AUTHOR_DATE="+date,
			"GIT_COMMITTER_NAME=Ada", "GIT_COMMITTER_EMAIL=ada@example.com", "GIT_COMMITTER_DATE="+date,
		).Configured("core.autocrlf=false")
		_, err := git.Output(ctx, spec)
		return err
	}
	run := func(args ...string) {
		t.Helper()
		if err := invoke(args...); err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
	}
	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) {
		t.Helper()
		run("add", "-A")
		run("commit", "-q", "--no-verify", "-m", message)
	}
	run("init", "-q", "-b", "main")
	write("a.txt", "1\n2\n3\n")
	write("b.txt", "b\n")
	write("gone.txt", "gone\n")
	commit("base")

	run("checkout", "-q", "-b", "ours")
	write("a.txt", "1\n2\n3\nside\n")
	commit("side work the merge discards")
	run("checkout", "-q", "main")
	run("merge", "-q", "--no-edit", "-s", "ours", "ours")

	run("checkout", "-q", "-b", "taken")
	run("mv", "b.txt", "c.txt")
	write("c.txt", "b\nc\n")
	run("rm", "-q", "gone.txt")
	commit("rename and delete on a side branch")
	run("checkout", "-q", "main")
	run("merge", "-q", "--no-ff", "--no-edit", "taken")

	run("checkout", "-q", "-b", "conflict")
	write("a.txt", "1\nconflict side\n3\n")
	commit("side edits line 2")
	run("checkout", "-q", "main")
	write("a.txt", "1\nconflict main\n3\n")
	commit("main edits line 2")
	if err := invoke("merge", "-q", "--no-edit", "conflict"); err == nil {
		t.Fatalf("the conflicting merge succeeded, so the repository holds no conflict resolution")
	}
	write("a.txt", "1\nresolved\n3\n")
	commit("resolve the conflict")

	for _, branch := range []string{"o1", "o2"} {
		run("checkout", "-q", "-b", branch, "main")
		write(branch+".txt", branch+"\n")
		commit("octopus arm " + branch)
	}
	run("checkout", "-q", "main")
	write("m.txt", "main\n")
	commit("main moves on")
	run("merge", "-q", "--no-edit", "o1", "o2")
	return dir
}

// Every object name the records carry is the one the commit's own tree and its
// parents' trees hold, and every file whose content or path differs from the
// first parent is recorded: nothing replay reads can disagree with git's own
// trees (WP-0013 clause 2).
func TestReplayCollectObjectNamesMatchTheTrees(t *testing.T) {
	t.Parallel()
	repos := map[string]string{"scripted": scriptedRepository(t)}
	for _, name := range []string{"basic", "merges", "renames-and-copied-block", "binary", "noise"} {
		repos[name] = fixture(t, name)
	}
	for name, repo := range repos {
		history, err := newCollector().Collect(Options{RepoPath: repo})
		if err != nil {
			t.Fatalf("%s: Collect: %v", name, err)
		}
		trees := map[string]string{}
		for _, c := range history.Commits {
			trees[c.Hash] = c.Tree
		}
		merges := 0
		for _, c := range history.Commits {
			own := treeBlobs(t, repo, c.Tree)
			parent := map[string]string{}
			if len(c.Parents) > 0 {
				parent = treeBlobs(t, repo, trees[c.Parents[0]])
			}
			covered := map[string]bool{}
			if c.IsMerge {
				merges++
				for _, m := range c.MergeChanges {
					covered[m.Path], covered[m.Parents[0].Path] = true, true
					if m.NewBlob != own[m.Path] {
						t.Errorf("%s: merge %.8s records %s as %q, and its tree holds %q", name, c.Hash, m.Path,
							m.NewBlob, own[m.Path])
					}
					for k, v := range m.Parents {
						if held := treeBlobs(t, repo, trees[c.Parents[k]])[v.Path]; v.Blob != held {
							t.Errorf("%s: merge %.8s records parent %d's %s as %q, and its tree holds %q",
								name, c.Hash, k, v.Path, v.Blob, held)
						}
					}
				}
			} else {
				for _, f := range c.Files {
					source := f.Path
					if f.PreviousPath != "" {
						source = f.PreviousPath
					}
					covered[f.Path], covered[source] = true, true
					if f.NewBlob != own[f.Path] || f.OldBlob != parent[source] {
						t.Errorf("%s: commit %.8s records %s as %q to %q, and the trees hold %q to %q", name,
							c.Hash, f.Path, f.OldBlob, f.NewBlob, parent[source], own[f.Path])
					}
				}
			}
			for path := range union(own, parent) {
				if own[path] != parent[path] && !covered[path] {
					t.Errorf("%s: commit %.8s changes %s from its first parent and records no change to it",
						name, c.Hash, path)
				}
			}
		}
		if name == "scripted" && merges != 4 {
			t.Errorf("the scripted repository has %d merges, want 4", merges)
		}
	}
}

func union(a, b map[string]string) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
