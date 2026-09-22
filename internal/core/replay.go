// The replay state: what the replay stage hands to aggregation (ADR-0020
// clause 3), in core because the families that read it may import nothing
// else (ADR-0040 clause 4).
//
// The ownership map is stored as ADR-0051 clause 1 requires: a line's owner is
// an index into the map's identity table, never a name or an address, and its
// authoring time is a day; a file's lines are one contiguous slice. The table
// holds identity digests, so the serialised map carries no raw address by
// construction (ADR-0051 clause 3, ADR-0033), and TestReplayMapCarriesNoRawAddress
// in internal/checks holds it to that.
//
// Beside the map, replay records the inputs of work-type classification
// (ADR-0074): the line-level change events of the analysed commits, counted by
// kind, editor, previous owner and age. It applies no recency window to them
// (clauses 8 and 9); the worktype family does, when it is aggregated, so the
// replay state is the same whatever the window.

package core

import (
	"encoding/json"
	"io"
)

// ReplayState is the replay stage's output for one analysis.
type ReplayState struct {
	// Tracked is every path in the analysed commit's tree, files and
	// submodules alike, in the order git lists the tree.
	Tracked []string
	// TextFileCount is the number of tracked text files at the analysed
	// commit (docs/metrics.md section 1): paths of Tracked that are not
	// excluded, hold a file, and are not binary.
	TextFileCount int
	// Ownership is the line ownership at the analysed commit, or nil where
	// replay could not derive it.
	Ownership *Ownership
	// Worktype is the work-type classification inputs of the analysed
	// commit's ancestry (ADR-0074), or nil where Ownership is nil.
	Worktype *WorktypeInputs
	// Unavailable says why Ownership is nil. It is a diagnostic, never report
	// content.
	Unavailable string
}

// WorktypeInputs are the inputs of work-type classification that replay
// records (ADR-0074 clause 8): the line-level change events of the analysed,
// non-bulk commits in the analysed commit's ancestry, counted rather than
// listed. A replacement or deletion is counted by editing identity, by the
// removed line's previous owner and by the removed line's age; an addition by
// editing identity alone. docs/metrics.md section 8 defines the events.
type WorktypeInputs struct {
	// Pairs are the replacements and deletions, one entry for each editing
	// identity and previous owner that have any, ordered by editor and then
	// by owner.
	Pairs []WorktypePair `json:"pairs"`
	// Additions are the additions, one entry for each editing identity that
	// made any, ordered by editor.
	Additions []WorktypeAdditions `json:"additions"`
	// Degraded names the limit that kept changes out of the counts:
	// limit_reached_size, where a counted commit changed a file whose version
	// replay could not read because it was over the single-file-size limit,
	// or whose lines derive from such a version (ADR-0074 clause 7, ADR-0048
	// clause 3). The worktype family turns it into its degraded status.
	Degraded Reason `json:"degraded,omitempty"`
}

// WorktypePair is the replacements and deletions one identity made of lines
// another identity owned, or of its own.
type WorktypePair struct {
	// Editor and Owner are identity digests (docs/metrics.md section 14,
	// `id`), never addresses.
	Editor string `json:"editor"`
	Owner  string `json:"owner"`
	// Replaced and Deleted are the histograms of the removed lines' ages, one
	// for replacements and one for deletions, each ordered by age. Their sum is
	// the histogram ADR-0074 clause 8 describes; they are kept apart so that
	// the kind of every event stays recorded.
	Replaced []AgeCount `json:"replaced,omitempty"`
	Deleted  []AgeCount `json:"deleted,omitempty"`
}

// AgeCount is one bar of an age histogram.
type AgeCount struct {
	// Days is an age: the editing commit's day minus the removed line's
	// authoring day, both by the configured date source (ADR-0074 clause 5).
	// It is negative where the editing commit is dated before the line.
	Days int32 `json:"days"`
	// Lines is the number of removed lines of that age.
	Lines int `json:"lines"`
}

// WorktypeAdditions is the number of additions one identity made.
type WorktypeAdditions struct {
	// Editor is an identity digest.
	Editor string `json:"editor"`
	Lines  int    `json:"lines"`
}

// Ownership is the line ownership map at the analysed commit.
type Ownership struct {
	// Commit is the analysed commit.
	Commit string `json:"commit"`
	// Identities are the owning identities, as their digests (docs/metrics.md
	// section 14, `id`). A line's Owner indexes this table.
	Identities []string `json:"identities"`
	// Files are the analysed commit's files whose path is not excluded,
	// ordered by path.
	Files []OwnedFile `json:"files"`
}

// OwnedFile is one file of the ownership map.
type OwnedFile struct {
	Path string `json:"path"`
	// Blob is the object name of the file's content at the analysed commit.
	Blob string `json:"blob"`
	// Binary marks a file detected as binary (docs/metrics.md section 1). It
	// has no lines.
	Binary bool `json:"binary,omitempty"`
	// Degraded names the limit that kept replay from reading the file's
	// content, or the content of a version its lines derive from. Its lines
	// are absent where the analysed commit's own content was not read, and
	// otherwise attributed without the version that was not (ADR-0072
	// clause 5, ADR-0048 clause 3).
	Degraded Reason `json:"degraded,omitempty"`
	// Lines are the file's lines in order, for a text file.
	Lines []OwnedLine `json:"lines,omitempty"`
}

// OwnedLine is one line's owner and authoring day.
type OwnedLine struct {
	// Owner is the index of the line's owning identity in the map's
	// Identities.
	Owner uint32 `json:"o"`
	// Day is the local calendar date the line was written on, as the number
	// of days since 1970-01-01: the date of the commit that wrote it, in the
	// timezone offset that commit recorded.
	Day int32 `json:"d"`
}

// Encode writes the map in its serialised form.
func (o *Ownership) Encode(w io.Writer) error {
	return json.NewEncoder(w).Encode(o)
}

// DecodeOwnership reads a map written by Encode.
func DecodeOwnership(r io.Reader) (*Ownership, error) {
	var o Ownership
	if err := json.NewDecoder(r).Decode(&o); err != nil {
		return nil, Internalf(err, "decoding an ownership map")
	}
	return &o, nil
}

// Lines returns the number of lines the map holds: every tracked text line at
// the analysed commit whose content replay read.
func (o *Ownership) Lines() int {
	n := 0
	for _, f := range o.Files {
		n += len(f.Lines)
	}
	return n
}
