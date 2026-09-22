package ownership

import "github.com/sinanganiz/commitography/internal/core"

// method states how line ownership is derived (docs/metrics.md section 7),
// where that differs from git blame, the reference implementation (ADR-0020,
// ADR-0032 clause 8). The report carries it whatever the family's status. The
// pipeline root writes the same text into the report until the aggregate
// stage reads it from the declaration (WP-0061).
const method = "Line ownership is derived by forward replay of the history's diffs over the commit graph, " +
	"not by git blame. Each version of a file is aligned with the version it was changed from by one fixed " +
	"alignment computed in-process, and a line keeps its owner for as long as it is unchanged; at a merge, a line " +
	"keeps the owner it has in whichever parent holds it unchanged, and a line no parent holds is the merge's. " +
	"Blame's copy and move detection is not reproduced, so a line copied or moved from another file is owned by " +
	"the commit that copied or moved it. A text file larger than the single-file-size limit is not read, and is " +
	"marked as such."

// Family is the ownership family's contract (ADR-0076 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061). The family
// computes nothing until WP-0023, so it carries the zero version.
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name:      "ownership",
		Inputs:    []core.InputKind{core.InputReplayState},
		Namespace: "ownership",
		Version:   core.Version{},
		Method:    method,
		Status:    core.StatusNotImplemented,
	}
}
