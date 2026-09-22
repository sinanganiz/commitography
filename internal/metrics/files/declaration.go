package files

import "github.com/sinanganiz/commitography/internal/core"

// Family is the files family's contract (ADR-0024 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061).
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name: "files",
		// Replay state as well as commit records: the file counts and the
		// extension distribution describe the analysed commit's tree, which
		// replay derives (ADR-0075 clause 1).
		Inputs:    []core.InputKind{core.InputCommitRecords, core.InputReplayState},
		Namespace: "files",
		Version:   version(),
		Status:    core.StatusFromComputation,
	}
}
