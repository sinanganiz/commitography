package replay

import (
	"sort"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/filter"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// The work-type classification inputs (ADR-0074). As each counted commit is
// replayed, the lines it changes in each file are read off the alignment that
// derived the file's new version (diff.go), as line-level change events:
//
//   - The lines the alignment leaves unmatched fall into changed blocks: runs
//     of R removed lines of the previous version and A added lines of the new
//     one, between two lines the alignment matched.
//   - Within a block, the first min(R, A) removed lines pair with the first
//     min(R, A) added lines by position. A paired added line is a
//     replacement, an added line left over is an addition, and a removed line
//     left over is a deletion. Each event counts once, so rewriting a line is
//     one replacement.
//   - A replacement or deletion carries the removed line's previous owner and
//     age: the commit's day minus the day the line was written, both by the
//     configured date source. An addition carries neither.
//
// A merge's blocks are taken against its first parent, and a merge records
// only what it did itself (ADR-0074 clause 6): an addition or replacement
// only where the added line is its own, matching no parent (ADR-0073
// clause 5), and a deletion only of a line every parent's version holds and
// its own does not. Every other difference was made by a parent's own
// commit, which recorded it.
//
// A change to or from a binary version has no lines to count. A change to or
// from a version whose lines replay cannot tell apart produces no events
// either: a version over the single-file-size limit, whose content replay did
// not read, or one derived from such a version, whose lines replay gives to
// its commit's author as though they were new. The inputs are then marked
// limit_reached_size, so that the family says so rather than counting those
// lines as new (ADR-0074 clause 7, ADR-0048 clause 3).
//
// Replay records ages and counts, and applies no recency window (ADR-0074
// clauses 8 and 9): the worktype family applies it.

// counted reports whether a commit's changes are change events: those of an
// analysed, non-bulk commit (ADR-0073 clause 6, docs/metrics.md section 8).
// Every other commit is replayed for ownership all the same.
//
// Deviation, removed by WP-0017: with a year set, core.Input leaves the
// commits of other years out of every family and out of the identities
// section. Replay reads no analysis parameter beyond those that decide which
// lines exist and who owns them, and the year is not one of them (ADR-0074),
// so with a year set its events include the other years.
func counted(c *model.Commit) bool {
	return filter.CountsForLines(*c)
}

// derivation is how the lines of a file's new version came about: for each,
// the line of the first version it was aligned with, or -1, and whether the
// commit wrote it itself, aligned with no version at all.
type derivation struct {
	first []int32
	own   []bool
}

// record records the change events of one file a counted commit changes.
// versions are the file's versions in the commit's parents, the first
// parent's first, any of which may be nil; file is its new version, or nil
// where the commit deletes it; d is how derive aligned file, or nil where it
// did not.
func (w *walker) record(c *model.Commit, versions []*core.OwnedFile, file *core.OwnedFile, d *derivation) error {
	if file != nil {
		for _, v := range versions {
			if v != nil && v.Blob == file.Blob {
				// The new version is a parent's version, whole: no line of
				// it changed.
				return nil
			}
		}
	}
	all := append([]*core.OwnedFile{file}, versions...)
	for _, v := range all {
		if v != nil && v.Binary {
			return nil
		}
	}
	for _, v := range all {
		if v != nil && v.Degraded != "" {
			w.events.degraded = true
			return nil
		}
	}

	var removed []core.OwnedLine
	if versions[0] != nil {
		removed = versions[0].Lines
	}
	var matched []int32
	var own []bool
	if d != nil {
		matched, own = d.first, d.own
	}
	present, err := w.presence(versions)
	if err != nil {
		return err
	}
	editor := w.events.editor(c.IdentityID)
	day := dayOf(c.LocalTime)
	was := func(i int) (uint32, int32) { return removed[i].Owner, day - removed[i].Day }
	blocks(matched, len(removed), func(out, in []int) {
		paired := min(len(out), len(in))
		for k, j := range in {
			switch {
			case !own[j]:
				// A merge's line from another parent, recorded there.
			case k < paired:
				editor.replaced(was(out[k]))
			default:
				editor.additions++
			}
		}
		for k, i := range out {
			if k < paired && own[in[k]] {
				continue // the removed half of a replacement
			}
			if present == nil || present[i] {
				editor.deleted(was(i))
			}
		}
	})
	return nil
}

// blocks calls visit with each changed block of an alignment, in order: its
// removed lines, as indices into the older version, and its added lines, as
// indices into the newer. matched is the alignment: for each line of the newer
// version, the line of the older it is, or -1, increasing where it is not -1.
// older is the number of lines of the older version.
func blocks(matched []int32, older int, visit func(removed, added []int)) {
	next := 0 // the first line of the older version not yet passed
	var added []int
	flush := func(end int) {
		if end > next || len(added) > 0 {
			removed := make([]int, 0, end-next)
			for i := next; i < end; i++ {
				removed = append(removed, i)
			}
			visit(removed, added)
		}
		added = nil
	}
	for j, i := range matched {
		if i < 0 {
			added = append(added, j)
			continue
		}
		flush(int(i))
		next = int(i) + 1
	}
	flush(older)
}

// presence returns, for a merge, which lines of its first parent's version of
// a file every other parent's version holds too, aligned with one of its own
// lines; a merge deletes only those. It returns nil for a commit with one
// parent, whose version holds every line it has.
func (w *walker) presence(versions []*core.OwnedFile) ([]bool, error) {
	if len(versions) < 2 || versions[0] == nil {
		return nil, nil
	}
	first, err := w.linesOf(versions[0])
	if err != nil {
		return nil, err
	}
	present := make([]bool, len(first))
	for i := range present {
		present[i] = true
	}
	for _, v := range versions[1:] {
		held := make([]bool, len(first))
		if v != nil {
			other, err := w.linesOf(v)
			if err != nil {
				return nil, err
			}
			for _, i := range align(first, other) {
				if i >= 0 {
					held[i] = true
				}
			}
		}
		for i := range present {
			present[i] = present[i] && held[i]
		}
	}
	return present, nil
}

// tally accumulates the classification inputs as the walk records events.
type tally struct {
	editors map[string]*editorTally // by identity digest
	// degraded is set when a counted commit changed a file whose lines replay
	// could not tell apart.
	degraded bool
}

// editorTally is one editing identity's events.
type editorTally struct {
	additions int
	owners    map[uint32]*ageTally // by the owner's index in the identity table
}

// ageTally is the ages of the lines one editor removed of one owner's.
type ageTally struct {
	replaced, deleted map[int32]int
}

func (t *tally) editor(id string) *editorTally {
	if t.editors == nil {
		t.editors = map[string]*editorTally{}
	}
	e, ok := t.editors[id]
	if !ok {
		e = &editorTally{owners: map[uint32]*ageTally{}}
		t.editors[id] = e
	}
	return e
}

func (e *editorTally) of(owner uint32) *ageTally {
	a, ok := e.owners[owner]
	if !ok {
		a = &ageTally{replaced: map[int32]int{}, deleted: map[int32]int{}}
		e.owners[owner] = a
	}
	return a
}

func (e *editorTally) replaced(owner uint32, age int32) { e.of(owner).replaced[age]++ }

func (e *editorTally) deleted(owner uint32, age int32) { e.of(owner).deleted[age]++ }

// worktypeInputs returns what the walk recorded, in its stored order: editors
// by digest, each editor's owners by digest, each histogram by age.
func (w *walker) worktypeInputs() *core.WorktypeInputs {
	out := &core.WorktypeInputs{Pairs: []core.WorktypePair{}, Additions: []core.WorktypeAdditions{}}
	if w.events.degraded {
		out.Degraded = core.ReasonLimitReachedSize
	}
	editors := make([]string, 0, len(w.events.editors))
	for id := range w.events.editors {
		editors = append(editors, id)
	}
	sort.Strings(editors)
	for _, id := range editors {
		e := w.events.editors[id]
		if e.additions > 0 {
			out.Additions = append(out.Additions, core.WorktypeAdditions{Editor: id, Lines: e.additions})
		}
		owners := make([]uint32, 0, len(e.owners))
		for owner := range e.owners {
			owners = append(owners, owner)
		}
		sort.Slice(owners, func(a, b int) bool { return w.identities[owners[a]] < w.identities[owners[b]] })
		for _, owner := range owners {
			a := e.owners[owner]
			out.Pairs = append(out.Pairs, core.WorktypePair{Editor: id, Owner: w.identities[owner],
				Replaced: histogram(a.replaced), Deleted: histogram(a.deleted)})
		}
	}
	return out
}

// histogram returns counts by age as bars ordered by age, or nil for none.
func histogram(counts map[int32]int) []core.AgeCount {
	if len(counts) == 0 {
		return nil
	}
	out := make([]core.AgeCount, 0, len(counts))
	for days, lines := range counts {
		out = append(out, core.AgeCount{Days: days, Lines: lines})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Days < out[b].Days })
	return out
}
