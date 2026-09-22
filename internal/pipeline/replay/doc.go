// Package replay is the replay stage (ADR-0020 clause 3): it derives the line
// ownership of the analysed commit by walking the commit graph, and lists the
// analysed commit's tree. It is the only stage that reads the repository's
// file contents, and it reads them from git objects, never from the working
// tree, so a bare repository is analysable (ADR-0073 clause 8).
//
// The walk (ADR-0073):
//
//   - It covers the analysed commit and its ancestors, in topological order,
//     every parent before its children, and older commits first where the
//     graph leaves the order open. It is one sequential walk and starts no
//     goroutine (ADR-0052 clause 2, ADR-0073 clause 9);
//     TestReplayWalkStartsNoGoroutine in internal/checks holds it to that.
//   - Each commit's state is derived from its parents' states, never from a
//     running state. States share every file a commit leaves alone
//     (trie.go), and a state is released once its last child has been
//     replayed.
//   - A commit that changes a file aligns the file's new version with the
//     version in its first parent (diff.go). Aligned lines keep their owner;
//     every other line is owned by the commit's author, on the commit's day.
//   - A merge starts from its first parent's state and aligns each file it
//     changes with the first parent's version. A line that alignment leaves
//     new, but which exists unchanged in another parent's version of the file,
//     inherits that parent's owner. Only a line matching no parent is owned by
//     the merge's author (ADR-0073 clause 5). Merges are replayed whatever the
//     merge-counting setting (clause 6).
//
// Ownership is stored compactly (ADR-0051 clause 1): an owner is an index into
// the map's identity table, which holds identity digests, never addresses
// (ADR-0033); a line's authoring time is a day; a file's lines are one
// contiguous slice.
//
// As it replays an analysed commit, the walk also records the commit's
// line-level change events, the inputs of work-type classification (ADR-0074,
// worktype.go): each replacement and deletion counted by editor, previous
// owner and the removed line's age, and each addition by editor. The walk
// applies no recency window to them, and reads no analysis parameter to
// record them.
//
// A file's content is read through the length-framed object reader of the git
// package (ADR-0072), capped at the single-file-size limit (ADR-0048). A text
// file over the cap is marked degraded; a file whose first bytes show it
// binary has no lines to lose, and is binary whatever its size. Excluded paths
// (docs/metrics.md section 1) are never read.
//
// Where the collected history does not reach the analysed commit and all its
// ancestors, as the date bounds can leave it, no state can be derived and the
// ownership map is absent, with the reason. The tree is listed regardless.
package replay
