package temporal

import "github.com/sinanganiz/commitography/internal/core"

// Family is the temporal family's contract (ADR-0076 clause 1).
type Family struct{}

// Declaration returns what the family declares about itself. Nothing routes
// through it until the aggregate stage's registry does (WP-0061).
func (Family) Declaration() core.FamilyDeclaration {
	return core.FamilyDeclaration{
		Name:      "temporal",
		Inputs:    []core.InputKind{core.InputCommitRecords},
		Namespace: "temporal",
		Version:   version(),
		Status:    core.StatusFromComputation,
	}
}
