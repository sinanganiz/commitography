package collect

import (
	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/core/model"
)

// Placing a merge's diffs against its parents, for the merge rule of ADR-0073
// clause 5.
//
// `git log -m` shows a merge's diff against each parent it differs from, in
// parent order, and nothing for a parent whose tree is the merge's own: an
// empty diff gets no header. A merge that kept its first parent's tree, as
// `git merge -s ours` does, therefore shows one diff, and that diff is against
// the second parent. Reading the diffs by position would take it for the first
// parent's and give the merge changes it never made.
//
// The diffs are placed by tree instead. A parent is shown exactly when its tree
// differs from the merge's, so the parents with a differing tree, in order, are
// the parents of the shown diffs, in order. The trees come from the records'
// own headers. A merge with a parent outside the records, beyond the date
// bounds, can be placed only when every parent was shown; otherwise its changes
// stay empty, and replay refuses any ancestry that reaches that parent before
// it would read them.
//
// A merge whose tree is none of its parents' is shown once per parent, and one
// whose tree is every parent's is shown once, with no diff, because git shows
// every commit at least once.

// resolveMerges gives every merge its changes against its first parent, with
// each parent's version of every changed file.
func resolveMerges(commits []model.Commit, sections map[string][][]rawChange) ([]model.Commit, error) {
	trees := make(map[string]string, len(commits))
	for _, c := range commits {
		trees[c.Hash] = c.Tree
	}
	for i := range commits {
		c := &commits[i]
		if !c.IsMerge {
			continue
		}
		placed, ok, err := placeSections(*c, sections[c.Hash], trees)
		if err != nil {
			return nil, err
		}
		if ok {
			c.MergeChanges = mergeChanges(placed)
		}
	}
	return commits, nil
}

// placeSections returns a merge's diffs by parent: placed[k] is the diff
// against parent k, and nil where parent k's tree is the merge's. ok is false
// where a parent's tree is unknown and the diffs cannot be placed.
func placeSections(c model.Commit, shown [][]rawChange, trees map[string]string) ([][]rawChange, bool, error) {
	placed := make([][]rawChange, len(c.Parents))
	differs := make([]bool, len(c.Parents))
	known := true
	count := 0
	for k, parent := range c.Parents {
		tree, ok := trees[parent]
		if !ok {
			known = false
			continue
		}
		if tree != c.Tree {
			differs[k] = true
			count++
		}
	}

	if !known {
		// Only the case with nothing left to place is placeable: every parent
		// shown, each with a diff.
		if len(shown) != len(c.Parents) {
			return nil, false, nil
		}
		for k, section := range shown {
			if len(section) == 0 {
				return nil, false, nil
			}
			placed[k] = section
		}
		return placed, true, nil
	}

	if count == 0 {
		for _, section := range shown {
			if len(section) != 0 {
				return nil, false, core.Internalf(nil, "merge %s has every parent's tree and yet git showed it a diff",
					short(c.Hash))
			}
		}
		return placed, true, nil
	}
	if len(shown) != count {
		return nil, false, core.Internalf(nil, "merge %s: git showed %d diffs, and %d of its parents' trees differ "+
			"from its own", short(c.Hash), len(shown), count)
	}
	next := 0
	for k := range c.Parents {
		if !differs[k] {
			continue
		}
		if len(shown[next]) == 0 {
			return nil, false, core.Internalf(nil, "merge %s: git showed an empty diff against a parent whose tree "+
				"differs from its own", short(c.Hash))
		}
		placed[k] = shown[next]
		next++
	}
	return placed, true, nil
}

// mergeChanges builds a merge's changes from its placed diffs. The files it
// changes are those of the diff against its first parent; a parent that has
// no diff has every file as the merge has it, and so does one whose diff does
// not name the file.
func mergeChanges(placed [][]rawChange) []model.MergeChange {
	if len(placed) == 0 || len(placed[0]) == 0 {
		return nil
	}
	byPath := make([]map[string]rawChange, len(placed))
	for k := 1; k < len(placed); k++ {
		byPath[k] = make(map[string]rawChange, len(placed[k]))
		for _, entry := range placed[k] {
			byPath[k][entry.path] = entry
		}
	}

	changes := make([]model.MergeChange, 0, len(placed[0]))
	for _, entry := range placed[0] {
		change := model.MergeChange{
			Path:    entry.path,
			NewBlob: entry.after(),
			Parents: make([]model.ParentVersion, len(placed)),
		}
		change.Parents[0] = model.ParentVersion{Path: entry.source(), Blob: entry.before()}
		for k := 1; k < len(placed); k++ {
			version := model.ParentVersion{Path: entry.path, Blob: entry.after()}
			if other, ok := byPath[k][entry.path]; ok {
				version = model.ParentVersion{Path: other.source(), Blob: other.before()}
			}
			change.Parents[k] = version
		}
		changes = append(changes, change)
	}
	return changes
}
