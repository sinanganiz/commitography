package replay

import (
	"context"
	"crypto/sha1"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/config"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
)

// fakeObjects stands in for the object reader: contents by object name,
// capped as the real reader caps them, with every name asked for recorded.
type fakeObjects struct {
	contents map[string]string
	limit    int
	reads    []string
}

func (f *fakeObjects) Read(name string) (git.Object, error) {
	f.reads = append(f.reads, name)
	content, ok := f.contents[name]
	if !ok {
		return git.Object{Missing: true}, nil
	}
	object := git.Object{Type: "blob", Size: int64(len(content))}
	if f.limit > 0 && len(content) > f.limit {
		object.Oversized = true
		object.Head = []byte(content[:min(len(content), git.OversizedHeadBytes)])
		return object, nil
	}
	object.Content = []byte(content)
	return object, nil
}

// history builds a history commit by commit, oldest first, the way the
// collect stage records it.
type history struct {
	commits  []model.Commit
	contents map[string]string
	trees    map[string]map[string]string // path to content, by commit
}

func newHistory() *history {
	return &history{contents: map[string]string{}, trees: map[string]map[string]string{}}
}

// name is the object name the history gives a content.
func (h *history) name(content string) string {
	name := fmt.Sprintf("%x", sha1.Sum([]byte(content)))
	h.contents[name] = content
	return name
}

// edit is one file a commit changes: its new content, or its removal, and the
// path it came from where it is renamed.
type edit struct {
	path, from string
	content    string
	remove     bool
}

// day is the time of the history's commits on a numbered day.
func day(n int) time.Time { return time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, n) }

// commit records a non-merge commit with its changes against its parent, or
// against nothing for a root.
func (h *history) commit(hash, author string, on int, parent string, edits ...edit) {
	tree := map[string]string{}
	var parents []string
	if parent != "" {
		parents = []string{parent}
		for p, c := range h.trees[parent] {
			tree[p] = c
		}
	}
	var files []model.FileChange
	for _, e := range edits {
		source := e.path
		if e.from != "" {
			source = e.from
		}
		change := model.FileChange{Path: e.path, PreviousPath: e.from}
		// A raw diff states each old version as the parent holds it.
		if old, ok := h.trees[parent][source]; ok {
			change.OldBlob = h.name(old)
		}
		if !e.remove {
			change.NewBlob = h.name(e.content)
		}
		files = append(files, change)
	}
	// Every path a change leaves goes first, then every path it writes, so a
	// swap leaves both files in place.
	for _, e := range edits {
		if e.from != "" {
			delete(tree, e.from)
		}
		if e.remove {
			delete(tree, e.path)
		}
	}
	for _, e := range edits {
		if !e.remove {
			tree[e.path] = e.content
		}
	}
	h.trees[hash] = tree
	h.commits = append(h.commits, model.Commit{Hash: hash, Parents: parents, IdentityID: author,
		LocalTime: day(on), Files: files})
}

// merge records a merge whose tree is given in full, with its changes against
// its first parent and every parent's version of each changed file.
func (h *history) merge(hash, author string, on int, parents []string, tree map[string]string) {
	var changes []model.MergeChange
	first := h.trees[parents[0]]
	paths := map[string]bool{}
	for p := range tree {
		paths[p] = true
	}
	for p := range first {
		paths[p] = true
	}
	for p := range paths {
		if tree[p] == first[p] && hasKey(tree, p) == hasKey(first, p) {
			continue
		}
		change := model.MergeChange{Path: p}
		if c, ok := tree[p]; ok {
			change.NewBlob = h.name(c)
		}
		for _, parent := range parents {
			version := model.ParentVersion{Path: p}
			if c, ok := h.trees[parent][p]; ok {
				version.Blob = h.name(c)
			}
			change.Parents = append(change.Parents, version)
		}
		changes = append(changes, change)
	}
	h.trees[hash] = tree
	h.commits = append(h.commits, model.Commit{Hash: hash, Parents: parents, IsMerge: true, IdentityID: author,
		LocalTime: day(on), MergeChanges: changes})
}

func hasKey(m map[string]string, k string) bool { _, ok := m[k]; return ok }

// records returns the history as collect records it, newest first, with head
// as the analysed commit.
func (h *history) records(head string) *model.History {
	commits := make([]model.Commit, len(h.commits))
	for i, c := range h.commits {
		commits[len(commits)-1-i] = c
	}
	return &model.History{Repository: model.RepositoryInfo{HeadCommit: head}, Commits: commits}
}

