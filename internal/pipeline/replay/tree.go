package replay

import (
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/git"
)

// tree lists the analysed commit's tree through git and counts its tracked
// text files (docs/metrics.md section 1): paths that are not excluded, hold a
// file, and are not binary by git's rule and the analysed commit's attributes.
//
// Where the walk produced a map, the map already holds every such file with
// its content's object name, and the tree must agree with it file for file;
// a disagreement is a defect in the walk, and is refused rather than counted
// around. Where it did not, each file's content is read to classify it.
func (w *walker) tree(head string, ownership *core.Ownership) ([]string, int, error) {
	// -z, and the "--" after the commit, because a tracked path is repository
	// content: it may hold a newline or a quote (ADR-0045, ADR-0065 clause 2).
	entries, err := git.Records(w.ctx, git.At(w.opts.RepoPath, "ls-tree", "-r", "-z", head).Pathspecs())
	if err != nil {
		return nil, 0, err
	}
	var held map[string]*core.OwnedFile
	if ownership != nil {
		held = make(map[string]*core.OwnedFile, len(ownership.Files))
		for i := range ownership.Files {
			held[ownership.Files[i].Path] = &ownership.Files[i]
		}
	}

	tracked := make([]string, 0, len(entries))
	text, matched := 0, 0
	for _, entry := range entries {
		// <mode> SP <type> SP <object> TAB <path>; the path is the rest of
		// the record, whatever it contains.
		meta, path, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, 0, core.Internalf(nil, "an entry of the analysed commit's tree does not parse")
		}
		tracked = append(tracked, path)
		if fields[1] != "blob" || w.opts.PathFilter.Excluded(path) {
			continue
		}
		var binary bool
		if held != nil {
			file := held[path]
			if file == nil || file.Blob != fields[2] {
				return nil, 0, core.Internalf(nil, "the ownership map does not hold the analysed commit's version "+
					"of one of its files")
			}
			binary = file.Binary
			matched++
		} else {
			content, _, err := w.read(fields[2])
			if err != nil {
				return nil, 0, err
			}
			binary = w.binary.IsBinary(path, content)
		}
		if !binary {
			text++
		}
	}
	if held != nil && matched != len(held) {
		return nil, 0, core.Internalf(nil, "the ownership map holds %d files and the analysed commit's tree %d",
			len(held), matched)
	}
	return tracked, text, nil
}
