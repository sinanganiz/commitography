package replay

import (
	"container/heap"
	"context"
	"sort"
	"time"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
	"github.com/sinanganiz/commitography/internal/git"
)

// Options is what one replay reads.
type Options struct {
	Context context.Context
	// RepoPath is the repository the objects are read from, as resolved.
	RepoPath string
	// History is the collect stage's records for the analysis.
	History *model.History
	// PathFilter decides the excluded paths, from the analysed commit's
	// attributes (docs/metrics.md section 1). No excluded path is read.
	PathFilter *filter.PathFilter
	// MaxFileBytes is the single-file-size limit of ADR-0048 clause 1.
	MaxFileBytes int64
}

// Stats describes one replay, for the measurements of ADR-0050 clause 3.
type Stats struct {
	// Commits is the number of commits the walk replayed.
	Commits int
	// PeakStates is the largest number of commit states held at once.
	PeakStates int
}

// Unavailability reasons, which Run returns in ReplayState.Unavailable.
const (
	headOutsideHistory     = "the analysed commit is outside the collected history"
	ancestorOutsideHistory = "an ancestor of the analysed commit is outside the collected history"
)

// Run replays the history to the analysed commit and lists its tree.
func Run(opts Options) (*core.ReplayState, Stats, error) {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.History == nil {
		return nil, Stats{}, core.Internalf(nil, "replaying without the collected history")
	}
	rule, err := filter.NewBinaryRule([]byte(opts.History.Attributes))
	if err != nil {
		return nil, Stats{}, core.Internalf(err, "reading binary attributes from the analysed commit's attributes")
	}
	objects, err := git.OpenObjects(ctx, opts.RepoPath, opts.MaxFileBytes)
	if err != nil {
		return nil, Stats{}, err
	}
	w := &walker{
		ctx:      ctx,
		opts:     opts,
		objects:  objects,
		binary:   rule,
		cache:    newContentCache(cacheBytes),
		identity: map[string]uint32{},
	}
	state, err := w.run()
	if closeErr := objects.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, Stats{}, err
	}
	return state, w.stats, nil
}

// objectReader reads file contents by object name: the git package's
// length-framed reader (ADR-0072), or a stand-in in this package's tests.
type objectReader interface {
	Read(name string) (git.Object, error)
}

// walker is one replay.
type walker struct {
	ctx     context.Context
	opts    Options
	objects objectReader
	binary  *filter.BinaryRule
	cache   *contentCache

	// paths numbers every path the walk can hold: each path a change names
	// that is not excluded. depth is the trie depth those numbers need.
	paths map[string]int
	depth int

	// identities is the ownership map's identity table, and identity each
	// digest's index in it.
	identities []string
	identity   map[string]uint32

	stats Stats
}

func (w *walker) run() (*core.ReplayState, error) {
	head := w.opts.History.Repository.HeadCommit
	ownership, unavailable, err := w.walk(head)
	if err != nil {
		return nil, err
	}
	tracked, text, err := w.tree(head, ownership)
	if err != nil {
		return nil, err
	}
	return &core.ReplayState{Tracked: tracked, TextFileCount: text, Ownership: ownership, Unavailable: unavailable}, nil
}

// walk replays the analysed commit's ancestry and returns its ownership, or
// the reason there is none.
func (w *walker) walk(head string) (*core.Ownership, string, error) {
	commits := w.opts.History.Commits
	index := make(map[string]int, len(commits))
	for i, c := range commits {
		index[c.Hash] = i
	}
	target, ok := index[head]
	if !ok {
		return nil, headOutsideHistory, nil
	}
	ancestry, ok := ancestors(commits, index, target)
	if !ok {
		return nil, ancestorOutsideHistory, nil
	}
	order, remaining := topological(commits, index, ancestry)
	w.number(commits, order)

	states := make(map[int]trie, 2)
	for _, i := range order {
		if err := w.ctx.Err(); err != nil {
			return nil, "", err
		}
		c := &commits[i]
		parents := make([]trie, len(c.Parents))
		for k, p := range c.Parents {
			parents[k] = states[index[p]]
		}
		state, err := w.replay(c, parents)
		if err != nil {
			return nil, "", err
		}
		states[i] = state
		w.stats.PeakStates = max(w.stats.PeakStates, len(states))
		for _, p := range c.Parents {
			j := index[p]
			if remaining[j]--; remaining[j] == 0 {
				delete(states, j)
			}
		}
		w.stats.Commits++
	}

	var files []core.OwnedFile
	states[target].each(func(_ int, file *core.OwnedFile) { files = append(files, *file) })
	sort.Slice(files, func(a, b int) bool { return files[a].Path < files[b].Path })
	return &core.Ownership{Commit: head, Identities: w.identities, Files: files}, "", nil
}

