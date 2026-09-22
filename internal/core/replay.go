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
	// Unavailable says why Ownership is nil. It is a diagnostic, never report
	// content.
	Unavailable string
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
