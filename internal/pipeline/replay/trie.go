package replay

import "github.com/sinanganiz/commitography/internal/core"

// A commit's state is a persistent trie from path numbers to files. Setting a
// file copies only the nodes on the way to it and shares every other node with
// the state it was derived from, so a commit costs the files it changes and
// not the files it leaves alone, and every state still held stays exactly as
// it was (ADR-0073 clause 3). A file itself is never changed once in a state:
// a commit that changes one builds a new one.

// fanout is the number of children of a node, 2^bits.
const (
	bits   = 5
	fanout = 1 << bits
)

// node is one node of the trie. kids is set at every level but the last, and
// files at the last.
type node struct {
	kids  []*node
	files []*core.OwnedFile
}

// trie is one state's files.
type trie struct {
	root  *node
	depth int
}

// depthFor is the number of levels a trie over n path numbers needs.
func depthFor(n int) int {
	depth, reach := 1, fanout
	for reach < n {
		depth++
		reach *= fanout
	}
	return depth
}

// get returns the file with the given path number, or nil.
func (t trie) get(id int) *core.OwnedFile {
	n := t.root
	for level := t.depth - 1; level > 0; level-- {
		if n == nil {
			return nil
		}
		n = n.kids[(id>>(bits*level))&(fanout-1)]
	}
	if n == nil {
		return nil
	}
	return n.files[id&(fanout-1)]
}

// set returns a trie in which the path number holds file, or nothing when file
// is nil. t itself is unchanged.
func (t trie) set(id int, file *core.OwnedFile) trie {
	return trie{root: t.setAt(t.root, id, file, t.depth-1), depth: t.depth}
}

func (t trie) setAt(n *node, id int, file *core.OwnedFile, level int) *node {
	copied := &node{}
	slot := (id >> (bits * level)) & (fanout - 1)
	if level == 0 {
		copied.files = make([]*core.OwnedFile, fanout)
		if n != nil {
			copy(copied.files, n.files)
		}
		copied.files[slot] = file
		return copied
	}
	copied.kids = make([]*node, fanout)
	var child *node
	if n != nil {
		copy(copied.kids, n.kids)
		child = n.kids[slot]
	}
	copied.kids[slot] = t.setAt(child, id, file, level-1)
	return copied
}

// each calls visit with every file the trie holds, in path number order.
func (t trie) each(visit func(id int, file *core.OwnedFile)) {
	var walk func(n *node, level, base int)
	walk = func(n *node, level, base int) {
		if n == nil {
			return
		}
		if level == 0 {
			for slot, file := range n.files {
				if file != nil {
					visit(base+slot, file)
				}
			}
			return
		}
		for slot, kid := range n.kids {
			walk(kid, level-1, base+slot<<(bits*level))
		}
	}
	walk(t.root, t.depth-1, 0)
}