// ancestors marks the target and every ancestor of it. It reports false where
// an ancestor is not among the records.
func ancestors(commits []model.Commit, index map[string]int, target int) ([]bool, bool) {
	marked := make([]bool, len(commits))
	marked[target] = true
	stack := []int{target}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, p := range commits[i].Parents {
			j, ok := index[p]
			if !ok {
				return nil, false
			}
			if !marked[j] {
				marked[j] = true
				stack = append(stack, j)
			}
		}
	}
	return marked, true
}

// topological orders the marked commits every parent before its children.
// Where the graph leaves the order open, the commit later in the records goes
// first: the records are newest first, so the walk runs oldest first. It also
// returns how many times each commit is a parent within the walk, which is
// how many children its state waits for.
func topological(commits []model.Commit, index map[string]int, marked []bool) ([]int, []int) {
	children := make([][]int, len(commits))
	waiting := make([]int, len(commits))
	remaining := make([]int, len(commits))
	for i, c := range commits {
		if !marked[i] {
			continue
		}
		for _, p := range c.Parents {
			j := index[p]
			children[j] = append(children[j], i)
			remaining[j]++
			waiting[i]++
		}
	}
	ready := &latestFirst{}
	for i := range commits {
		if marked[i] && waiting[i] == 0 {
			heap.Push(ready, i)
		}
	}
	var order []int
	for ready.Len() > 0 {
		i := heap.Pop(ready).(int)
		order = append(order, i)
		for _, child := range children[i] {
			if waiting[child]--; waiting[child] == 0 {
				heap.Push(ready, child)
			}
		}
	}
	return order, remaining
}

// latestFirst is a heap of record positions, the largest first.
type latestFirst []int

func (h latestFirst) Len() int           { return len(h) }
func (h latestFirst) Less(a, b int) bool { return h[a] > h[b] }
func (h latestFirst) Swap(a, b int)      { h[a], h[b] = h[b], h[a] }
func (h *latestFirst) Push(x any)        { *h = append(*h, x.(int)) }
func (h *latestFirst) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// number gives every path the walk can hold a number, in the order the walk
// first meets it.
func (w *walker) number(commits []model.Commit, order []int) {
	w.paths = map[string]int{}
	add := func(path string) {
		if path == "" || w.opts.PathFilter.Excluded(path) {
			return
		}
		if _, ok := w.paths[path]; !ok {
			w.paths[path] = len(w.paths)
		}
	}
	for _, i := range order {
		for _, f := range commits[i].Files {
			add(f.PreviousPath)
			add(f.Path)
		}
		for _, m := range commits[i].MergeChanges {
			for _, v := range m.Parents {
				add(v.Path)
			}
			add(m.Path)
		}
	}
	w.depth = depthFor(len(w.paths))
}

// pathID returns a path's number, and false for a path the walk never holds.
func (w *walker) pathID(path string) (int, bool) {
	id, ok := w.paths[path]
	return id, ok
}

// replay derives one commit's state from its parents'.
func (w *walker) replay(c *model.Commit, parents []trie) (trie, error) {
	switch len(parents) {
	case 0:
		return w.change(c, trie{depth: w.depth})
	case 1:
		return w.change(c, parents[0])
	default:
		return w.merge(c, parents)
	}
}

// write is one file a commit sets, or removes where file is nil.
type write struct {
	id   int
	file *core.OwnedFile
}

// apply writes a commit's removals and then its files. Every version a commit
// reads is taken from its parents before anything is written, so a commit that
// swaps two paths, or deletes a path and renames another onto it, reads each
// from where it was.
func apply(base trie, removals []int, sets []write) trie {
	t := base
	for _, id := range removals {
		t = t.set(id, nil)
	}
	for _, s := range sets {
		t = t.set(s.id, s.file)
	}
	return t
}

// change derives a non-merge commit's state from its parent's.
func (w *walker) change(c *model.Commit, base trie) (trie, error) {
	var removals []int
	var sets []write
	for _, f := range c.Files {
		source := f.Path
		if f.PreviousPath != "" {
			source = f.PreviousPath
		}
		var old *core.OwnedFile
		if id, held := w.pathID(source); held {
			old = base.get(id)
			if blobOf(old) != f.OldBlob {
				return trie{}, core.Internalf(nil, "commit %s changes a version of a file its parent's state does "+
					"not hold", short(c.Hash))
			}
			if f.PreviousPath != "" {
				removals = append(removals, id)
			}
		}
		id, held := w.pathID(f.Path)
		if !held {
			continue
		}
		if f.NewBlob == "" {
			removals = append(removals, id)
			continue
		}
		file, err := w.derive(c, f.Path, f.NewBlob, []*core.OwnedFile{old})
		if err != nil {
			return trie{}, err
		}
		sets = append(sets, write{id: id, file: file})
	}
	return apply(base, removals, sets), nil
}