// replayed walks a history with a fake reader and returns the map, the reason
// when there is none, and the reader.
func replayed(t *testing.T, h *history, head string, limit int, attributes string) (*core.Ownership, string, *fakeObjects, Stats) {
	t.Helper()
	cfg := config.Default()
	paths, err := filter.NewPathFilterFromAttributes(cfg, []byte(attributes))
	if err != nil {
		t.Fatal(err)
	}
	rule, err := filter.NewBinaryRule([]byte(attributes))
	if err != nil {
		t.Fatal(err)
	}
	objects := &fakeObjects{contents: h.contents, limit: limit}
	w := &walker{
		ctx:      context.Background(),
		opts:     Options{History: h.records(head), PathFilter: paths},
		objects:  objects,
		binary:   rule,
		cache:    newContentCache(cacheBytes),
		identity: map[string]uint32{},
	}
	ownership, unavailable, err := w.walk(head)
	if err != nil {
		t.Fatalf("replaying: %v", err)
	}
	return ownership, unavailable, objects, w.stats
}

// owners renders a file's lines as "identity@day" for comparison.
func owners(t *testing.T, o *core.Ownership, path string) []string {
	t.Helper()
	for _, f := range o.Files {
		if f.Path == path {
			var out []string
			for _, line := range f.Lines {
				out = append(out, fmt.Sprintf("%s@%d", o.Identities[line.Owner], line.Day-dayOf(day(0))))
			}
			return out
		}
	}
	t.Fatalf("the map holds no %s; it holds %v", path, paths(o))
	return nil
}

func paths(o *core.Ownership) []string {
	var out []string
	for _, f := range o.Files {
		out = append(out, f.Path)
	}
	return out
}

func text(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

// A line keeps the owner and day of the commit that wrote it until a commit
// changes it; renames carry ownership with the file.
func TestReplayLinearHistory(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "f.go", content: text("a", "b", "c")})
	h.commit("c2", "grace", 2, "c1", edit{path: "f.go", content: text("a", "B", "c")})
	h.commit("c3", "alan", 3, "c2", edit{path: "g.go", from: "f.go", content: text("a", "B", "c", "d")})
	h.commit("c4", "ada", 4, "c3", edit{path: "h.go", content: text("h")})
	h.commit("c5", "grace", 5, "c4", edit{path: "h.go", remove: true}, edit{path: "i.go", content: text("i")})

	ownership, unavailable, _, stats := replayed(t, h, "c5", 0, "")
	if unavailable != "" {
		t.Fatalf("no map: %s", unavailable)
	}
	if got, want := owners(t, ownership, "g.go"), []string{"ada@1", "grace@2", "ada@1", "alan@3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("g.go is owned %v, want %v", got, want)
	}
	if got := paths(ownership); !reflect.DeepEqual(got, []string{"g.go", "i.go"}) {
		t.Errorf("the map holds %v, want g.go and i.go", got)
	}
	if stats.Commits != 5 || stats.PeakStates != 2 {
		t.Errorf("stats = %+v, want 5 commits and at most 2 states held on a line", stats)
	}
}

// A commit that swaps two files by renaming each onto the other reads both
// from where they were.
func TestReplaySwapReadsBothFromTheParent(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "a", content: text("from a")}, edit{path: "b", content: text("from b")})
	h.commit("c2", "grace", 2, "c1", edit{path: "b", from: "a", content: text("from a")},
		edit{path: "a", from: "b", content: text("from b")})
	ownership, _, _, _ := replayed(t, h, "c2", 0, "")
	if got := owners(t, ownership, "a"); !reflect.DeepEqual(got, []string{"ada@1"}) {
		t.Errorf("a is owned %v, want ada's line", got)
	}
	if got := owners(t, ownership, "b"); !reflect.DeepEqual(got, []string{"ada@1"}) {
		t.Errorf("b is owned %v, want ada's line", got)
	}
}

// A binary file has no lines, and a text file over the cap is degraded, and
// stays degraded for the versions that derive from it. Neither loses the walk.
func TestReplayBinaryAndOversizedFiles(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("big line\n", 20)
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "logo.png", content: "PNG\x00\x01"}, edit{path: "data.txt", content: big},
		edit{path: "doc.txt", content: text("doc")})
	h.commit("c2", "grace", 2, "c1", edit{path: "data.txt", content: text("small again")},
		edit{path: "doc.txt", content: "now\x00binary"})
	ownership, _, _, _ := replayed(t, h, "c2", 64, "")

	byPath := map[string]core.OwnedFile{}
	for _, f := range ownership.Files {
		byPath[f.Path] = f
	}
	if f := byPath["logo.png"]; !f.Binary || f.Lines != nil || f.Degraded != "" {
		t.Errorf("logo.png = %+v, want binary with no lines", f)
	}
	if f := byPath["doc.txt"]; !f.Binary || f.Lines != nil {
		t.Errorf("doc.txt = %+v, want binary with no lines", f)
	}
	if f := byPath["data.txt"]; f.Degraded != core.ReasonLimitReachedSize || len(f.Lines) != 1 {
		t.Errorf("data.txt = %+v, want its one line, degraded with %s", f, core.ReasonLimitReachedSize)
	}
}

// A binary file over the cap is binary, not degraded: its first bytes show it,
// and it has no lines to lose.
func TestReplayAnOversizedBinaryFileIsBinary(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "movie.mp4", content: "\x00" + strings.Repeat("x", 200)})
	ownership, _, _, _ := replayed(t, h, "c1", 64, "")
	if f := ownership.Files[0]; !f.Binary || f.Degraded != "" {
		t.Errorf("movie.mp4 = %+v, want binary and not degraded", f)
	}
}

// Attributes decide binary detection where they say something: -diff makes a
// file binary whatever its content.
func TestReplayAttributesDecideBinaryDetection(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "table.csv", content: text("a,b")})
	ownership, _, _, _ := replayed(t, h, "c1", 0, "*.csv -diff\n")
	if f := ownership.Files[0]; !f.Binary {
		t.Errorf("table.csv = %+v, want binary by its attribute", f)
	}
}

// An excluded path is never read, and is not in the map.
func TestReplayReadsNoExcludedPath(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "vendor/lib.go", content: text("vendored")},
		edit{path: "package-lock.json", content: text("{}")}, edit{path: "main.go", content: text("main")})
	h.commit("c2", "grace", 2, "c1", edit{path: "vendor/lib.go", content: text("vendored", "again")})
	ownership, _, objects, _ := replayed(t, h, "c2", 0, "")
	if got := paths(ownership); !reflect.DeepEqual(got, []string{"main.go"}) {
		t.Errorf("the map holds %v, want main.go alone", got)
	}
	for _, read := range objects.reads {
		if content := h.contents[read]; content != text("main") {
			t.Errorf("replay read %q, the content of an excluded path", content)
		}
	}
}

// A history that does not reach the analysed commit, or one of its ancestors,
// gives no map, and says why.
func TestReplayACutHistoryGivesNoMap(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "f", content: text("a")})
	h.commit("c2", "ada", 2, "c1", edit{path: "f", content: text("b")})
	cut := h.records("c2")
	cut.Commits = cut.Commits[:1]

	for name, records := range map[string]*model.History{"an ancestor": cut, "the analysed commit": h.records("c9")} {
		w := &walker{ctx: context.Background(), opts: Options{History: records}, objects: &fakeObjects{},
			cache: newContentCache(cacheBytes), identity: map[string]uint32{}}
		ownership, unavailable, err := w.walk(records.Repository.HeadCommit)
		if err != nil || ownership != nil || unavailable == "" {
			t.Errorf("%s outside the history: map %v, reason %q, error %v; want no map and a reason",
				name, ownership, unavailable, err)
		}
	}
}

// A change to a version the parent's state does not hold means the walk and
// the records disagree, which is refused rather than replayed around.
func TestReplayRefusesAChangeToAVersionItDoesNotHold(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("c1", "ada", 1, "", edit{path: "f", content: text("a")})
	h.commit("c2", "ada", 2, "c1", edit{path: "f", content: text("b")})
	records := h.records("c2")
	records.Commits[0].Files[0].OldBlob = h.name(text("not the parent's"))
	w := &walker{ctx: context.Background(), opts: Options{History: records}, objects: &fakeObjects{contents: h.contents},
		cache: newContentCache(cacheBytes), identity: map[string]uint32{}}
	if _, _, err := w.walk("c2"); core.ClassOf(err) != core.ClassInternal {
		t.Errorf("replaying a change to a version the parent does not hold returned %v, want an internal error", err)
	}
}

// Branches are replayed from their own parents, not from whatever was replayed
// last: the side branch's line positions are the side branch's, and the order
// is older commits first where the graph leaves it open. The fork's state is
// held until its last child is replayed.
func TestReplayBranchesDeriveFromTheirOwnParents(t *testing.T) {
	t.Parallel()
	h := newHistory()
	h.commit("root", "ada", 1, "", edit{path: "f", content: text("1", "2", "3")})
	h.commit("main1", "alan", 2, "root", edit{path: "f", content: text("0", "1", "2", "3")})
	h.commit("side1", "grace", 3, "root", edit{path: "f", content: text("1", "two", "3")})
	h.commit("main2", "alan", 4, "main1", edit{path: "g", content: text("g")})
	h.merge("merge", "ada", 5, []string{"main2", "side1"}, map[string]string{"f": text("0", "1", "two", "3"),
		"g": text("g")})

	ownership, _, _, stats := replayed(t, h, "merge", 0, "")
	f := owners(t, ownership, "f")
	if f[0] != "alan@2" || f[1] != "ada@1" || f[3] != "ada@1" {
		t.Errorf("f is owned %v; the lines from the first parent kept the wrong owners", f)
	}
	if stats.PeakStates != 3 {
		t.Errorf("held at most %d states, want 3: the fork's, a branch's and one being replayed", stats.PeakStates)
	}
}