// merge derives a merge's state: its first parent's, with every file it
// changes relative to that parent derived under the merge rule of ADR-0073
// clause 5. The file's new version is aligned with the first parent's version,
// and a line that alignment leaves new but which another parent's version
// holds unchanged inherits that parent's owner. Only a line no parent holds is
// the merge's own: a conflict resolved by hand, or an evil merge.
func (w *walker) merge(c *model.Commit, parents []trie) (trie, error) {
	var removals []int
	var sets []write
	for _, m := range c.MergeChanges {
		if len(m.Parents) != len(parents) {
			return trie{}, core.Internalf(nil, "merge %s records %d parents' versions of a file and has %d parents",
				short(c.Hash), len(m.Parents), len(parents))
		}
		versions := make([]*core.OwnedFile, len(parents))
		for k, v := range m.Parents {
			id, held := w.pathID(v.Path)
			if !held {
				continue
			}
			versions[k] = parents[k].get(id)
			if blobOf(versions[k]) != v.Blob {
				return trie{}, core.Internalf(nil, "merge %s records a version of a file its parent %d's state does "+
					"not hold", short(c.Hash), k+1)
			}
			if k == 0 && v.Path != m.Path {
				removals = append(removals, id)
			}
		}
		id, held := w.pathID(m.Path)
		if !held {
			continue
		}
		if m.NewBlob == "" {
			removals = append(removals, id)
			continue
		}
		file, err := w.derive(c, m.Path, m.NewBlob, versions)
		if err != nil {
			return trie{}, err
		}
		sets = append(sets, write{id: id, file: file})
	}
	return apply(parents[0], removals, sets), nil
}

// derive builds a file's new version from the versions it derives from: the
// first parent's first, then the others', any of which may be nil. A version
// with the same content is the file, owners and all. Otherwise each line of the
// new content aligned with a line of a version takes that line's owner, the
// first version to align it deciding, and every other line is the commit's.
func (w *walker) derive(c *model.Commit, path, blob string, versions []*core.OwnedFile) (*core.OwnedFile, error) {
	for _, v := range versions {
		if v != nil && v.Blob == blob {
			kept := *v
			kept.Path = path
			return &kept, nil
		}
	}
	content, oversized, err := w.read(blob)
	if err != nil {
		return nil, err
	}
	if w.binary.IsBinary(path, content) {
		return &core.OwnedFile{Path: path, Blob: blob, Binary: true}, nil
	}
	if oversized {
		return &core.OwnedFile{Path: path, Blob: blob, Degraded: core.ReasonLimitReachedSize}, nil
	}

	lines := splitLines(content)
	file := &core.OwnedFile{Path: path, Blob: blob, Lines: make([]core.OwnedLine, len(lines))}
	settled := make([]bool, len(lines))
	for _, v := range versions {
		if v == nil || v.Binary {
			continue
		}
		if v.Degraded != "" {
			// The version could not be read, so lines that came from it
			// cannot be told apart from new ones.
			file.Degraded = v.Degraded
			continue
		}
		older, err := w.linesOf(v)
		if err != nil {
			return nil, err
		}
		for j, i := range align(older, lines) {
			if i >= 0 && !settled[j] {
				file.Lines[j], settled[j] = v.Lines[i], true
			}
		}
	}
	author := w.author(c)
	for j := range file.Lines {
		if !settled[j] {
			file.Lines[j] = author
		}
	}
	return file, nil
}

// linesOf reads a version's content again, to align against it.
func (w *walker) linesOf(v *core.OwnedFile) ([][]byte, error) {
	content, oversized, err := w.read(v.Blob)
	if err != nil {
		return nil, err
	}
	lines := splitLines(content)
	if oversized || len(lines) != len(v.Lines) {
		return nil, core.Internalf(nil, "a file version replay holds %d lines for reads back as %d lines",
			len(v.Lines), len(lines))
	}
	return lines, nil
}

// read returns a file's content, or the head of a content over the cap.
func (w *walker) read(blob string) ([]byte, bool, error) {
	if content, ok := w.cache.get(blob); ok {
		return content, false, nil
	}
	object, err := w.objects.Read(blob)
	if err != nil {
		return nil, false, err
	}
	switch {
	case object.Missing:
		return nil, false, core.Internalf(nil, "object %s, a file's content, is missing from the repository", blob)
	case object.Type != "blob":
		return nil, false, core.Internalf(nil, "object %s is a %s, not a file's content", blob, object.Type)
	case object.Oversized:
		return object.Head, true, nil
	}
	w.cache.put(blob, object.Content)
	return object.Content, false, nil
}

// author is the owner and day of the lines a commit writes.
func (w *walker) author(c *model.Commit) core.OwnedLine {
	owner, ok := w.identity[c.IdentityID]
	if !ok {
		owner = uint32(len(w.identities))
		w.identities = append(w.identities, c.IdentityID)
		w.identity[c.IdentityID] = owner
	}
	return core.OwnedLine{Owner: owner, Day: dayOf(c.LocalTime)}
}

// dayOf is the number of days from 1970-01-01 to t's calendar date, in the
// offset t carries.
func dayOf(t time.Time) int32 {
	y, m, d := t.Date()
	return int32(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

// blobOf is the object name of a file's content, or empty for no file.
func blobOf(f *core.OwnedFile) string {
	if f == nil {
		return ""
	}
	return f.Blob
}

func short(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
